// VirtTerm — TerminalConnection.cs
//
// Owns the TLS socket to VirtBBS's internal/virtterm listener — the live
// 80x25 terminal pane's transport. Runs a dedicated background read thread
// that feeds bytes straight into an AnsiScreen; critically, that thread
// NEVER touches WinForms controls directly (AnsiScreen.Feed only mutates
// its own cell grid and raises an event — TerminalControl marshals its own
// repaint via Control.Invoke). Writes (keystrokes) are synchronous sends on
// the calling thread, which is fine since they're tiny and infrequent.
//
// Server certificates are self-signed (internal/virtterm generates one on
// first run) — there's no CA to validate against, so the validation
// callback accepts any certificate. This is the same trust model as SSH's
// host-key-on-first-connect; a future version could pin the cert fingerprint
// after the first successful connect, but that's out of scope for Phase 2/3.
//
// Zmodem handoff: the server announces a download/upload by writing one of
// two fixed literal strings (see session.go) immediately before it starts
// behaving as a raw Zmodem sender/receiver instead of a text session. The
// read loop watches a small rolling tail of raw bytes (the same pattern
// AnsiScreen.UpdateTail already uses for its own "Command: " marker) for
// either string. On a match, everything up to and including the cue is fed
// to AnsiScreen as normal text (so it's visible in the terminal pane), and
// anything already read past it in the same chunk is handed to Zmodem as a
// prefix — then the live SslStream is handed off to Zmodem.ReceiveFile or
// Zmodem.SendFile, blocking this background thread for the duration of the
// transfer (fine: it's not the UI thread, and keystrokes/Send calls don't
// touch this thread). Once the transfer returns, the loop resumes reading
// the same stream as plain terminal bytes.
using System;
using System.Net.Security;
using System.Net.Sockets;
using System.Security.Cryptography.X509Certificates;
using System.Text;
using System.Threading;
using VirtTerm.Terminal;

namespace VirtTerm.Net;

public class TerminalConnection : IDisposable
{
    private static readonly byte[] DownloadCue = Encoding.ASCII.GetBytes("Start your Zmodem receive now. Press Ctrl+X to abort.");
    private static readonly byte[] UploadCue = Encoding.ASCII.GetBytes("Ready to receive via Zmodem. Start your upload now.");
    private readonly byte[] _cueTail = new byte[Math.Max(DownloadCue.Length, UploadCue.Length)];
    private int _cueTailLen;

    private readonly AnsiScreen _screen;
    private TcpClient? _tcp;
    private SslStream? _ssl;
    private Thread? _readThread;
    private volatile bool _running;

    public bool IsConnected => _ssl != null && _tcp is { Connected: true };

    public event Action? Disconnected;
    public event Action<Exception>? ConnectionError;

    /// <summary>Called when the server announces a download; return the destination path to save to, or null to skip it.</summary>
    public Func<ZmodemFileInfo, string?>? ZmodemResolveDownloadPath { get; set; }

    /// <summary>Called when the server announces an upload; return the local file path to send, or null to skip it.</summary>
    public Func<string?>? ZmodemResolveUploadPath { get; set; }

    /// <summary>Bytes transferred so far, for either direction.</summary>
    public Action<long>? ZmodemProgress { get; set; }

    public event Action<string>? ZmodemCompleted;
    public event Action<string>? ZmodemFailed;

    public TerminalConnection(AnsiScreen screen)
    {
        _screen = screen;
    }

    public void Connect(string host, int port)
    {
        Disconnect();

        _tcp = new TcpClient { NoDelay = true };
        _tcp.Connect(host, port);

        // Self-signed cert, no CA — accept unconditionally (see file header).
        _ssl = new SslStream(_tcp.GetStream(), false, (_, _, _, _) => true);
        _ssl.AuthenticateAsClient(host);

        _running = true;
        _readThread = new Thread(ReadLoop) { IsBackground = true, Name = "VirtTerm-ReadLoop" };
        _readThread.Start();
    }

    private void ReadLoop()
    {
        var buf = new byte[4096];
        try
        {
            while (_running)
            {
                int n = _ssl!.Read(buf, 0, buf.Length);
                if (n == 0) break; // remote closed

                int cueEnd = -1;
                bool isDownload = false;
                for (int i = 0; i < n; i++)
                {
                    if (ScanForCue(buf[i], out isDownload))
                    {
                        cueEnd = i + 1;
                        break;
                    }
                }

                if (cueEnd < 0)
                {
                    _screen.Feed(buf, n); // AnsiScreen.Feed only mutates its own state + raises an event
                    continue;
                }

                _screen.Feed(buf, cueEnd); // show the cue line itself as normal terminal text
                byte[] leftover = cueEnd < n ? buf[cueEnd..n] : Array.Empty<byte>();
                _cueTailLen = 0; // don't let a stale partial match leak into the resumed text stream
                RunZmodem(isDownload, leftover);
            }
        }
        catch (Exception ex)
        {
            if (_running) ConnectionError?.Invoke(ex);
        }
        finally
        {
            _running = false;
            Disconnected?.Invoke();
        }
    }

    /// <summary>Appends one byte to the rolling cue-detection tail and reports whether it now ends with either cue string.</summary>
    private bool ScanForCue(byte b, out bool isDownload)
    {
        if (_cueTailLen < _cueTail.Length)
        {
            _cueTail[_cueTailLen++] = b;
        }
        else
        {
            Array.Copy(_cueTail, 1, _cueTail, 0, _cueTail.Length - 1);
            _cueTail[^1] = b;
        }

        if (EndsWith(DownloadCue)) { isDownload = true; return true; }
        if (EndsWith(UploadCue)) { isDownload = false; return true; }
        isDownload = false;
        return false;

        bool EndsWith(byte[] cue)
        {
            if (_cueTailLen < cue.Length) return false;
            int start = _cueTailLen - cue.Length;
            for (int i = 0; i < cue.Length; i++)
            {
                if (_cueTail[start + i] != cue[i]) return false;
            }
            return true;
        }
    }

    /// <summary>Hands the live stream off to the Zmodem state machine for one file transfer, then resumes normal terminal I/O.</summary>
    private void RunZmodem(bool isDownload, byte[] leftover)
    {
        var io = new ZmodemIO(_ssl!, leftover);
        try
        {
            if (isDownload)
            {
                var resolve = ZmodemResolveDownloadPath;
                string? saved = Zmodem.ReceiveFile(
                    io,
                    info => resolve?.Invoke(info),
                    bytes => ZmodemProgress?.Invoke(bytes));
                ZmodemCompleted?.Invoke(saved ?? "(download skipped)");
            }
            else
            {
                string? localPath = ZmodemResolveUploadPath?.Invoke();
                if (localPath == null)
                {
                    ZmodemFailed?.Invoke("upload skipped — no file selected");
                    return;
                }
                Zmodem.SendFile(io, localPath, bytes => ZmodemProgress?.Invoke(bytes));
                ZmodemCompleted?.Invoke(localPath);
            }
        }
        catch (Exception ex)
        {
            ZmodemFailed?.Invoke(ex.Message);
        }
    }

    /// <summary>Sends raw bytes (typed keystrokes, or a menu-generated single keystroke) as-is.</summary>
    public void Send(byte[] data)
    {
        if (_ssl == null) return;
        try { _ssl.Write(data, 0, data.Length); }
        catch (Exception ex) { ConnectionError?.Invoke(ex); }
    }

    public void Disconnect()
    {
        _running = false;
        try { _ssl?.Close(); } catch { /* ignore */ }
        try { _tcp?.Close(); } catch { /* ignore */ }
        _ssl = null;
        _tcp = null;
    }

    public void Dispose() => Disconnect();
}

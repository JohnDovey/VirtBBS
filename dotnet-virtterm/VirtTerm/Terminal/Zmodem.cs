// VirtTerm — Zmodem.cs
//
// Pure C# port of the server's internal/transfer/zmodem.go wire format —
// same frame encoding, same CRC tables, same hex-header/escaped-data-
// subpacket layout — so this client can actually participate in file
// transfers instead of rendering a Zmodem handshake as garbage characters.
//
// Direction mapping (mirrors the Go server's two entry points exactly):
//   - Download (BBS calls transfer.SendFile, which blocks waiting for
//     ZRQINIT from the far end): ReceiveFile() below sends ZRQINIT first,
//     then receives ZFILE/ZDATA/ZEOF like a classic Zmodem receiver.
//   - Upload (BBS calls transfer.ReceiveFile, which sends ZRINIT
//     immediately): SendFile() below waits for that ZRINIT, then sends
//     ZFILE/ZDATA/ZEOF like a classic Zmodem sender.
//
// Deliberately ports only what the Go server actually emits/expects: hex
// headers (ZHEX) and CRC-16 data subpackets — the Go side never sends
// binary-encoded (ZBIN/ZBIN32) headers, so this doesn't implement decoding
// them either. No crash-recovery/resume support (always starts at offset 0)
// — the Go server supports resuming a partial download, but that's out of
// scope for a first working version here.
using System;
using System.Collections.Generic;
using System.IO;
using System.Text;

namespace VirtTerm.Terminal;

public class ZmodemFileInfo
{
    public string Filename { get; set; } = "";
    public long Size { get; set; }
}

/// <summary>Thrown when the far end cancels (five CAN bytes) or a frame/CRC error occurs.</summary>
public class ZmodemException(string message) : Exception(message);

/// <summary>
/// Buffered byte-level I/O over a live socket stream, with an optional
/// "prefix" of bytes already read off the wire by the caller (the part of
/// the read buffer that arrived after the literal text cue that triggered
/// the handoff, but before this class took over reading the stream).
/// </summary>
public class ZmodemIO(Stream stream, byte[]? prefix = null)
{
    private readonly Queue<byte> _prefix = new(prefix ?? Array.Empty<byte>());
    private readonly byte[] _buf = new byte[4096];
    private int _pos, _len;

    public int ReadByte()
    {
        if (_prefix.Count > 0) return _prefix.Dequeue();
        if (_pos >= _len)
        {
            _len = stream.Read(_buf, 0, _buf.Length);
            _pos = 0;
            if (_len <= 0) return -1;
        }
        return _buf[_pos++];
    }

    public void Write(byte[] data) => stream.Write(data, 0, data.Length);
}

public static class Zmodem
{
    // ── Frame type constants (must match internal/transfer/zmodem.go) ────────
    private const byte ZRQINIT = 0x00;
    private const byte ZRINIT = 0x01;
    private const byte ZACK = 0x03;
    private const byte ZFILE = 0x04;
    private const byte ZSKIP = 0x05;
    private const byte ZFIN = 0x08;
    private const byte ZRPOS = 0x09;
    private const byte ZDATA = 0x0A;
    private const byte ZEOF = 0x0B;
    private const byte ZCAN = 0x10; // sentinel: 5 consecutive CAN bytes seen

    private const byte ZPAD = 0x2A;
    private const byte ZDLE = 0x18;
    private const byte ZDLEE = 0x58;
    private const byte ZHEX = 0x42;

    private const byte ZCRCE = 0x68;
    private const byte ZCRCG = 0x69;
    private const byte ZCRCQ = 0x6A;
    private const byte ZCRCW = 0x6B;

    private const byte CANFC32 = 0x20;
    private const byte CANOVIO = 0x02;

    // ── CRC-16 (CCITT) ─────────────────────────────────────────────────────
    private static readonly ushort[] Crc16Tab = BuildCrc16Table();

    private static ushort[] BuildCrc16Table()
    {
        var tab = new ushort[256];
        for (int i = 0; i < 256; i++)
        {
            // Go's crc is uint16, which truncates to 16 bits on every shift
            // automatically. Masking with & 0xFFFF after each step here
            // reproduces that — using a wider int without masking per-step
            // (as an earlier draft of this did) silently desyncs the table
            // from Go's the moment an intermediate value exceeds 16 bits,
            // which is most table entries. Caught via a Go<->C# Zmodem
            // interop test: every CRC came out wrong despite identical-
            // looking frame bytes on the wire.
            int crc = i << 8;
            for (int j = 0; j < 8; j++)
                crc = (crc & 0x8000) != 0 ? ((crc << 1) ^ 0x1021) & 0xFFFF : (crc << 1) & 0xFFFF;
            tab[i] = (ushort)crc;
        }
        return tab;
    }

    private static ushort UpdateCrc16(ushort crc, byte b) =>
        (ushort)(Crc16Tab[(byte)(crc >> 8) ^ b] ^ (ushort)(crc << 8));

    private static ushort Crc16(byte[] data)
    {
        ushort crc = 0;
        foreach (var b in data) crc = UpdateCrc16(crc, b);
        return crc;
    }

    // ── Frame encoding ─────────────────────────────────────────────────────

    private static void SendHexHeader(ZmodemIO io, byte frameType, uint pos)
    {
        var hdr = new byte[] { frameType, (byte)pos, (byte)(pos >> 8), (byte)(pos >> 16), (byte)(pos >> 24) };
        ushort checksum = Crc16(hdr);

        var sb = new StringBuilder();
        foreach (var b in hdr) sb.Append(b.ToString("x2"));
        sb.Append(((byte)(checksum >> 8)).ToString("x2"));
        sb.Append(((byte)checksum).ToString("x2"));

        var frame = new List<byte> { ZPAD, ZPAD, ZDLE, ZHEX };
        frame.AddRange(Encoding.ASCII.GetBytes(sb.ToString()));
        frame.Add((byte)'\r');
        frame.Add((byte)'\n');
        frame.Add(0x11); // XON
        io.Write(frame.ToArray());
    }

    private static bool NeedsEscape(byte b) =>
        b == ZDLE || b == 0x0D || b == 0x8D || b == 0x11 || b == 0x91 || b == 0x13 || b == 0x93;

    private static byte[] EscapeData(byte[] data)
    {
        var outBytes = new List<byte>(data.Length + data.Length / 8);
        foreach (var b in data)
        {
            if (NeedsEscape(b)) { outBytes.Add(ZDLE); outBytes.Add((byte)(b ^ 0x40)); }
            else outBytes.Add(b);
        }
        return outBytes.ToArray();
    }

    /// <summary>Sends a ZDLE-escaped data subpacket (CRC-16 only — matches the Go sender's default).</summary>
    private static void SendDataSubpacket(ZmodemIO io, byte[] data, byte pktType)
    {
        var escaped = EscapeData(data);
        var crcData = new byte[data.Length + 1];
        Array.Copy(data, crcData, data.Length);
        crcData[^1] = pktType;
        ushort c = Crc16(crcData);
        var crcBytes = EscapeData(new[] { (byte)(c >> 8), (byte)c });

        var frame = new byte[escaped.Length + 2 + crcBytes.Length];
        Array.Copy(escaped, frame, escaped.Length);
        frame[escaped.Length] = ZDLE;
        frame[escaped.Length + 1] = pktType;
        Array.Copy(crcBytes, 0, frame, escaped.Length + 2, crcBytes.Length);
        io.Write(frame);
    }

    // ── Frame reading ──────────────────────────────────────────────────────

    // Marker is the ZCRC end-of-subpacket byte (ZCRCE/ZCRCG/ZCRCQ/ZCRCW) for
    // ZFILE/ZDATA frames, 0 otherwise. ZCRCG/ZCRCQ mean "more subpackets
    // follow with no new header in between" — see ReceiveFile's inner loop.
    private record Frame(byte Type, byte[] Data, byte Marker = 0);

    /// <summary>Scans for the next Zmodem frame, skipping any non-frame bytes (e.g. trailing CR/LF noise).</summary>
    private static Frame ReadAnyFrame(ZmodemIO io)
    {
        int canCount = 0;
        while (true)
        {
            int bi = io.ReadByte();
            if (bi < 0) throw new EndOfStreamException("connection closed during Zmodem transfer");
            byte b = (byte)bi;

            if (b == ZDLE) // CAN shares the ZDLE byte value (0x18) in this protocol's cancel convention
            {
                canCount++;
                if (canCount >= 5) return new Frame(ZCAN, Array.Empty<byte>());
                continue;
            }
            canCount = 0;

            if (b != ZPAD) continue;

            int b2i = io.ReadByte();
            if (b2i < 0) throw new EndOfStreamException("connection closed during Zmodem transfer");
            byte b2 = (byte)b2i;
            if (b2 == ZPAD)
            {
                b2i = io.ReadByte();
                if (b2i < 0) throw new EndOfStreamException("connection closed during Zmodem transfer");
                b2 = (byte)b2i;
            }
            if (b2 != ZDLE) continue; // not a frame start — keep scanning

            int enci = io.ReadByte();
            if (enci < 0) throw new EndOfStreamException("connection closed during Zmodem transfer");
            if ((byte)enci == ZHEX) return ReadHexFrame(io);
            // Unknown encoding (the Go server never sends ZBIN/ZBIN32) — keep scanning.
        }
    }

    private static Frame ReadHexFrame(ZmodemIO io)
    {
        var hexBuf = new byte[14];
        for (int i = 0; i < 14; i++)
        {
            int bi = io.ReadByte();
            if (bi < 0) throw new EndOfStreamException("connection closed reading hex header");
            hexBuf[i] = (byte)bi;
        }
        var raw = HexDecode(hexBuf);
        ushort got = Crc16(raw[..5]);
        ushort want = (ushort)((raw[5] << 8) | raw[6]);
        if (got != want) throw new ZmodemException($"hex header CRC mismatch: got {got:x4} want {want:x4}");

        byte type = raw[0];
        if (type == ZFILE || type == ZDATA)
        {
            // A data subpacket follows directly on the wire — the sender
            // wrote it in the same burst right after this header (see the
            // matching comment in the server's internal/transfer/zmodem.go
            // readHexFrame). Safe to consume the trailing CR LF XON here
            // before reading it, since more bytes are already in flight —
            // unlike header-only frames below, where the peer may be
            // waiting on *our* response first.
            //
            // Consumes exactly 3 bytes (CR, LF, XON), not "however many
            // match" — sendHexHeader on the server always emits exactly
            // those 3 trailing bytes, never more or fewer. A generic
            // "read until the next byte doesn't match" loop would need to
            // push the first non-matching byte (the payload's first byte)
            // back onto the stream to avoid losing it, which ZmodemIO
            // doesn't support (unlike Go's bufio.Reader.UnreadByte).
            for (int i = 0; i < 3; i++)
            {
                if (io.ReadByte() < 0) throw new EndOfStreamException("connection closed reading hex header");
            }
            var (payload, marker) = ReadDataSubpacket(io);
            return new Frame(type, payload, marker);
        }

        // Header-only frame: deliberately does NOT try to consume its
        // optional trailing CR LF XON — see the matching comment in
        // internal/transfer/zmodem.go for why that would deadlock.
        return new Frame(type, raw[1..5]);
    }

    /// <summary>
    /// Reads one byte, transparently un-escaping it if it's a ZDLE-prefixed
    /// sequence. SendDataSubpacket always escapes the trailing CRC bytes
    /// the same way as the payload, so they need this same unescaping when
    /// read back — reading them as two raw bytes only happens to work when
    /// neither CRC byte's value needs escaping, which is true most of the
    /// time but not always (see the matching fix in the server's
    /// internal/transfer/zmodem.go, caught by the same interop test).
    /// </summary>
    private static byte ReadEscapedByte(ZmodemIO io)
    {
        int bi = io.ReadByte();
        if (bi < 0) throw new EndOfStreamException("connection closed reading escaped byte");
        byte b = (byte)bi;
        if (b != ZDLE) return b;

        int b2i = io.ReadByte();
        if (b2i < 0) throw new EndOfStreamException("connection closed reading escaped byte");
        byte b2 = (byte)b2i;
        return b2 == ZDLEE ? ZDLE : (byte)(b2 ^ 0x40);
    }

    private static (byte[] Data, byte Marker) ReadDataSubpacket(ZmodemIO io)
    {
        var data = new List<byte>();
        while (true)
        {
            int bi = io.ReadByte();
            if (bi < 0) throw new EndOfStreamException("connection closed reading data subpacket");
            byte b = (byte)bi;
            if (b != ZDLE) { data.Add(b); continue; }

            int b2i = io.ReadByte();
            if (b2i < 0) throw new EndOfStreamException("connection closed reading data subpacket");
            byte b2 = (byte)b2i;
            if (b2 is ZCRCE or ZCRCG or ZCRCQ or ZCRCW)
            {
                byte c1 = ReadEscapedByte(io), c2 = ReadEscapedByte(io);
                ushort want = (ushort)((c1 << 8) | c2);
                var crcData = new byte[data.Count + 1];
                data.CopyTo(crcData);
                crcData[^1] = b2;
                ushort got = Crc16(crcData);
                if (got != want) throw new ZmodemException($"subpacket CRC mismatch: got {got:x4} want {want:x4}");
                return (data.ToArray(), b2);
            }
            data.Add(b2 == ZDLEE ? ZDLE : (byte)(b2 ^ 0x40));
        }
    }

    private static byte[] HexDecode(byte[] h)
    {
        var outBytes = new byte[7];
        for (int i = 0; i < 7; i++)
        {
            outBytes[i] = (byte)((FromHexNibble(h[i * 2]) << 4) | FromHexNibble(h[i * 2 + 1]));
        }
        return outBytes;
    }

    private static int FromHexNibble(byte b) => b switch
    {
        >= (byte)'0' and <= (byte)'9' => b - '0',
        >= (byte)'a' and <= (byte)'f' => b - 'a' + 10,
        >= (byte)'A' and <= (byte)'F' => b - 'A' + 10,
        _ => throw new ZmodemException($"invalid hex digit: {b}"),
    };

    // ── Public API ─────────────────────────────────────────────────────────

    /// <summary>
    /// Download: receives a single file. Sends ZRQINIT first (the Go
    /// server's transfer.SendFile blocks waiting for exactly this), then
    /// follows the same exchange as a classic Zmodem receiver.
    /// </summary>
    /// <param name="io">Buffered I/O over the live connection, primed with any leftover bytes from cue detection.</param>
    /// <param name="resolveDestPath">Given the incoming file's name/size, returns the full path to save to, or null to cancel.</param>
    /// <param name="onProgress">Called with bytes received so far, after each data chunk.</param>
    /// <returns>The destination path actually written, or null if cancelled/no file was offered.</returns>
    public static string? ReceiveFile(ZmodemIO io, Func<ZmodemFileInfo, string?> resolveDestPath, Action<long>? onProgress = null)
    {
        SendHexHeader(io, ZRQINIT, 0);

        var frame = ReadAnyFrame(io);
        if (frame.Type == ZFIN)
        {
            SendHexHeader(io, ZFIN, 0);
            return null; // no file offered
        }
        if (frame.Type != ZFILE)
            throw new ZmodemException($"expected ZFILE, got 0x{frame.Type:x2}");

        string nameField = Encoding.ASCII.GetString(frame.Data);
        string name = nameField.Split('\0')[0];
        if (string.IsNullOrEmpty(name))
            throw new ZmodemException("empty filename in ZFILE");
        // Defensive: strip any path the sender included, just like the Go receiver does.
        name = Path.GetFileName(name);

        long size = 0;
        var parts = nameField.Split('\0');
        if (parts.Length > 1)
        {
            var fields = parts[1].TrimStart().Split(' ');
            if (fields.Length > 0) long.TryParse(fields[0], out size);
        }

        string? destPath = resolveDestPath(new ZmodemFileInfo { Filename = name, Size = size });
        if (destPath == null)
        {
            SendHexHeader(io, ZSKIP, 0);
            return null;
        }

        using var f = new FileStream(destPath, FileMode.Create, FileAccess.Write);

        SendHexHeader(io, ZRPOS, 0);

        long received = 0;
        while (true)
        {
            frame = ReadAnyFrame(io);
            switch (frame.Type)
            {
                case ZDATA:
                    // A single ZDATA header is followed by a *run* of raw
                    // data subpackets with no header in between (the sender
                    // only sends a fresh header again once a subpacket's
                    // marker is ZCRCW). Keep consuming subpackets directly
                    // — bypassing ReadAnyFrame's header scan — until that
                    // marker is seen, then fall through to the outer loop
                    // for the next real header (ZEOF or another ZDATA).
                    // Missing this loop previously truncated every transfer
                    // at exactly one 1024-byte chunk: the scanner silently
                    // ate the headerless follow-on subpackets as noise
                    // while hunting for a ZPAD ZPAD ZDLE that never came.
                    while (true)
                    {
                        f.Write(frame.Data, 0, frame.Data.Length);
                        received += frame.Data.Length;
                        onProgress?.Invoke(received);
                        // Only ZCRCW (and ZCRCQ) actually request an ack —
                        // ZCRCG means "keep streaming, no ack needed", which
                        // SendFile relies on: it never reads any ack until
                        // after the final ZCRCW chunk. Acking every chunk
                        // anyway (as this used to do) queues up stale ACKs
                        // the sender never drains; SendFile's one
                        // post-ZCRCW read then picks up an early stale one,
                        // decides it's done, and closes the socket out from
                        // under us mid-transfer — observed as a broken-pipe
                        // write failure partway through a real download.
                        if (frame.Marker is ZCRCW or ZCRCQ)
                            SendHexHeader(io, ZACK, (uint)received);
                        if (frame.Marker == ZCRCW) break;
                        var (data, marker) = ReadDataSubpacket(io);
                        frame = new Frame(ZDATA, data, marker);
                    }
                    break;
                case ZEOF:
                    SendHexHeader(io, ZRINIT, (uint)((CANOVIO | CANFC32)));
                    var fin = ReadAnyFrame(io);
                    if (fin.Type == ZFIN)
                    {
                        SendHexHeader(io, ZFIN, 0);
                        io.Write(Encoding.ASCII.GetBytes("OO"));
                    }
                    return destPath;
                case ZCAN:
                    throw new ZmodemException("transfer cancelled by sender");
            }
        }
    }

    /// <summary>
    /// Upload: sends a single local file. Waits for ZRINIT first (the Go
    /// server's transfer.ReceiveFile sends this immediately on entry), then
    /// follows the same exchange as a classic Zmodem sender.
    /// </summary>
    public static void SendFile(ZmodemIO io, string localFilePath, Action<long>? onProgress = null)
    {
        var info = new FileInfo(localFilePath);
        long size = info.Length;
        long mtime = new DateTimeOffset(info.LastWriteTimeUtc).ToUnixTimeSeconds();

        var initial = ReadAnyFrame(io);
        if (initial.Type != ZRINIT)
            throw new ZmodemException($"expected ZRINIT, got 0x{initial.Type:x2}");

        var fileData = Encoding.ASCII.GetBytes($"{Path.GetFileName(localFilePath)}\0{size} {mtime} 0 0");
        SendHexHeader(io, ZFILE, 0);
        SendDataSubpacket(io, fileData, ZCRCW);

        var frame = ReadAnyFrame(io);
        if (frame.Type == ZSKIP) return;
        if (frame.Type == ZRINIT)
        {
            SendHexHeader(io, ZFILE, 0);
            SendDataSubpacket(io, fileData, ZCRCW);
            frame = ReadAnyFrame(io);
        }
        if (frame.Type != ZRPOS)
            throw new ZmodemException($"expected ZRPOS, got 0x{frame.Type:x2}");

        uint offset = frame.Data.Length >= 4
            ? (uint)(frame.Data[0] | (frame.Data[1] << 8) | (frame.Data[2] << 16) | (frame.Data[3] << 24))
            : 0;

        using var f = new FileStream(localFilePath, FileMode.Open, FileAccess.Read);
        f.Seek(offset, SeekOrigin.Begin);

        SendHexHeader(io, ZDATA, offset);

        const int chunkSize = 1024;
        var buf = new byte[chunkSize];
        long remaining = size - offset;
        long sent = offset;
        while (true)
        {
            int n = f.Read(buf, 0, chunkSize);
            if (n <= 0) break;
            remaining -= n;
            sent += n;
            bool isLast = remaining <= 0;
            byte pktType = isLast ? ZCRCW : ZCRCG;
            var chunk = n == buf.Length ? buf : buf[..n];
            SendDataSubpacket(io, chunk, pktType);
            onProgress?.Invoke(sent);
            if (isLast)
            {
                ReadAnyFrame(io); // wait for ZACK before ZEOF
                break;
            }
        }

        SendHexHeader(io, ZEOF, (uint)size);

        ReadAnyFrame(io); // wait for "ready for next" (ZRINIT)
        SendHexHeader(io, ZFIN, 0);

        // Some receivers send "OO" after ZFIN — drain up to two bytes if present.
        _ = io.ReadByte();
        _ = io.ReadByte();
    }
}

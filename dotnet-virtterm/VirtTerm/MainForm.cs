// VirtTerm — MainForm.cs
// Hosts the TerminalControl (the live 80x25 ANSI pane, internal/virtterm)
// and the dynamic MenuStrip (internal/userapi-driven, see
// Menu/DynamicMenuBuilder.cs). Wires keystrokes from both the terminal
// control and the menu into the same TerminalConnection.Send call, and
// polls nodelist versions for subscribed networks once per connection.
using System;
using System.Drawing;
using System.Threading.Tasks;
using System.Windows.Forms;
using VirtTerm.Forms;
using VirtTerm.Menu;
using VirtTerm.Net;
using VirtTerm.Nodelist;
using VirtTerm.Settings;
using VirtTerm.Terminal;

namespace VirtTerm;

public class MainForm : Form
{
    private AppSettings _settings;
    private readonly AnsiScreen _screen = new();
    private readonly TerminalConnection _conn;
    private readonly TerminalControl _terminalControl;
    private readonly DynamicMenuBuilder _menuBuilder = new();
    private readonly StatusStrip _status = new();
    private readonly ToolStripStatusLabel _statusLabel = new("Not connected");

    public MainForm()
    {
        _settings = AppSettings.Load();

        Text = "VirtTerm";
        StartPosition = FormStartPosition.CenterScreen;

        _conn = new TerminalConnection(_screen);
        _conn.Disconnected += () => BeginInvoke(new MethodInvoker(() => SetStatus("Disconnected")));
        _conn.ConnectionError += ex => BeginInvoke(new MethodInvoker(() => SetStatus($"Error: {ex.Message}")));

        _terminalControl = new TerminalControl(_screen);
        _terminalControl.KeyInput += data => _conn.Send(data);

        // The "Command: " gate is checked on every screen update (cheap
        // substring check — see AnsiScreen.UpdateTail) and reflected into
        // the menu's enabled state on the UI thread.
        _screen.Changed += () => BeginInvoke(new MethodInvoker(
            () => _menuBuilder.SetAtPrompt(_screen.IsAtCommandPrompt)));

        _menuBuilder.Keystroke += b => _conn.Send(new[] { b });
        _menuBuilder.LogonRequested += () => _ = ConnectAsync();
        _menuBuilder.LogoffRequested += () => _conn.Disconnect();
        _menuBuilder.HelpRequested += ShowHelp;
        _menuBuilder.AboutRequested += () => new AboutForm().ShowDialog(this);

        var menuStrip = _menuBuilder.Build();
        _menuBuilder.SetSysopVisible(_settings.IsSysop);

        _status.Items.Add(_statusLabel);
        _status.Dock = DockStyle.Bottom;
        menuStrip.Dock = DockStyle.Top;
        _terminalControl.Dock = DockStyle.Fill;

        // Fill-docked control must be added last so Top/Bottom controls
        // reserve their space first and the terminal pane takes the rest.
        MainMenuStrip = menuStrip;
        Controls.Add(menuStrip);
        Controls.Add(_status);
        Controls.Add(_terminalControl);

        ClientSize = new Size(_terminalControl.Width, _terminalControl.Height + menuStrip.Height + _status.Height);

        Shown += async (_, _) => await ConnectAsync();
    }

    private void SetStatus(string text) => _statusLabel.Text = text;

    private async Task ConnectAsync()
    {
        using var dlg = new ConnectForm(_settings);
        if (dlg.ShowDialog(this) != DialogResult.OK) return;

        _settings = dlg.Result;
        _settings.Save();
        _menuBuilder.SetSysopVisible(_settings.IsSysop);

        SetStatus($"Connecting to {_settings.Host}:{_settings.TerminalPort}...");
        try
        {
            // Connect() blocks on the TCP+TLS handshake — run it off the UI
            // thread so the window doesn't freeze while it's in progress.
            await Task.Run(() => _conn.Connect(_settings.Host, _settings.TerminalPort));
            SetStatus($"Connected to {_settings.Host}:{_settings.TerminalPort}");
        }
        catch (Exception ex)
        {
            SetStatus($"Connect failed: {ex.Message}");
            MessageBox.Show(this, ex.Message, "Connection failed", MessageBoxButtons.OK, MessageBoxIcon.Error);
            return;
        }

        _ = SyncNodelistsAsync();
    }

    private async Task SyncNodelistsAsync()
    {
        var api = new UserApiClient { Host = _settings.Host, Port = _settings.UserApiPort, Token = _settings.Token };
        var sync = new NodelistSyncService(api);
        try
        {
            var changed = await sync.CheckAllAsync(_settings.SubscribedNetworks);
            if (changed.Length > 0)
                BeginInvoke(new MethodInvoker(() =>
                    SetStatus($"Nodelist updated: {string.Join(", ", changed)}")));
        }
        catch
        {
            // userapi unreachable/misconfigured — nodelist sync is a
            // background convenience, never block the terminal session on it.
        }
    }

    private void ShowHelp()
    {
        MessageBox.Show(this,
            "VirtTerm is a graphical terminal for VirtBBS.\r\n\r\n" +
            "Type at the terminal pane exactly as you would over Telnet/SSH.\r\n" +
            "The BBS menu (top) sends the same single keystroke as typing it\r\n" +
            "yourself, and is only enabled while the BBS is showing its main\r\n" +
            "\"Command:\" prompt — mid-flow prompts (composing a message, etc.)\r\n" +
            "must be typed directly in the terminal pane.",
            "VirtTerm Help", MessageBoxButtons.OK, MessageBoxIcon.Information);
    }

    protected override void OnFormClosing(FormClosingEventArgs e)
    {
        _conn.Dispose();
        base.OnFormClosing(e);
    }
}

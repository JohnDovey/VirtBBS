// VirtTerm — ConnectForm.cs
// Login dialog: server address + the two ports (terminal TLS / user API)
// + the per-device API token. The token itself is generated on the BBS
// side via the profile menu's [T]okens option (internal/session/session.go,
// manageAPITokens) — this dialog has no "create account" flow, it just
// collects what the user already generated there.
using System;
using System.Drawing;
using System.Windows.Forms;
using VirtTerm.Settings;

namespace VirtTerm.Forms;

public class ConnectForm : Form
{
    private readonly TextBox _host = new();
    private readonly NumericUpDown _termPort = new();
    private readonly NumericUpDown _apiPort = new();
    private readonly TextBox _token = new();
    private readonly CheckBox _isSysop = new();

    private readonly AppSettings _current;
    public AppSettings Result { get; private set; } = new();

    public ConnectForm(AppSettings current)
    {
        _current = current;
        Text = "Connect to VirtBBS";
        FormBorderStyle = FormBorderStyle.FixedDialog;
        StartPosition = FormStartPosition.CenterScreen;
        MaximizeBox = false;
        MinimizeBox = false;
        ClientSize = new Size(380, 260);

        var layout = new TableLayoutPanel
        {
            Dock = DockStyle.Fill,
            Padding = new Padding(12),
            ColumnCount = 2,
            RowCount = 6,
        };
        layout.ColumnStyles.Add(new ColumnStyle(SizeType.Absolute, 110));
        layout.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 100));

        _host.Text = current.Host;
        _termPort.Minimum = 1; _termPort.Maximum = 65535; _termPort.Value = current.TerminalPort;
        _apiPort.Minimum = 1; _apiPort.Maximum = 65535; _apiPort.Value = current.UserApiPort;
        _token.Text = current.Token;
        _token.UseSystemPasswordChar = true;
        _isSysop.Checked = current.IsSysop;
        _isSysop.Text = "I am the sysop (shows the Sysop Menu item)";

        AddRow(layout, "Host:", _host);
        AddRow(layout, "Terminal port:", _termPort);
        AddRow(layout, "User API port:", _apiPort);
        AddRow(layout, "API token:", _token);
        layout.Controls.Add(_isSysop, 1, 4);
        layout.SetColumnSpan(_isSysop, 1);

        var hint = new Label
        {
            Text = "Generate a token on the BBS: Profile menu -> [T]okens -> [G]enerate.",
            AutoSize = false,
            Height = 32,
            ForeColor = Color.DimGray,
        };
        layout.Controls.Add(hint, 0, 5);
        layout.SetColumnSpan(hint, 2);

        var buttons = new FlowLayoutPanel { Dock = DockStyle.Bottom, FlowDirection = FlowDirection.RightToLeft, Height = 40 };
        var ok = new Button { Text = "Connect", DialogResult = DialogResult.OK };
        var cancel = new Button { Text = "Cancel", DialogResult = DialogResult.Cancel };
        ok.Click += (_, _) => OnOk();
        buttons.Controls.Add(ok);
        buttons.Controls.Add(cancel);
        AcceptButton = ok;
        CancelButton = cancel;

        Controls.Add(layout);
        Controls.Add(buttons);
    }

    private static void AddRow(TableLayoutPanel layout, string label, Control input)
    {
        int row = layout.Controls.Count / 2;
        layout.Controls.Add(new Label { Text = label, TextAlign = ContentAlignment.MiddleLeft, Dock = DockStyle.Fill }, 0, row);
        input.Dock = DockStyle.Fill;
        layout.Controls.Add(input, 1, row);
    }

    private void OnOk()
    {
        Result = new AppSettings
        {
            Host = _host.Text.Trim(),
            TerminalPort = (int)_termPort.Value,
            UserApiPort = (int)_apiPort.Value,
            Token = _token.Text.Trim(),
            IsSysop = _isSysop.Checked,
            SubscribedNetworks = _current.SubscribedNetworks,
        };
        DialogResult = DialogResult.OK;
        Close();
    }
}

// VirtTerm — AboutForm.cs
using System.Drawing;
using System.Windows.Forms;

namespace VirtTerm.Forms;

public class AboutForm : Form
{
    public AboutForm()
    {
        Text = "About VirtTerm";
        FormBorderStyle = FormBorderStyle.FixedDialog;
        StartPosition = FormStartPosition.CenterScreen;
        MaximizeBox = false;
        MinimizeBox = false;
        ClientSize = new Size(360, 180);

        var label = new Label
        {
            Dock = DockStyle.Fill,
            TextAlign = ContentAlignment.MiddleCenter,
            Padding = new Padding(16),
            Text = "VirtTerm\r\n\r\n" +
                   "A graphical terminal client for VirtBBS,\r\n" +
                   "connecting over its own TLS protocol\r\n" +
                   "(internal/virtterm) instead of Telnet/SSH.\r\n\r\n" +
                   "Part of the VirtBBS project.",
        };

        var ok = new Button { Text = "OK", DialogResult = DialogResult.OK, Dock = DockStyle.Bottom };

        Controls.Add(label);
        Controls.Add(ok);
        AcceptButton = ok;
        CancelButton = ok;
    }
}

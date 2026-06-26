// VirtTerm — TerminalControl.cs
//
// A custom WinForms UserControl that paints an AnsiScreen's 80x25 character
// grid using a monospace, CP437-friendly font. For best fidelity, install a
// real DOS-VGA font such as "Px437 IBM VGA8" or "Perfect DOS VGA 437" and
// set FontFamilyName below; this falls back to Consolas (close enough for
// box-drawing/block glyphs, though not pixel-identical to real CP437 art)
// when that font isn't installed on the machine.
//
// Raises KeyInput for every key the user types while the control has
// focus, so MainForm can forward raw keystrokes (typed or menu-generated)
// to the TerminalConnection — VirtTerm does not interpret input locally at
// all, exactly like a dumb terminal.
using System;
using System.Drawing;
using System.Windows.Forms;

namespace VirtTerm.Terminal;

public class TerminalControl : Control
{
    public static readonly string[] PreferredFontFamilies =
    {
        "Px437 IBM VGA8", "Perfect DOS VGA 437", "Consolas", "Courier New"
    };

    private readonly AnsiScreen _screen;
    private Font _font;
    private SizeF _cellSize;

    // Classic 16-color ANSI palette (matches SyncTerm/PCBoard conventions).
    private static readonly Color[] Palette =
    {
        Color.FromArgb(0, 0, 0),       Color.FromArgb(170, 0, 0),
        Color.FromArgb(0, 170, 0),     Color.FromArgb(170, 85, 0),
        Color.FromArgb(0, 0, 170),     Color.FromArgb(170, 0, 170),
        Color.FromArgb(0, 170, 170),   Color.FromArgb(170, 170, 170),
        Color.FromArgb(85, 85, 85),    Color.FromArgb(255, 85, 85),
        Color.FromArgb(85, 255, 85),   Color.FromArgb(255, 255, 85),
        Color.FromArgb(85, 85, 255),   Color.FromArgb(255, 85, 255),
        Color.FromArgb(85, 255, 255),  Color.FromArgb(255, 255, 255),
    };

    /// <summary>Raised for every keystroke typed while focused — raw bytes to send as-is.</summary>
    public event Action<byte[]>? KeyInput;

    public TerminalControl(AnsiScreen screen)
    {
        _screen = screen;
        _screen.Changed += () => { if (IsHandleCreated) Invoke(new MethodInvoker(() => Invalidate())); };

        SetStyle(ControlStyles.AllPaintingInWmPaint | ControlStyles.UserPaint |
                  ControlStyles.OptimizedDoubleBuffer | ControlStyles.ResizeRedraw, true);

        DoubleBuffered = true;
        TabStop = true;

        _font = PickAvailableFont(14f);
        using var g = CreateGraphics();
        var charSize = g.MeasureString("█", _font, int.MaxValue, StringFormat.GenericTypographic);
        _cellSize = new SizeF((float)Math.Ceiling(charSize.Width), (float)Math.Ceiling(_font.GetHeight(g)));

        Size = new Size((int)(_cellSize.Width * AnsiScreen.Cols), (int)(_cellSize.Height * AnsiScreen.Rows));
    }

    private static Font PickAvailableFont(float size)
    {
        foreach (var name in PreferredFontFamilies)
        {
            try
            {
                var f = new Font(name, size, FontStyle.Regular, GraphicsUnit.Pixel);
                if (string.Equals(f.Name, name, StringComparison.OrdinalIgnoreCase)) return f;
                f.Dispose();
            }
            catch { /* try next */ }
        }
        return new Font(FontFamily.GenericMonospace, size, FontStyle.Regular, GraphicsUnit.Pixel);
    }

    protected override void OnPaint(PaintEventArgs e)
    {
        var g = e.Graphics;
        g.TextRenderingHint = System.Drawing.Text.TextRenderingHint.SingleBitPerPixelGridFit;
        g.Clear(Color.Black);

        var sf = StringFormat.GenericTypographic;
        for (int r = 0; r < AnsiScreen.Rows; r++)
        {
            for (int c = 0; c < AnsiScreen.Cols; c++)
            {
                var cell = _screen.GetCell(r, c);
                float x = c * _cellSize.Width;
                float y = r * _cellSize.Height;

                var bg = Palette[cell.Bg & 0x07];
                if (bg != Color.Black)
                {
                    using var bgBrush = new SolidBrush(bg);
                    g.FillRectangle(bgBrush, x, y, _cellSize.Width, _cellSize.Height);
                }

                if (cell.Ch != ' ')
                {
                    var fg = Palette[cell.Fg & 0x0F];
                    using var fgBrush = new SolidBrush(fg);
                    g.DrawString(cell.Ch.ToString(), _font, fgBrush, x, y, sf);
                }
            }
        }

        // Block cursor.
        float cx = _screen.CursorCol * _cellSize.Width;
        float cy = _screen.CursorRow * _cellSize.Height;
        using var cursorBrush = new SolidBrush(Color.FromArgb(120, Color.White));
        g.FillRectangle(cursorBrush, cx, cy, _cellSize.Width, _cellSize.Height);
    }

    protected override bool IsInputKey(Keys keyData) => true; // claim arrows/tab/etc., don't let the form steal them

    // Only the arrow keys are handled here — they produce no WM_CHAR and so
    // never reach OnKeyPress. Enter/Backspace/Tab/Escape are deliberately
    // NOT handled in OnKeyDown: WinForms raises OnKeyPress for all of them
    // with the correct ASCII code (13/8/9/27) already, and handling them in
    // both places would send each keystroke to the BBS twice.
    protected override void OnKeyDown(KeyEventArgs e)
    {
        byte[]? seq = e.KeyCode switch
        {
            Keys.Up => Bytes(0x1B, '[', 'A'),
            Keys.Down => Bytes(0x1B, '[', 'B'),
            Keys.Right => Bytes(0x1B, '[', 'C'),
            Keys.Left => Bytes(0x1B, '[', 'D'),
            _ => null,
        };
        if (seq != null)
        {
            KeyInput?.Invoke(seq);
            e.Handled = true;
        }
    }

    protected override void OnKeyPress(KeyPressEventArgs e)
    {
        // Printable characters, plus Enter/Backspace/Tab/Escape, arrive here
        // — encode as the single CP437 byte where possible (ASCII passes
        // through as-is).
        KeyInput?.Invoke(new[] { (byte)e.KeyChar });
        e.Handled = true;
    }

    private static byte[] Bytes(params object[] vals)
    {
        var b = new byte[vals.Length];
        for (int i = 0; i < vals.Length; i++)
            b[i] = vals[i] is char ch ? (byte)ch : Convert.ToByte(vals[i]);
        return b;
    }

    protected override void Dispose(bool disposing)
    {
        if (disposing) _font.Dispose();
        base.Dispose(disposing);
    }
}

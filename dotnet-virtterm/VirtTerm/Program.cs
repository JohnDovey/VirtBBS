// VirtTerm — Program.cs
using System;
using System.Windows.Forms;

namespace VirtTerm;

internal static class Program
{
    [STAThread]
    private static void Main()
    {
        ApplicationConfiguration.Initialize();
        Application.Run(new MainForm());
    }
}

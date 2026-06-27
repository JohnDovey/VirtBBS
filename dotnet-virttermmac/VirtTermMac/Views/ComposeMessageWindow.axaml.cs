using Avalonia.Controls;
using Avalonia.Interactivity;

namespace VirtTermMac.Views;

public partial class ComposeMessageWindow : Window
{
    public bool Accepted { get; private set; }
    public string ToName => ToBox.Text?.Trim() ?? "";
    public string FromName => FromBox.Text?.Trim() ?? "";
    public string Subject => SubjectBox.Text?.Trim() ?? "";
    public string Body => BodyBox.Text ?? "";

    public ComposeMessageWindow() => InitializeComponent();

    public ComposeMessageWindow(string title, string to, string subject, string from) : this()
    {
        TitleText.Text = title;
        ToBox.Text = to;
        SubjectBox.Text = subject;
        FromBox.Text = from;
        QueueButton.Click += OnQueue;
        CancelButton.Click += (_, _) => Close();
    }

    private void OnQueue(object? sender, RoutedEventArgs e)
    {
        if (string.IsNullOrWhiteSpace(Subject) || string.IsNullOrWhiteSpace(Body))
            return;
        Accepted = true;
        Close();
    }
}

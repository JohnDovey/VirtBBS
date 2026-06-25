using System;
using System.Threading;
using System.Threading.Tasks;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using VirtBBS.GUI.Models;

namespace VirtBBS.GUI.ViewModels;

public partial class MainWindowViewModel : ViewModelBase
{
    public ApiClient Client { get; } = new();

    [ObservableProperty] private string _host       = "127.0.0.1";
    [ObservableProperty] private string _portText   = "9999";
    [ObservableProperty] private string _username   = "sysop";
    [ObservableProperty] private string _password   = "";
    [ObservableProperty] private string _connStatus = "Not connected.";
    [ObservableProperty] private bool   _isConnected;

    public NodesViewModel       Nodes        { get; }
    public UsersViewModel       Users        { get; }
    public MessagesViewModel    Messages     { get; }
    public ConferencesViewModel Conferences  { get; }
    public CallersViewModel     Callers      { get; }
    public ConfigViewModel      Config       { get; }
    public FidoViewModel        Fido         { get; }

    public MainWindowViewModel()
    {
        Nodes       = new NodesViewModel(Client);
        Users       = new UsersViewModel(Client);
        Messages    = new MessagesViewModel(Client);
        Conferences = new ConferencesViewModel(Client);
        Callers     = new CallersViewModel(Client);
        Config      = new ConfigViewModel(Client);
        Fido        = new FidoViewModel(Client);
    }

    [RelayCommand]
    private async Task ConnectAsync(CancellationToken ct)
    {
        Client.Host     = Host.Trim();
        Client.Port     = int.TryParse(PortText, out int p) ? p : 9999;
        Client.Username = Username.Trim();
        Client.Password = Password;

        ConnStatus  = "Connecting…";
        IsConnected = false;

        try
        {
            bool ok = await Client.TestConnectionAsync(ct);
            if (ok)
            {
                IsConnected = true;
                ConnStatus  = $"✓ Connected to {Client.Host}:{Client.Port}";
                await Task.WhenAll(
                    Nodes.LoadAsync(ct),
                    Users.LoadAsync(ct),
                    Conferences.LoadAsync(ct),
                    Config.LoadAsync(ct),
                    Callers.LoadAsync(ct)
                );
            }
            else
            {
                ConnStatus = "✗ Connection failed — check host, port and credentials.";
            }
        }
        catch (Exception ex)
        {
            ConnStatus = $"✗ {ex.Message}";
        }
    }
}

using System;
using System.Collections.ObjectModel;
using System.Threading;
using System.Threading.Tasks;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using VirtBBS.GUI.Models;

namespace VirtBBS.GUI.ViewModels;

public partial class MessagesViewModel(ApiClient client) : ViewModelBase
{
    [ObservableProperty] private string      _status          = "";
    [ObservableProperty] private int         _conferenceId    = 0;
    [ObservableProperty] private int         _startMsgNumber  = 1;
    [ObservableProperty] private BbsMessage? _selectedMessage;
    [ObservableProperty] private string      _selectedBody    = "";

    public ObservableCollection<BbsMessage> Messages { get; } = [];

    partial void OnSelectedMessageChanged(BbsMessage? value)
    {
        SelectedBody = value?.Body.Replace("\r\n", "\n") ?? "";
    }

    [RelayCommand]
    public async Task LoadAsync(CancellationToken ct = default)
    {
        try
        {
            var list = await client.CallAsync<BbsMessage[]>("messages.list",
                new { ConferenceID = ConferenceId, StartNum = StartMsgNumber, Limit = 50 }, ct) ?? [];
            Messages.Clear();
            foreach (var m in list) Messages.Add(m);
            Status = $"{Messages.Count} message(s) in conference {ConferenceId}.";
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task DeleteAsync(CancellationToken ct = default)
    {
        if (SelectedMessage is null) return;
        try
        {
            await client.CallAsync("messages.delete", new { ID = SelectedMessage.ID }, ct);
            Status = $"Message #{SelectedMessage.MsgNumber} deleted.";
            await LoadAsync(ct);
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }
}

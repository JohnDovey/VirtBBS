using System;
using System.Collections.ObjectModel;
using System.Threading;
using System.Threading.Tasks;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using VirtBBS.GUI.Models;

namespace VirtBBS.GUI.ViewModels;

public partial class ConferencesViewModel(ApiClient client) : ViewModelBase
{
    [ObservableProperty] private string          _status    = "";
    [ObservableProperty] private BbsConference?  _selected;

    // Edit fields.
    [ObservableProperty] private string _editName        = "";
    [ObservableProperty] private string _editDescription = "";
    [ObservableProperty] private bool   _editPublic      = true;
    [ObservableProperty] private int    _editReadSec     = 10;
    [ObservableProperty] private int    _editWriteSec    = 10;
    [ObservableProperty] private int    _editSysopSec    = 110;
    [ObservableProperty] private bool   _editEcho;
    [ObservableProperty] private string _editEchoTag     = "";
    [ObservableProperty] private string _editUplinkAddr  = "";
    [ObservableProperty] private string _editNetwork     = "";

    public ObservableCollection<BbsConference> Conferences { get; } = [];

    partial void OnSelectedChanged(BbsConference? value)
    {
        if (value is null) return;
        EditName        = value.Name;
        EditDescription = value.Description;
        EditPublic      = value.Public;
        EditReadSec     = value.ReadSec;
        EditWriteSec    = value.WriteSec;
        EditSysopSec    = value.SysopSec;
        EditEcho        = value.Echo;
        EditEchoTag     = value.EchoTag;
        EditUplinkAddr  = value.UplinkAddr;
        EditNetwork     = value.Network;
    }

    [RelayCommand]
    public async Task LoadAsync(CancellationToken ct = default)
    {
        try
        {
            var list = await client.CallAsync<BbsConference[]>("conferences.list", null, ct) ?? [];
            Conferences.Clear();
            foreach (var c in list) Conferences.Add(c);
            Status = $"{Conferences.Count} conference(s) loaded.";
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task SaveAsync(CancellationToken ct = default)
    {
        if (Selected is null) return;
        var c = new BbsConference
        {
            ID          = Selected.ID,
            Name        = EditName,
            Description = EditDescription,
            Public      = EditPublic,
            ReadSec     = EditReadSec,
            WriteSec    = EditWriteSec,
            SysopSec    = EditSysopSec,
            Echo        = EditEcho,
            EchoTag     = EditEchoTag,
            UplinkAddr  = EditUplinkAddr,
            Network     = EditNetwork,
        };
        try
        {
            await client.CallAsync("conferences.update", c, ct);
            Status = $"Conference '{c.Name}' saved.";
            await LoadAsync(ct);
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task CreateAsync(CancellationToken ct = default)
    {
        var c = new BbsConference
        {
            Name        = EditName,
            Description = EditDescription,
            Public      = EditPublic,
            ReadSec     = EditReadSec,
            WriteSec    = EditWriteSec,
            SysopSec    = EditSysopSec,
        };
        try
        {
            await client.CallAsync("conferences.create", c, ct);
            Status = $"Conference '{c.Name}' created.";
            await LoadAsync(ct);
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task DeleteAsync(CancellationToken ct = default)
    {
        if (Selected is null) return;
        try
        {
            await client.CallAsync("conferences.delete", new { ID = Selected.ID }, ct);
            Status = $"Conference deleted.";
            await LoadAsync(ct);
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }
}

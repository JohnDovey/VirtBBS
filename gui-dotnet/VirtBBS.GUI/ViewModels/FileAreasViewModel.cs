using System;
using System.Collections.ObjectModel;
using System.Threading;
using System.Threading.Tasks;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using VirtBBS.GUI.Models;

namespace VirtBBS.GUI.ViewModels;

public partial class FileAreasViewModel(ApiClient client) : ViewModelBase
{
    [ObservableProperty] private string _status = "";
    [ObservableProperty] private FileDir? _selected;

    [ObservableProperty] private string _editName = "";
    [ObservableProperty] private string _editDescription = "";
    [ObservableProperty] private string _editPath = "";
    [ObservableProperty] private int _editReadSec;
    [ObservableProperty] private int _editUploadSec;
    [ObservableProperty] private bool _editActive = true;

    public ObservableCollection<FileDir> Directories { get; } = [];

    partial void OnSelectedChanged(FileDir? value)
    {
        if (value is null) return;
        EditName = value.Name;
        EditDescription = value.Description;
        EditPath = value.Path;
        EditReadSec = value.ReadSec;
        EditUploadSec = value.UploadSec;
        EditActive = value.Active;
    }

    [RelayCommand]
    public async Task LoadAsync(CancellationToken ct = default)
    {
        try
        {
            var list = await client.CallAsync<FileDir[]>("files.list", null, ct) ?? [];
            Directories.Clear();
            foreach (var d in list) Directories.Add(d);
            Status = $"{Directories.Count} file area(s) loaded.";
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task SaveAsync(CancellationToken ct = default)
    {
        if (Selected is null) return;
        var d = new FileDir
        {
            ID = Selected.ID,
            Name = EditName,
            Description = EditDescription,
            Path = EditPath,
            ReadSec = EditReadSec,
            UploadSec = EditUploadSec,
            Active = EditActive,
        };
        try
        {
            await client.CallAsync("files.update", d, ct);
            Status = $"File area '{d.Name}' saved.";
            await LoadAsync(ct);
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task CreateAsync(CancellationToken ct = default)
    {
        if (string.IsNullOrWhiteSpace(EditName)) { Status = "Name required."; return; }
        try
        {
            await client.CallAsync("files.create", new
            {
                Name = EditName,
                Description = EditDescription,
                Path = string.IsNullOrWhiteSpace(EditPath) ? EditName.ToLowerInvariant() : EditPath,
                ReadSec = EditReadSec,
                UploadSec = EditUploadSec,
            }, ct);
            Status = $"File area '{EditName}' created.";
            await LoadAsync(ct);
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }
}

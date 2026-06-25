using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using VirtBBS.GUI.Models;

namespace VirtBBS.GUI.ViewModels;

public partial class UsersViewModel(ApiClient client) : ViewModelBase
{
    private List<BbsUser> _allUsers = [];

    [ObservableProperty] private string   _status       = "";
    [ObservableProperty] private string   _searchQuery  = "";
    [ObservableProperty] private BbsUser? _selectedUser;

    // Edit fields (bound to detail pane).
    [ObservableProperty] private string _editCity          = "";
    [ObservableProperty] private string _editPhone         = "";
    [ObservableProperty] private int    _editSecurity      = 10;
    [ObservableProperty] private int    _editPageLength    = 24;
    [ObservableProperty] private bool   _editAnsi          = true;
    [ObservableProperty] private bool   _editSysop;
    [ObservableProperty] private bool   _editDeleted;
    [ObservableProperty] private string _editComment       = "";
    [ObservableProperty] private string _editEditorType    = "simple";

    public ObservableCollection<BbsUser> Users { get; } = [];

    partial void OnSelectedUserChanged(BbsUser? value)
    {
        if (value is null) return;
        EditCity       = value.City;
        EditPhone      = value.PhoneBusiness;
        EditSecurity   = value.SecurityLevel;
        EditPageLength = value.PageLength;
        EditAnsi       = value.ANSI;
        EditSysop      = value.Sysop;
        EditDeleted    = value.Deleted;
        EditComment    = value.Comment1;
        EditEditorType = value.EditorType;
    }

    [RelayCommand]
    public async Task LoadAsync(CancellationToken ct = default)
    {
        try
        {
            _allUsers = (await client.CallAsync<BbsUser[]>("users.list", null, ct) ?? []).ToList();
            Users.Clear();
            foreach (var u in _allUsers) Users.Add(u);
            Status = $"{Users.Count} user(s) loaded.";
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task SaveAsync(CancellationToken ct = default)
    {
        if (SelectedUser is null) return;
        try
        {
            var u = SelectedUser;
            u.City          = EditCity;
            u.PhoneBusiness = EditPhone;
            u.SecurityLevel = EditSecurity;
            u.PageLength    = EditPageLength;
            u.ANSI          = EditAnsi;
            u.Sysop         = EditSysop;
            u.Deleted       = EditDeleted;
            u.Comment1      = EditComment;
            u.EditorType    = EditEditorType;
            await client.CallAsync("users.update", u, ct);
            Status = $"User '{u.Name}' saved.";
            await LoadAsync(ct);
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task DeleteAsync(CancellationToken ct = default)
    {
        if (SelectedUser is null) return;
        try
        {
            await client.CallAsync("users.delete", new { ID = SelectedUser.ID }, ct);
            Status = $"User '{SelectedUser.Name}' deleted.";
            await LoadAsync(ct);
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private void Search()
    {
        var q = SearchQuery.Trim();
        var filtered = q.Length == 0
            ? _allUsers
            : _allUsers.Where(u =>
                u.Name.Contains(q, StringComparison.OrdinalIgnoreCase) ||
                u.City.Contains(q, StringComparison.OrdinalIgnoreCase));

        Users.Clear();
        foreach (var u in filtered) Users.Add(u);
        Status = $"{Users.Count} user(s) matched.";
    }
}

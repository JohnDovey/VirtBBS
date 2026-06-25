using System;
using System.Collections.ObjectModel;
using System.Text.Json.Nodes;
using System.Threading;
using System.Threading.Tasks;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using VirtBBS.GUI.Models;

namespace VirtBBS.GUI.ViewModels;

public partial class CallersViewModel(ApiClient client) : ViewModelBase
{
    [ObservableProperty] private string _status      = "";
    [ObservableProperty] private string _searchQuery = "";
    [ObservableProperty] private string _statsText   = "";

    public ObservableCollection<CallerEntry> Callers { get; } = [];

    [RelayCommand]
    public async Task LoadAsync(CancellationToken ct = default)
    {
        try
        {
            var list = await client.CallAsync<CallerEntry[]>("callers.list",
                new { N = 100 }, ct) ?? [];
            Callers.Clear();
            foreach (var e in list) Callers.Add(e);

            // Also fetch today's stats.
            var stats = await client.CallAsync<JsonNode>("callers.stats", null, ct);
            if (stats is not null)
                StatsText = $"Today: {stats["unique"]?.GetValue<int>()} unique, {stats["total"]?.GetValue<int>()} total calls.";

            Status = $"{Callers.Count} recent caller(s) shown.";
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task SearchAsync(CancellationToken ct = default)
    {
        if (string.IsNullOrWhiteSpace(SearchQuery)) { await LoadAsync(ct); return; }
        try
        {
            var list = await client.CallAsync<CallerEntry[]>("callers.search",
                new { Query = SearchQuery, N = 50 }, ct) ?? [];
            Callers.Clear();
            foreach (var e in list) Callers.Add(e);
            Status = $"{Callers.Count} result(s) for '{SearchQuery}'.";
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }
}

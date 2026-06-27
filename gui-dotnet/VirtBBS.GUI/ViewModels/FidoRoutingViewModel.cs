using System;
using System.Collections.ObjectModel;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using VirtBBS.GUI.Models;

namespace VirtBBS.GUI.ViewModels;

public partial class FidoRoutingViewModel(ApiClient client) : ViewModelBase
{
    [ObservableProperty] private string _status = "";
    [ObservableProperty] private string _selectedNetwork = FidoNetworksViewModel.PrimaryNetwork;
    [ObservableProperty] private string _newPattern = "";
    [ObservableProperty] private string _newRouteTo = "";
    [ObservableProperty] private FidoRoute? _selectedRoute;
    [ObservableProperty] private FidoMember? _selectedMember;

    public ObservableCollection<string> NetworkNames { get; } = [];
    public ObservableCollection<FidoRoute> Routes { get; } = [];
    public ObservableCollection<FidoMember> Members { get; } = [];

    partial void OnSelectedNetworkChanged(string value) => _ = LoadAsync();

    [RelayCommand]
    public async Task LoadAsync(CancellationToken ct = default)
    {
        try
        {
            var names = await client.CallAsync<string[]>("fido.networks.list", null, ct) ?? [FidoNetworksViewModel.PrimaryNetwork];
            NetworkNames.Clear();
            foreach (var n in names) NetworkNames.Add(n);
            if (!NetworkNames.Contains(SelectedNetwork))
                SelectedNetwork = NetworkNames.FirstOrDefault() ?? FidoNetworksViewModel.PrimaryNetwork;

            var routes = await client.CallAsync<FidoRoute[]>("fido.routes.list",
                new { Network = SelectedNetwork }, ct) ?? [];
            Routes.Clear();
            foreach (var r in routes) Routes.Add(r);

            var members = await client.CallAsync<FidoMember[]>("fido.members.list",
                new { Network = SelectedNetwork }, ct) ?? [];
            Members.Clear();
            foreach (var m in members) Members.Add(m);

            Status = $"{Routes.Count} route(s), {Members.Count} member(s).";
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task AddRouteAsync(CancellationToken ct = default)
    {
        if (string.IsNullOrWhiteSpace(NewPattern) || string.IsNullOrWhiteSpace(NewRouteTo))
        {
            Status = "Pattern and route-to required.";
            return;
        }
        try
        {
            await client.CallAsync("fido.routes.add",
                new { Network = SelectedNetwork, Pattern = NewPattern.Trim(), RouteTo = NewRouteTo.Trim() }, ct);
            NewPattern = NewRouteTo = "";
            await LoadAsync(ct);
            Status = "Route added.";
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task RemoveRouteAsync(CancellationToken ct = default)
    {
        if (SelectedRoute is null) return;
        try
        {
            await client.CallAsync("fido.routes.remove",
                new { Network = SelectedNetwork, Pattern = SelectedRoute.Pattern }, ct);
            await LoadAsync(ct);
            Status = "Route removed.";
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }
}

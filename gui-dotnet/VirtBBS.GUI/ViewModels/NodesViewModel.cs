using System;
using System.Collections.ObjectModel;
using System.Threading;
using System.Threading.Tasks;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using VirtBBS.GUI.Models;

namespace VirtBBS.GUI.ViewModels;

public partial class NodesViewModel(ApiClient client) : ViewModelBase
{
    [ObservableProperty] private string _status = "";
    [ObservableProperty] private string _broadcastMessage = "";
    [ObservableProperty] private NodeStatus? _selectedNode;

    public ObservableCollection<NodeStatus> Nodes { get; } = [];

    [RelayCommand]
    public async Task LoadAsync(CancellationToken ct = default)
    {
        try
        {
            var list = await client.CallAsync<NodeStatus[]>("nodes.list", null, ct) ?? [];
            Nodes.Clear();
            foreach (var n in list) Nodes.Add(n);
            Status = $"Refreshed — {Nodes.Count} node(s) active.";
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task KickAsync(CancellationToken ct = default)
    {
        if (SelectedNode is null) return;
        try
        {
            await client.CallAsync("node.kick", new { NodeID = SelectedNode.NodeID }, ct);
            Status = $"Node {SelectedNode.NodeID} kicked.";
            await LoadAsync(ct);
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task BroadcastAsync(CancellationToken ct = default)
    {
        if (string.IsNullOrWhiteSpace(BroadcastMessage)) return;
        try
        {
            await client.CallAsync("node.broadcast", new { From = "Sysop", Message = BroadcastMessage }, ct);
            Status = "Broadcast sent.";
            BroadcastMessage = "";
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }
}

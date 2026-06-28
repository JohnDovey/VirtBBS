using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Linq;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using VirtBBS.GUI.Models;

namespace VirtBBS.GUI.ViewModels;

public partial class FidoViewModel(ApiClient client) : ViewModelBase
{
    [ObservableProperty] private string _status = "";
    [ObservableProperty] private string _selectedNetwork = FidoNetworksViewModel.DefaultPrimaryNetwork;

    // Nodelist search.
    [ObservableProperty] private string _nodeQuery = "";
    [ObservableProperty] private int _nodePage = 1;
    [ObservableProperty] private int _nodeTotalPages = 1;
    [ObservableProperty] private FidoNode? _selectedNode;

    public ObservableCollection<string> NetworkNames { get; } = [];
    public ObservableCollection<FidoNode> NodeResults { get; } = [];

    // Netmail compose.
    [ObservableProperty] private string _toAddr = "";
    [ObservableProperty] private string _toName = "";
    [ObservableProperty] private string _nmSubject = "";
    [ObservableProperty] private string _nmBody = "";
    [ObservableProperty] private bool _crash;

    // Nodelist import/export.
    [ObservableProperty] private string _importPath = "";
    [ObservableProperty] private string _exportPath = "";
    [ObservableProperty] private string _versionText = "";

    // Local nodelist editor.
    public ObservableCollection<LocalNodeEditRow> LocalEditorNodes { get; } = [];
    private readonly HashSet<string> _localNodesPendingDelete = new(StringComparer.OrdinalIgnoreCase);
    [ObservableProperty] private LocalNodeEditRow? _selectedLocalNode;

    // BinkP session log.
    [ObservableProperty] private string _binkpLogText = "";
    [ObservableProperty] private string _binkpLogPath = "";

    // BinkP statistics.
    public ObservableCollection<string> StatsPeriods { get; } =
        ["Today", "Yesterday", "This Month", "This Year", "All Time"];
    [ObservableProperty] private string _selectedStatsPeriod = "Today";
    [ObservableProperty] private string _binkpStatsText = "";
    [ObservableProperty] private string _binkpStatsCaption = "";

    [RelayCommand]
    public async Task LoadNetworksAsync(CancellationToken ct = default)
    {
        try
        {
            var names = await client.CallAsync<string[]>("fido.networks.list", null, ct)
                ?? [FidoNetworksViewModel.DefaultPrimaryNetwork];
            NetworkNames.Clear();
            foreach (var n in names) NetworkNames.Add(n);
            if (!NetworkNames.Contains(SelectedNetwork))
                SelectedNetwork = NetworkNames.FirstOrDefault() ?? FidoNetworksViewModel.DefaultPrimaryNetwork;
            await LoadLocalNodelistAsync(ct);
        }
        catch { /* ignore */ }
    }

    partial void OnSelectedNetworkChanged(string value)
    {
        if (string.IsNullOrEmpty(value)) return;
        _ = LoadLocalNodelistAsync();
        _ = CheckVersionAsync();
    }

    [RelayCommand]
    public async Task LoadLocalNodelistAsync(CancellationToken ct = default)
    {
        try
        {
            var r = await client.CallAsync<LocalNodelistListResult>("fido.nodelist.local.list",
                new { network = SelectedNetwork }, ct);
            LocalEditorNodes.Clear();
            _localNodesPendingDelete.Clear();
            foreach (var n in r?.Nodes ?? [])
                LocalEditorNodes.Add(LocalNodeEditRow.FromFidoNode(n));
            Status = $"{LocalEditorNodes.Count} local node(s) for {SelectedNetwork}.";
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private void AddLocalNode()
    {
        LocalEditorNodes.Add(new LocalNodeEditRow { IsNew = true, Address = "0:0/0" });
    }

    [RelayCommand]
    private void RemoveLocalNode()
    {
        if (SelectedLocalNode is null) return;
        if (!SelectedLocalNode.IsNew && !string.IsNullOrWhiteSpace(SelectedLocalNode.Address))
            _localNodesPendingDelete.Add(SelectedLocalNode.Address);
        LocalEditorNodes.Remove(SelectedLocalNode);
        SelectedLocalNode = null;
    }

    [RelayCommand]
    private async Task CommitLocalNodelistAsync(CancellationToken ct = default)
    {
        try
        {
            var upsert = LocalEditorNodes
                .Where(r => !string.IsNullOrWhiteSpace(r.Address))
                .Select(r => r.ToUpsertPayload())
                .ToList();
            var result = await client.CallAsync<LocalNodelistCommitResult>("fido.nodelist.local.commit",
                new { network = SelectedNetwork, upsert, delete = _localNodesPendingDelete.ToList() }, ct);
            _localNodesPendingDelete.Clear();
            Status = result?.Message ?? "Local nodelist committed.";
            await LoadLocalNodelistAsync(ct);
            await CheckVersionAsync(ct);
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task ExportNodelistAsync(CancellationToken ct = default)
    {
        try
        {
            var result = await client.CallAsync<NodelistExportResult>("fido.nodelist.export",
                new { network = SelectedNetwork, path = ExportPath }, ct);
            Status = result is null
                ? "Export complete."
                : $"Exported {result.Size} bytes to {result.Path}.";
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task SearchNodesAsync(CancellationToken ct = default)
    {
        try
        {
            NodePage = 1;
            await PageSearchAsync(ct);
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task NextPageAsync(CancellationToken ct = default)
    {
        if (NodePage >= NodeTotalPages) return;
        NodePage++;
        await PageSearchAsync(ct);
    }

    [RelayCommand]
    private async Task PrevPageAsync(CancellationToken ct = default)
    {
        if (NodePage <= 1) return;
        NodePage--;
        await PageSearchAsync(ct);
    }

    private async Task PageSearchAsync(CancellationToken ct)
    {
        var r = await client.CallAsync<NodelistSearchResult>("fido.nodes.search",
            new { network = SelectedNetwork, query = NodeQuery, page = NodePage, size = 25 }, ct);
        if (r is null) return;
        NodeResults.Clear();
        foreach (var n in r.Nodes ?? []) NodeResults.Add(n);
        NodeTotalPages = r.Pages;
        Status = $"{r.Total} node(s) found.";
    }

    [RelayCommand]
    private async Task SendNetmailAsync(CancellationToken ct = default)
    {
        if (string.IsNullOrWhiteSpace(ToAddr)) { Status = "Destination address required."; return; }
        try
        {
            var cfg = await client.CallAsync<BbsConfig>("config.get", null, ct);
            var fromAddr = cfg?.Fido.Address ?? "";
            await client.CallAsync("fido.netmail.send", new
            {
                FromName = "Sysop",
                FromAddr = fromAddr,
                ToAddr = ToAddr,
                ToName = ToName,
                Subject = NmSubject,
                Body = NmBody,
                Crash = Crash,
                Network = SelectedNetwork == FidoNetworksViewModel.DefaultPrimaryNetwork ? "" : SelectedNetwork,
            }, ct);
            Status = "Netmail queued.";
            ToAddr = ToName = NmSubject = NmBody = "";
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task ImportNodelistAsync(CancellationToken ct = default)
    {
        if (string.IsNullOrWhiteSpace(ImportPath)) { Status = "Path required."; return; }
        try
        {
            await client.CallAsync("fido.import.nodelist",
                new { path = ImportPath, network = SelectedNetwork }, ct);
            Status = "Nodelist imported.";
            await LoadLocalNodelistAsync(ct);
            await CheckVersionAsync(ct);
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task FetchNodelistAsync(CancellationToken ct = default)
    {
        try
        {
            await client.CallAsync("fido.nodelist.fetch", new { network = SelectedNetwork }, ct);
            Status = "Nodelist fetched and imported.";
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task TossAsync(CancellationToken ct = default)
    {
        try
        {
            await client.CallAsync("fido.toss", null, ct);
            Status = "Toss complete.";
            await RefreshBinkpLogAsync(ct);
            await RefreshBinkpStatsAsync(ct);
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task ScanAsync(CancellationToken ct = default)
    {
        try
        {
            await client.CallAsync("fido.scan", null, ct);
            Status = "Scan complete.";
            await RefreshBinkpLogAsync(ct);
            await RefreshBinkpStatsAsync(ct);
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task PollAsync(CancellationToken ct = default)
    {
        try
        {
            var result = await client.CallAsync<FidoPollAndTossResult>("fido.poll",
                new { network = SelectedNetwork }, ct);
            if (result?.Poll is { } poll)
            {
                Status = $"Poll complete — sent {poll.Sent?.Count ?? 0}, received {poll.Received?.Count ?? 0}.";
                if (result.Toss is { } toss)
                    Status += $" Toss: {toss.Imported} imported, {toss.Skipped} skipped, {toss.Orphaned} held.";
            }
            else
            {
                Status = "Poll complete.";
            }
            await RefreshBinkpLogAsync(ct);
            await RefreshBinkpStatsAsync(ct);
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    public async Task RefreshBinkpStatsAsync(CancellationToken ct = default)
    {
        try
        {
            var (period, periodKey) = MapStatsPeriod(SelectedStatsPeriod);
            var stats = await client.CallAsync<BinkpStatsResult>("fido.binkp.stats", new
            {
                network = SelectedNetwork,
                period,
                period_key = periodKey,
            }, ct);
            if (stats is null)
            {
                BinkpStatsText = "(no stats available)";
                BinkpStatsCaption = "";
                return;
            }
            stats.Networks ??= [];
            stats.Links ??= [];
            BinkpStatsCaption = $"{stats.Period} {stats.PeriodKey}".Trim();
            BinkpStatsText = FormatStatsText(stats);
        }
        catch (Exception ex) { Status = $"Stats error: {ex.Message}"; }
    }

    partial void OnSelectedStatsPeriodChanged(string value)
    {
        if (string.IsNullOrEmpty(value)) return;
        _ = RefreshBinkpStatsAsync();
    }

    private static (string period, string periodKey) MapStatsPeriod(string label)
    {
        var now = DateTime.Now;
        return label switch
        {
            "Yesterday" => ("day", now.AddDays(-1).ToString("yyyy-MM-dd")),
            "This Month" => ("month", now.ToString("yyyy-MM")),
            "This Year" => ("year", now.ToString("yyyy")),
            "All Time" => ("all", ""),
            _ => ("day", now.ToString("yyyy-MM-dd")),
        };
    }

    private static string FormatStatsText(BinkpStatsResult stats)
    {
        var networks = stats.Networks ?? [];
        var links = stats.Links ?? [];

        if (networks.Count == 0)
            return "No BinkP activity recorded for this period.";

        var sb = new StringBuilder();
        foreach (var n in networks)
        {
            sb.AppendLine($"Network: {n.Network}");
            sb.AppendLine(new string('-', 50));
            sb.AppendLine($"Outbound polls (OK/fail):     {n.PollClientOK} / {n.PollClientFail}");
            sb.AppendLine($"  files sent/received:        {n.PollClientFilesSent} / {n.PollClientFilesRecv}");
            sb.AppendLine($"Inbound uplink (OK/fail):   {n.PollServerUplinkOK} / {n.PollServerUplinkFail}");
            sb.AppendLine($"  files sent/received:        {n.PollServerUplinkSent} / {n.PollServerUplinkRecv}");
            sb.AppendLine($"Inbound downlink (OK/fail): {n.PollServerDownlinkOK} / {n.PollServerDownlinkFail}");
            sb.AppendLine($"  files sent/received:        {n.PollServerDownlinkSent} / {n.PollServerDownlinkRecv}");
            sb.AppendLine($"Netmail received/sent:      {n.NetmailRecv} / {n.NetmailSent}");
            sb.AppendLine($"Echomail received/sent:     {n.EchomailRecv} / {n.EchomailSent}");
            sb.AppendLine($"Toss imported/skipped/held: {n.TossImported} / {n.TossSkipped} / {n.TossHeld}");
            sb.AppendLine($"Packets tossed:             {n.TossPackets}");
            if (n.SessionErrors > 0)
                sb.AppendLine($"Session errors:             {n.SessionErrors}");

            var netLinks = links.Where(l => l.Network == n.Network).ToList();
            if (netLinks.Count > 0)
            {
                sb.AppendLine();
                sb.AppendLine("Link detail:");
                foreach (var l in netLinks)
                {
                    sb.AppendLine($"  {l.LinkType} {l.PeerKey}: poll {l.PollOK}/{l.PollFail}, " +
                                  $"files {l.FilesSent}/{l.FilesRecv}, " +
                                  $"nm {l.NetmailSent}/{l.NetmailRecv}, echo {l.EchomailSent}/{l.EchomailRecv}");
                }
            }
            sb.AppendLine();
        }
        return sb.ToString().TrimEnd();
    }

    [RelayCommand]
    public async Task RefreshBinkpLogAsync(CancellationToken ct = default)
    {
        try
        {
            var log = await client.CallAsync<BinkpLogResult>("fido.binkp.log", new { lines = 300 }, ct);
            if (log is null) return;
            BinkpLogPath = string.IsNullOrWhiteSpace(log.Path) ? "" : log.Path;
            var lines = log.Lines ?? [];
            BinkpLogText = lines.Count == 0
                ? "(no BinkP sessions logged yet)"
                : string.Join(Environment.NewLine, lines);
        }
        catch (Exception ex) { Status = $"Log error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task CheckVersionAsync(CancellationToken ct = default)
    {
        try
        {
            var v = await client.CallAsync<NodelistVersion>("fido.nodelist.version",
                new { network = SelectedNetwork }, ct);
            VersionText = v is null
                ? $"No nodelist imported yet for '{SelectedNetwork}'."
                : $"{v.Network}: last imported {v.ImportedAt}, {v.NodeCount} node(s).";
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }
}

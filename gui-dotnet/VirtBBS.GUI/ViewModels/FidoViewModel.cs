using System;
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

    // Nodelist import.
    [ObservableProperty] private string _importPath = "";
    [ObservableProperty] private string _versionText = "";

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
        }
        catch { /* ignore */ }
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
        foreach (var n in r.Nodes) NodeResults.Add(n);
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
                Status = $"Poll complete — sent {poll.Sent.Count}, received {poll.Received.Count}.";
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
                return;
            }
            BinkpStatsCaption = $"{stats.Period} {stats.PeriodKey}".Trim();
            BinkpStatsText = FormatStatsText(stats);
        }
        catch (Exception ex) { Status = $"Stats error: {ex.Message}"; }
    }

    partial void OnSelectedStatsPeriodChanged(string value) =>
        _ = RefreshBinkpStatsAsync();

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
        if (stats.Networks.Count == 0)
            return "No BinkP activity recorded for this period.";

        var sb = new StringBuilder();
        foreach (var n in stats.Networks)
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

            var links = stats.Links.Where(l => l.Network == n.Network).ToList();
            if (links.Count > 0)
            {
                sb.AppendLine();
                sb.AppendLine("Link detail:");
                foreach (var l in links)
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
            BinkpLogText = log.Lines.Count == 0
                ? "(no BinkP sessions logged yet)"
                : string.Join(Environment.NewLine, log.Lines);
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

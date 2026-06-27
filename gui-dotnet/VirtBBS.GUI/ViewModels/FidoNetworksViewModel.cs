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

public partial class FidoNetworksViewModel(ApiClient client) : ViewModelBase
{
    public const string PrimaryNetwork = "FidoNet";

    [ObservableProperty] private string _status = "";
    [ObservableProperty] private string _selectedNetwork = PrimaryNetwork;
    [ObservableProperty] private bool _isPrimaryNetwork = true;
    [ObservableProperty] private bool _enabled;
    [ObservableProperty] private string _address = "";
    [ObservableProperty] private string _uplink = "";
    [ObservableProperty] private string _password = "";
    [ObservableProperty] private string _inboundDir = "";
    [ObservableProperty] private string _outboundDir = "";
    [ObservableProperty] private string _nodelistDir = "";
    [ObservableProperty] private int _binkpPort = 24554;
    [ObservableProperty] private string _taglinesFile = "";
    [ObservableProperty] private string _areafixPassword = "";
    [ObservableProperty] private string _filefixPassword = "";
    [ObservableProperty] private int _pollIntervalMins;
    [ObservableProperty] private string _nodelistUrl = "";
    [ObservableProperty] private int _nodelistUpdateIntervalHours;
    [ObservableProperty] private string _nodelistEchoTag = "";
    [ObservableProperty] private string _akasText = "";

    public ObservableCollection<string> NetworkNames { get; } = [];
    public ObservableCollection<AreaMapRow> InboundAreas { get; } = [];
    public ObservableCollection<FileAreaMapRow> FileAreaMaps { get; } = [];
    public ObservableCollection<FidoDownlink> Downlinks { get; } = [];
    public ObservableCollection<AreaFixSubscription> AreaFixSubs { get; } = [];

    private BbsConfig? _cachedConfig;

    partial void OnSelectedNetworkChanged(string value)
    {
        IsPrimaryNetwork = value == PrimaryNetwork;
        if (_cachedConfig is not null)
            ApplyNetworkToForm(value);
    }

    [RelayCommand]
    public async Task LoadAsync(CancellationToken ct = default)
    {
        try
        {
            var names = await client.CallAsync<string[]>("fido.networks.list", null, ct) ?? [PrimaryNetwork];
            NetworkNames.Clear();
            foreach (var n in names) NetworkNames.Add(n);

            _cachedConfig = await client.CallAsync<BbsConfig>("config.get", null, ct);
            if (_cachedConfig is null) return;

            if (!NetworkNames.Contains(SelectedNetwork))
                SelectedNetwork = NetworkNames.FirstOrDefault() ?? PrimaryNetwork;

            ApplyNetworkToForm(SelectedNetwork);
            await LoadAreaFixSubsAsync(ct);
            Status = "Network settings loaded.";
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    private void ApplyNetworkToForm(string network)
    {
        if (_cachedConfig is null) return;
        var src = NetworkSource(_cachedConfig, network);

        Enabled = src.Enabled;
        Address = src.Address;
        Uplink = src.Uplink;
        Password = src.Password;
        InboundDir = src.InboundDir;
        OutboundDir = src.OutboundDir;
        NodelistDir = src.NodelistDir;
        BinkpPort = src.BinkpPort;
        TaglinesFile = src.TaglinesFile;
        AreafixPassword = src.AreaFixPassword;
        FilefixPassword = src.FileFixPassword;
        PollIntervalMins = src.PollIntervalMins;
        NodelistUrl = src.NodelistURL;
        NodelistUpdateIntervalHours = src.NodelistUpdateIntervalHours;
        NodelistEchoTag = src.NodelistEchoTag;
        AkasText = string.Join("\n", src.AKAs ?? []);

        InboundAreas.Clear();
        foreach (var kv in src.Areas ?? [])
            InboundAreas.Add(new AreaMapRow { Tag = kv.Key, ConferenceId = kv.Value });

        FileAreaMaps.Clear();
        foreach (var kv in src.FileAreas ?? [])
            FileAreaMaps.Add(new FileAreaMapRow { Tag = kv.Key, DirId = kv.Value });

        Downlinks.Clear();
        foreach (var dl in src.Downlinks ?? [])
            Downlinks.Add(new FidoDownlink
            {
                Name = dl.Name,
                Address = dl.Address,
                Password = dl.Password,
                AKAs = new List<string>(dl.AKAs ?? []),
            });
    }

    private static FidoNetworkDef NetworkSource(BbsConfig cfg, string network)
    {
        FidoNetworkDef src;
        if (network == PrimaryNetwork)
        {
            src = new FidoNetworkDef
            {
                Name = PrimaryNetwork,
                Enabled = cfg.Fido.Enabled,
                Address = cfg.Fido.Address,
                Uplink = cfg.Fido.Uplink,
                Password = cfg.Fido.Password,
                InboundDir = cfg.Fido.InboundDir,
                OutboundDir = cfg.Fido.OutboundDir,
                NodelistDir = cfg.Fido.NodelistDir,
                BinkpPort = cfg.Fido.BinkpPort,
                TaglinesFile = cfg.Fido.TaglinesFile,
                AreaFixPassword = cfg.Fido.AreaFixPassword,
                FileFixPassword = cfg.Fido.FileFixPassword,
                PollIntervalMins = cfg.Fido.PollIntervalMins,
                NodelistURL = cfg.Fido.NodelistURL,
                NodelistUpdateIntervalHours = cfg.Fido.NodelistUpdateIntervalHours,
                AKAs = cfg.Fido.AKAs,
                Areas = cfg.Fido.Areas,
                FileAreas = cfg.Fido.FileAreas,
                Downlinks = cfg.Fido.Downlinks,
            };
        }
        else
        {
            src = cfg.Fido.Networks?.FirstOrDefault(n => n.Name == network)
                ?? new FidoNetworkDef { Name = network };
        }

        src.AKAs ??= [];
        src.Areas ??= new Dictionary<string, int>();
        src.FileAreas ??= new Dictionary<string, int>();
        src.Downlinks ??= [];
        return src;
    }

    [RelayCommand]
    private async Task SaveAsync(CancellationToken ct = default)
    {
        if (_cachedConfig is null)
        {
            await LoadAsync(ct);
            if (_cachedConfig is null) return;
        }

        var areas = new Dictionary<string, int>();
        foreach (var row in InboundAreas)
            if (!string.IsNullOrWhiteSpace(row.Tag))
                areas[row.Tag.Trim()] = row.ConferenceId;

        var fileAreas = new Dictionary<string, int>();
        foreach (var row in FileAreaMaps)
            if (!string.IsNullOrWhiteSpace(row.Tag))
                fileAreas[row.Tag.Trim()] = row.DirId;

        var downlinks = Downlinks.Select(dl => new FidoDownlink
        {
            Name = dl.Name,
            Address = dl.Address,
            Password = dl.Password,
            AKAs = new List<string>(dl.AKAs ?? []),
        }).ToList();

        var akas = AkasText.Split('\n', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries).ToList();

        if (IsPrimaryNetwork)
        {
            _cachedConfig.Fido.Enabled = Enabled;
            _cachedConfig.Fido.Address = Address;
            _cachedConfig.Fido.Uplink = Uplink;
            _cachedConfig.Fido.Password = Password;
            _cachedConfig.Fido.InboundDir = InboundDir;
            _cachedConfig.Fido.OutboundDir = OutboundDir;
            _cachedConfig.Fido.NodelistDir = NodelistDir;
            _cachedConfig.Fido.BinkpPort = BinkpPort;
            _cachedConfig.Fido.TaglinesFile = TaglinesFile;
            _cachedConfig.Fido.AreaFixPassword = AreafixPassword;
            _cachedConfig.Fido.FileFixPassword = FilefixPassword;
            _cachedConfig.Fido.PollIntervalMins = PollIntervalMins;
            _cachedConfig.Fido.NodelistURL = NodelistUrl;
            _cachedConfig.Fido.NodelistUpdateIntervalHours = NodelistUpdateIntervalHours;
            _cachedConfig.Fido.AKAs = akas;
            _cachedConfig.Fido.Areas = areas;
            _cachedConfig.Fido.FileAreas = fileAreas;
            _cachedConfig.Fido.Downlinks = downlinks;
        }
        else
        {
            _cachedConfig.Fido.Networks ??= [];
            var nd = _cachedConfig.Fido.Networks.FirstOrDefault(n => n.Name == SelectedNetwork);
            if (nd is null)
            {
                nd = new FidoNetworkDef { Name = SelectedNetwork };
                _cachedConfig.Fido.Networks.Add(nd);
            }
            nd.Enabled = Enabled;
            nd.Address = Address;
            nd.Uplink = Uplink;
            nd.Password = Password;
            nd.InboundDir = InboundDir;
            nd.OutboundDir = OutboundDir;
            nd.NodelistDir = NodelistDir;
            nd.BinkpPort = BinkpPort;
            nd.TaglinesFile = TaglinesFile;
            nd.AreaFixPassword = AreafixPassword;
            nd.FileFixPassword = FilefixPassword;
            nd.PollIntervalMins = PollIntervalMins;
            nd.NodelistURL = NodelistUrl;
            nd.NodelistUpdateIntervalHours = NodelistUpdateIntervalHours;
            nd.NodelistEchoTag = NodelistEchoTag;
            nd.AKAs = akas;
            nd.Areas = areas;
            nd.FileAreas = fileAreas;
            nd.Downlinks = downlinks;
        }

        try
        {
            await client.CallAsync("config.update", new { fido = _cachedConfig.Fido }, ct);
            Status = $"Network '{SelectedNetwork}' saved.";
            await LoadAreaFixSubsAsync(ct);
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private void AddInboundArea() => InboundAreas.Add(new AreaMapRow());

    [RelayCommand]
    private void RemoveInboundArea(AreaMapRow? row)
    {
        if (row is not null) InboundAreas.Remove(row);
    }

    [RelayCommand]
    private void AddFileAreaMap() => FileAreaMaps.Add(new FileAreaMapRow());

    [RelayCommand]
    private void RemoveFileAreaMap(FileAreaMapRow? row)
    {
        if (row is not null) FileAreaMaps.Remove(row);
    }

    [RelayCommand]
    private void AddDownlink() => Downlinks.Add(new FidoDownlink());

    [RelayCommand]
    private void RemoveDownlink(FidoDownlink? dl)
    {
        if (dl is not null) Downlinks.Remove(dl);
    }

    [RelayCommand]
    private async Task AddNetworkAsync(CancellationToken ct = default)
    {
        var name = $"Network{(_cachedConfig?.Fido.Networks.Count ?? 0) + 1}";
        _cachedConfig ??= await client.CallAsync<BbsConfig>("config.get", null, ct) ?? new BbsConfig();
        _cachedConfig.Fido.Networks ??= [];
        _cachedConfig.Fido.Networks.Add(new FidoNetworkDef { Name = name, Enabled = true });
        await SaveFullFidoAsync(ct);
        if (!NetworkNames.Contains(name)) NetworkNames.Add(name);
        SelectedNetwork = name;
        Status = $"Added network '{name}'.";
    }

    private async Task SaveFullFidoAsync(CancellationToken ct)
    {
        if (_cachedConfig is null) return;
        await client.CallAsync("config.update", new { fido = _cachedConfig.Fido }, ct);
        await LoadAsync(ct);
    }

    private async Task LoadAreaFixSubsAsync(CancellationToken ct)
    {
        try
        {
            var subs = await client.CallAsync<AreaFixSubscription[]>("fido.areafix.subscriptions",
                new { network = SelectedNetwork }, ct) ?? [];
            AreaFixSubs.Clear();
            foreach (var s in subs) AreaFixSubs.Add(s);
        }
        catch { /* optional */ }
    }
}

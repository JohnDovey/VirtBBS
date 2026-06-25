// ============================================================================
// VirtBBS — A modern BBS server inspired by PCBoard BBS
//           (Clark Development Company, 1987-1996)
//
// Copyright (c) 2026 John Dovey <dovey.john@gmail.com>
//
// MIT License
//
// Permission is hereby granted, free of charge, to any person obtaining a
// copy of this software and associated documentation files (the "Software"),
// to deal in the Software without restriction, including without limitation
// the rights to use, copy, modify, merge, publish, distribute, sublicense,
// and/or sell copies of the Software, and to permit persons to whom the
// Software is furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included
// in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS
// OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
// THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
// DEALINGS IN THE SOFTWARE.
//
// Change History:
//   v0.0.8  2026-06-24  Avalonia GUI: FidoNet tab view model
// ============================================================================

using System;
using System.Collections.ObjectModel;
using System.Threading;
using System.Threading.Tasks;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using VirtBBS.GUI.Models;

namespace VirtBBS.GUI.ViewModels;

public partial class FidoViewModel(ApiClient client) : ViewModelBase
{
    [ObservableProperty] private string _status = "";

    // Nodelist search.
    [ObservableProperty] private string _nodeQuery      = "";
    [ObservableProperty] private string _nodeNetwork    = "";
    [ObservableProperty] private int    _nodePage       = 1;
    [ObservableProperty] private int    _nodeTotalPages = 1;
    [ObservableProperty] private FidoNode? _selectedNode;

    public ObservableCollection<FidoNode> NodeResults { get; } = [];

    // Netmail compose.
    [ObservableProperty] private string _toAddr    = "";
    [ObservableProperty] private string _toName    = "";
    [ObservableProperty] private string _nmSubject = "";
    [ObservableProperty] private string _nmBody    = "";
    [ObservableProperty] private bool   _crash     = false;

    // Echo flags (conference).
    [ObservableProperty] private int    _econfId         = 0;
    [ObservableProperty] private bool   _econfActive     = false;
    [ObservableProperty] private string _econfAreaTag    = "";
    [ObservableProperty] private string _econfUplinkAddr = "";
    [ObservableProperty] private string _econfNetwork    = "";

    // Nodelist import.
    [ObservableProperty] private string _importPath    = "";
    [ObservableProperty] private string _importNetwork = "";

    // Toss/Scan/Poll.
    [ObservableProperty] private string _pollNetwork = "";

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
        try
        {
            var r = await client.CallAsync<NodelistSearchResult>("fido.nodes.search",
                new { network = NodeNetwork, query = NodeQuery, page = NodePage, page_size = 25 }, ct);
            if (r is null) return;
            NodeResults.Clear();
            foreach (var n in r.Nodes) NodeResults.Add(n);
            NodeTotalPages = r.Pages;
            Status = $"{r.Total} node(s) found.";
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task SendNetmailAsync(CancellationToken ct = default)
    {
        if (string.IsNullOrWhiteSpace(ToAddr)) { Status = "Destination address required."; return; }
        try
        {
            await client.CallAsync("fido.netmail.send", new
            {
                to_addr = ToAddr,
                to_name = ToName,
                subject = NmSubject,
                body    = NmBody,
                crash   = Crash,
            }, ct);
            Status = "Netmail queued.";
            ToAddr = ToName = NmSubject = NmBody = "";
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task SaveEchoFlagsAsync(CancellationToken ct = default)
    {
        try
        {
            await client.CallAsync("conferences.update", new
            {
                id          = EconfId,
                active      = EconfActive,
                area_tag    = EconfAreaTag,
                uplink_addr = EconfUplinkAddr,
                network     = EconfNetwork,
            }, ct);
            Status = "Echo flags saved.";
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
                new { path = ImportPath, network = ImportNetwork }, ct);
            Status = "Nodelist import started.";
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
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task PollAsync(CancellationToken ct = default)
    {
        try
        {
            await client.CallAsync("fido.poll", new { network = PollNetwork }, ct);
            Status = "Poll initiated.";
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }
}

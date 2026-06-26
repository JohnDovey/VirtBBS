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
//   v0.9.0  2026-06-26  Sysop GUI gap-fill: lists/revokes the API tokens
//                        users generate for VirtAnd/VirtTerm (internal/userapi),
//                        a capability that previously existed only as
//                        self-service in the BBS terminal's profile menu.
// ============================================================================

using System;
using System.Collections.ObjectModel;
using System.Threading;
using System.Threading.Tasks;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using VirtBBS.GUI.Models;

namespace VirtBBS.GUI.ViewModels;

public partial class TokensViewModel(ApiClient client) : ViewModelBase
{
    [ObservableProperty] private string _status = "";
    [ObservableProperty] private ApiToken? _selectedToken;

    public ObservableCollection<ApiToken> Tokens { get; } = [];

    [RelayCommand]
    public async Task LoadAsync(CancellationToken ct = default)
    {
        try
        {
            var list = await client.CallAsync<ApiToken[]>("tokens.list", null, ct) ?? [];
            Tokens.Clear();
            foreach (var t in list) Tokens.Add(t);
            Status = $"{Tokens.Count} token(s) total ({Array.FindAll(list, t => t.IsActive).Length} active).";
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }

    [RelayCommand]
    private async Task RevokeAsync(CancellationToken ct = default)
    {
        if (SelectedToken is null) { Status = "Select a token first."; return; }
        if (!SelectedToken.IsActive) { Status = "Token is already revoked."; return; }
        try
        {
            await client.CallAsync("tokens.revoke", new { id = SelectedToken.ID }, ct);
            Status = $"Revoked token for {SelectedToken.UserName} ({SelectedToken.DeviceLabel}).";
            await LoadAsync(ct);
        }
        catch (Exception ex) { Status = $"Error: {ex.Message}"; }
    }
}

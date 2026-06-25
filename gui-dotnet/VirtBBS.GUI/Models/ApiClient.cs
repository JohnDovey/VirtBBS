// VirtBBS GUI — ApiClient.cs
// JSON-over-TCP client for the VirtBBS management API.
// Each call opens a fresh TCP connection, sends one JSON request, reads one JSON response.

using System;
using System.Net.Sockets;
using System.Text;
using System.Text.Json;
using System.Text.Json.Nodes;
using System.Threading;
using System.Threading.Tasks;

namespace VirtBBS.GUI.Models;

public class ApiClient
{
    public string Host     { get; set; } = "127.0.0.1";
    public int    Port     { get; set; } = 9999;
    public string Username { get; set; } = "sysop";
    public string Password { get; set; } = "";

    private static readonly JsonSerializerOptions _opts = new()
    {
        PropertyNamingPolicy        = null,   // keep as-is; server uses lowercase_underscore
        PropertyNameCaseInsensitive = true,   // server's untagged Go structs marshal as exact PascalCase
        WriteIndented               = false,
        DefaultIgnoreCondition      = System.Text.Json.Serialization.JsonIgnoreCondition.WhenWritingNull
    };

    /// <summary>
    /// Send a request and return the parsed result node (may be null on success with empty body).
    /// Throws ApiException on server-side errors or connection failures.
    /// </summary>
    public async Task<JsonNode?> CallAsync(string method, object? @params = null,
                                           CancellationToken ct = default)
    {
        var req = new
        {
            method,
            @params,
            auth = new { user = Username, password = Password }
        };

        string reqJson = JsonSerializer.Serialize(req, _opts) + "\n";
        byte[] reqBytes = Encoding.UTF8.GetBytes(reqJson);

        using var tcp = new TcpClient();
        tcp.NoDelay = true;

        await tcp.ConnectAsync(Host, Port, ct);
        var stream = tcp.GetStream();

        await stream.WriteAsync(reqBytes, ct);

        // Read response (newline-delimited JSON).
        var sb = new StringBuilder();
        var buf = new byte[4096];
        while (true)
        {
            int n = await stream.ReadAsync(buf, ct);
            if (n == 0) break;
            sb.Append(Encoding.UTF8.GetString(buf, 0, n));
            if (sb.ToString().Contains('\n')) break;
        }

        var respJson = sb.ToString().Trim();
        if (string.IsNullOrEmpty(respJson))
            throw new ApiException("Empty response from server.");

        var node = JsonNode.Parse(respJson)
                   ?? throw new ApiException("Invalid JSON from server.");

        var errNode = node["error"];
        if (errNode is not null && errNode.GetValueKind() != JsonValueKind.Null)
        {
            var errMsg = errNode.ToString();
            if (!string.IsNullOrEmpty(errMsg))
                throw new ApiException(errMsg);
        }

        return node["result"];
    }

    /// <summary>Convenience: deserialise result into T.</summary>
    public async Task<T?> CallAsync<T>(string method, object? @params = null,
                                       CancellationToken ct = default)
    {
        var node = await CallAsync(method, @params, ct);
        if (node is null) return default;
        return node.Deserialize<T>(_opts);
    }

    /// <summary>Quick connectivity test — attempts to list nodes.</summary>
    public async Task<bool> TestConnectionAsync(CancellationToken ct = default)
    {
        try
        {
            await CallAsync("nodes.list", null, ct);
            return true;
        }
        catch { return false; }
    }
}

public class ApiException(string message) : Exception(message);

using System.Collections.Generic;
using System.Text.Json.Serialization;
using CommunityToolkit.Mvvm.ComponentModel;

namespace VirtBBS.GUI.Models;

/// <summary>Editable row for the local nodelist editor grid.</summary>
public partial class LocalNodeEditRow : ObservableObject
{
    [ObservableProperty] private string _address = "";
    [ObservableProperty] private string _name = "";
    [ObservableProperty] private string _location = "";
    [ObservableProperty] private string _sysop = "";
    [ObservableProperty] private string _phone = "-Unpublished-";
    [ObservableProperty] private int _baud = 33600;
    [ObservableProperty] private string _flags = "";
    [ObservableProperty] private string _type = "Node";
    [ObservableProperty] private bool _active = true;
    [ObservableProperty] private bool _isNew;

    public static LocalNodeEditRow FromFidoNode(FidoNode n) => new()
    {
        Address = n.Address,
        Name = n.Name,
        Location = n.Location,
        Sysop = n.Sysop,
        Phone = n.Phone,
        Baud = n.Baud,
        Flags = n.Flags,
        Type = n.Type,
        Active = n.Active,
    };

    public object ToUpsertPayload() => new
    {
        address = Address,
        name = Name,
        location = Location,
        sysop = Sysop,
        phone = Phone,
        baud = Baud,
        flags = Flags,
        type = Type,
        active = Active,
    };
}

public class LocalNodelistListResult
{
    [JsonPropertyName("nodes")] public List<FidoNode> Nodes { get; set; } = [];
}

public class LocalNodelistCommitResult
{
    [JsonPropertyName("node_count")]     public int    NodeCount     { get; set; }
    [JsonPropertyName("nodelist_file")] public string NodelistFile { get; set; } = "";
    [JsonPropertyName("nodediff_file")] public string NodediffFile { get; set; } = "";
    [JsonPropertyName("netmail_sent")]  public bool   NetmailSent   { get; set; }
    [JsonPropertyName("netmail_to")]    public string NetmailTo    { get; set; } = "";
    [JsonPropertyName("message")]       public string Message       { get; set; } = "";
}

public class NodelistExportResult
{
    [JsonPropertyName("path")] public string Path { get; set; } = "";
    [JsonPropertyName("size")] public int    Size { get; set; }
}

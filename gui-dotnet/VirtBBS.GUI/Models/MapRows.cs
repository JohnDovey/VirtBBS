using CommunityToolkit.Mvvm.ComponentModel;

namespace VirtBBS.GUI.Models;

public partial class AreaMapRow : ObservableObject
{
    [ObservableProperty] private string _tag = "";
    [ObservableProperty] private int _conferenceId;
}

public partial class FileAreaMapRow : ObservableObject
{
    [ObservableProperty] private string _tag = "";
    [ObservableProperty] private int _dirId;
}

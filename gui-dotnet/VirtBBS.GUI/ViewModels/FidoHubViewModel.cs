using System.Threading;
using System.Threading.Tasks;
using VirtBBS.GUI.Models;

namespace VirtBBS.GUI.ViewModels;

public class FidoHubViewModel(ApiClient client)
{
    public FidoNetworksViewModel Networks { get; } = new(client);
    public FidoRoutingViewModel Routing { get; } = new(client);
    public FidoViewModel Operations { get; } = new(client);

    public async Task LoadAllAsync(CancellationToken ct = default)
    {
        await Networks.LoadAsync(ct);
        Routing.SelectedNetwork = Networks.SelectedNetwork;
        await Routing.LoadAsync(ct);
        Operations.SelectedNetwork = Networks.SelectedNetwork;
    }
}

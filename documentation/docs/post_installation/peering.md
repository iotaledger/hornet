---
keywords:
- IOTA Node 
- Hornet Node
- configuration
- peer
- peering
- identity
- autopeering
description: How to configure your nodes peers, neighbors and auto-peering. 
image: /img/logo/HornetLogo.png
---

# Peering Configuration

The IOTA network is a distributed network. It uses a gossip protocol to broadcast data among IOTA nodes. To participate in a network, each node has to establish a secure connection to other nodes in the network (to its peer neighbors), and mutually exchange messages.

## Node Identity

Each node can be uniquely identified by a `peer identity`. `Peer identity` (also called `PeerId`) is represented by a public
and private key pair. Since `PeerId` is a cryptographic hash of peer's public key, the `PeerId` represents a verifiable link between the given peer and its public key. It enables individual peers to establish a secure communication channel as the hash can be used to verify an identity of the peer.

When it is started for the first time, Hornet will automatically generate a `PeerId`, and save the identity's private key in the `./p2pstore/identity.key` file. Hornet will keep the generated identity between subsequent restarts.

Each time Hornet starts, the `PeerId` is written to stdout:

```plaintext
2021-08-23T17:17:50Z	INFO	  P2P	    WARNING: never share your "p2pstore" folder as it contains your node's private key!
2021-08-23T17:17:50Z	INFO	  P2P	    stored new private key for peer identity under "p2pstore/identity.key"
2021-08-23T17:17:50Z	INFO	  P2P     peer configured, ID: 12D3KooWQPA5woZRTT9ExAsq6gRrcPqf3bVn5M5taopJ5Uvhw4iz

```

Your `PeerId` is an essential part of your `multiaddr` used to configure neighbors. For example,  `/dns/example.com/tcp/15600/p2p/12D3KooWHiPg9gzmy1cbTFAUekyLHQKQKvsKmhzB7NJ5xnhK4WKq`, where `12D3KooWHiPg9gzmy1cbTFAUekyLHQKQKvsKmhzB7NJ5xnhK4WKq` corresponds to your `PeerId`. Your `PeerId` is also visible on the start page of the dashboard.

However, you can pre-generate the identity if you want. This way you can pre-communicate it to your peers before you even start your node.

You can use the `p2pidentity-gen` CLI tool to generate a `PeerId` which simply generates a private key file and logs the output to stdout:

```bash
./hornet tools p2pidentity-gen
```

Example output:

```plaintext
Your p2p private key (hex):   64166cc2627c369283cb1dd8412b6259232653611a3ae5cfb52398c23cfaead76af7a3cb775895046b9f28f2cf2b4150a9fd5dfd0ecf5c8d94529818578f40a2
Your p2p public key (hex):    6af7a3cb775895046b9f28f2cf2b4150a9fd5dfd0ecf5c8d94529818578f40a2
Your p2p public key (base58): 8CZELJwB3aBzxJgnLMvvt1FirAwNN6jif9LavYTNHCty
Your p2p PeerID:              12D3KooWH1vQ5SWtEUTVNCCCxxgGeoLZLVz1PnpCcjhcJXcGB9cu
```

:::info
Always take care of your `./p2pstore/identity.key` file, since it contains the private key to your nodes identity.
:::

More information regarding the `PeerId` is available on the [libp2p docs page](https://docs.libp2p.io/concepts/peer-id/).

## Addressing Peer Neighbors

In order to communicate to your peer neighbors, you will need an address to reach them.  To achieve that, Hornet uses the `MultiAddresses` format (also known as `multiaddr`).

`multiAddr` is a convention on how to encode multiple layers of addressing information into a single path structure that is future-proof. In other words, `multiaddr` is able to combine several pieces of information in a single human-readable and machine-optimized string, including network protocol and [`PeerId`](#node-identity).

For example, a node is reachable using IPv4 `100.1.1.1` using `TCP` on port `15600` and its `PeerId`
is `12D3KooWHjcCgWPnUEP8wNdbL2fx63Cmosk16xyZ25iUZagxmHb4`. A `multiaddr` encoding of this information would look like this:

```plaintext
/ip4/100.1.1.1/tcp/15600/p2p/12D3KooWHjcCgWPnUEP8wNdbL2fx63Cmosk16xyZ25iUZagxmHb4
```

:::info
Note how `ip4` is used. A common mistake is to use `ipv4`.
:::

If a node is reachable using a DNS name (for example `node01.iota.org`), then the given `multiaddr` would be:

```plaintext
/dns/node01.iota.org/tcp/15600/p2p/12D3KooWHjcCgWPnUEP8wNdbL2fx63Cmosk16xyZ25iUZagxmHb4
```

You will need to find out your own `multiaddr` to give to your peers for neighboring. To do so, combine the `peerId` you have gotten
from the stdout when the Hornet node started up (or which was shown via the `p2pidentity-gen` CLI tool), and your
configured `p2p.bindAddress`. Replace the `/ip4/<ip_address>`/`/dns/<hostname>` segments with the actual information.

You can find more information about `multiaddr` at the [libp2p docs page](https://docs.libp2p.io/concepts/addressing/).

## Adding Node Peers

Once you know your node's own `multiaddr`, it can be exchanged with other node owners to establish a mutual peer connection. We recommended a number of peer neighbors between 4-6 to get some degree of redundancy.

## Finding Neighbors

You can join the official IOTA Discord server and the `#nodesharing` channel.  There you will be able to describe your node location (Europe /
Germany / Asia, etc.), with your allocated high watermark resources and ask for neighbors.

:::warning
Do not publicly disclose your node `multiaddr` to all readers but wait for an individual direct chat.
:::

You can add peers using the Hornet [dashboard](post_installation.md#dashboard). To do so, go to *Peers* and click on *Add Peer*.  You can also add peers on the [peering.json](peering.md) file.

You can change the path or name of the `peering.json` file by using the `-n` or `--peeringConfig` argument while
executing the `hornet` executable.

This is `peering.json` example, with `ip4`, `ip6` and `dns` peers:

```json
{
  "peers": [
    {
      "alias": "Node1",
      "multiAddress": "/ip4/192.0.2.0/tcp/15600/p2p/12D3KooWCKWcTWevORKa2KEBputEGASvEBuDfRDSbe8t1DWugUmL"
    },
    {
      "alias": "Node2",
      "multiAddress": "/ip6/2001:db8:3333:4444:5555:6666:7777:8888/tcp/16600/p2p/12D3KooWJDqHjhd8us8XdbKy1Adp5nV6XoI7XhjZbPWAfbAbkLbH"
    },
    {
      "alias": "Node3",
      "multiAddress": "/dns/example.com/tcp/15600/p2p/12D3KooWN7F4eRAYbavnasME8WGXwkrpzWWoZSXfNSEpudmWi9YP"
    }
  ]
}
```

## Autopeering

Hornet also supports automatically finding peers through the _autopeering_ module. To minimize service distribution in case your autopeered peers are flaky, we recommend to only use autopeering if you have at least 4 static peers.

Autopeering is disabled by default. If you want to enable it, add `"Autopeering"` to `node.enablePlugins`.

WARNING: The autopeering plugin will disclose your public IP address to possibly all nodes and entry points.
Do not enable this plugin if you do not want this to happen!

Your node will use the specified entry nodes under `p2p.autopeering.entryNodes` to find new peers. `entryNodes` are also encoded as `multiaddr`:

```
/ip4/45.12.34.43/udp/14626/autopeering/8CZELJwB3aBzxJgnLMvvt1FirAwNN6jif9LavYTNHCty
```

Where the `/autopeering` portion defines the base58 encoded Ed25519 public key.

By default, Hornet will peer up to 4 autopeered peers and initiate a gossip protocol with them. Autopeered peers are not subject to connection trimming, the same way as mutually tethered peers aren't either.

### Entry Node

If you want to run your own node as an autopeering entry node, you should enable `p2p.autopeering.runAsEntryNode`. The base58 encoded public key is in the output of the `p2pidentity-gen` Hornet tool. Alternatively, if you already have an identity in a `./p2pstore`, you can use the `p2pidentity-extract` Hornet tool to extract it.

### Low/High Watermark

The `p2p.connectionManager.highWatermark` and `p2p.connectionManager.lowWatermark` configuration options define "watermark" points.  Watermark points can be thought of as a filling basin where if the `highWatermark` is reached, water will be drained until it reaches the `lowWatermark` again. Similarly, the connection manager within Hornet will start trimming away connections to peers if `highWatermark` peers are connected until it reaches `lowWatermark` count of peers. These watermarks exist for a certain buffer number of peers to be connected, which will not necessarily be targeted by the gossip protocol.

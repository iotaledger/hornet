# Peering Configuration

The IOTA network is a distributed network in which data is broadcasted among IOTA nodes through a gossip protocol. To be
able to participate in a network, each node has to establish a secure connection to other nodes in the network - to its
peer neighbors - and mutually exchange messages.

### Node Identity

Each node is uniquely identified by a `peer identity`. `Peer identity` (also called `PeerId`) is represented by a public
and private key pair. The `PeerId` represents a verifiable link between the given peer and its public key,
since `PeerId` is a cryptographic hash of peer's public key. It enables individual peers to establish a secure
communication channel as the hash can be used to verify an identity of the peer.

Hornet automatically generates a `PeerId` when it is started for the first time, and saves the identity's public key in
a file `./p2pstore/key.pub` and the private key within a BadgerDB within `./p2pstore`. The generated identity is kept
between subsequent restarts.

Each time Hornet starts, the `PeerId` is written to stdout:

```plaintext
2021-04-19T14:27:55Z  INFO    P2P     never share your ./p2pstore folder as it contains your node's private key!
2021-04-19T14:27:55Z  INFO    P2P     generating a new peer identity...
2021-04-19T14:27:55Z  INFO    P2P     stored public key under p2pstore/key.pub
2021-04-19T14:27:55Z  INFO    P2P     peer configured, ID: 12D3KooWEWunsQWGvSWYN2VR7wNNoHgae4XikBqwSre8K8sVTefu
```

Your `PeerId` is an essential part of your `multiaddr` used to configure neighbors, such
as `/dns/example.com/tcp/15600/p2p/12D3KooWHiPg9gzmy1cbTFAUekyLHQKQKvsKmhzB7NJ5xnhK4WKq`,
where `12D3KooWHiPg9gzmy1cbTFAUekyLHQKQKvsKmhzB7NJ5xnhK4WKq`
corresponds to your `PeerId`. Your `PeerId` is also visible on the start page of the dashboard.

It is recommended however to pre-generate the identity, so you can pre-communicate it to your peers before you even
start your node and also to retain the identity in case you delete your `./p2pstore` by accident.

You can use the `p2pidentity` CLI tool to generate a `PeerId` which simply generates a key pair and logs it to stdout:

```bash
./hornet tools p2pidentity
```

Sample output:

```plaintext
Your p2p private key (hex):  64166cc2627c369283cb1dd8412b6259232653611a3ae5cfb52398c23cfaead76af7a3cb775895046b9f28f2cf2b4150a9fd5dfd0ecf5c8d94529818578f40a2
Your p2p public key (hex):  6af7a3cb775895046b9f28f2cf2b4150a9fd5dfd0ecf5c8d94529818578f40a2
Your p2p public key (base58):  8CZELJwB3aBzxJgnLMvvt1FirAwNN6jif9LavYTNHCty
Your p2p PeerID:  12D3KooWH1vQ5SWtEUTVNCCCxxgGeoLZLVz1PnpCcjhcJXcGB9cu
```

Now simply copy the value of `Your p2p private key` to the `p2p.identityPrivateKey` configuration option.

Your Hornet node will now use the specified private key in `p2p.identityPrivateKey` to generate the `PeerId` (which will
ultimately be stored in `./p2pstore`).

:::info
In case there already is a `./p2pstore` with another identity, Hornet will panic and tell you that you have a previous identity which does not match with what is defined via `p2p.identityPrivateKey` (
in that case either delete the `./p2pstore` or reset the `p2p.identityPrivateKey`).
:::

More information regarding the `PeerId` is available on the [libp2p docs page](https://docs.libp2p.io/concepts/peer-id/)
.

### Addressing Peer Neighbors

In order to communicate to your peer neighbors, you also need an address to reach them. Hornet uses the `MultiAddresses`
format (also known as `multiaddr`) to achieve that.

`multiAddr` is a convention how to encode multiple layers of addressing information into a single path structure that is
future-proof. In other words, `multiaddr` is able to combine several different pieces of information in a single
human-readable and machine-optimized string, including network protocol and [`PeerId`](#node-identity).

For example, a node is reachable using IPv4 `100.1.1.1` using `TCP` on port `15600` and its `PeerId`
is `12D3KooWHjcCgWPnUEP8wNdbL2fx63Cmosk16xyZ25iUZagxmHb4`.

A `multiaddr` encoding such information would look like this:

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

In order to find out your own `multiaddr` to give to your peers for neighboring, combine the `peerId` you have gotten
from the stdout when the Hornet node started up (or which was shown via the `p2pidentity` CLI tool) and your
configured `p2p.bindAddress`. Obviously replace the `/ip4/<ip_address>`/`/dns/<hostname>` segments with the actual
information.

More information about `multiaddr` is available at the [libp2p docs page](https://docs.libp2p.io/concepts/addressing/).

### Adding Node Peers

Once you know your node's own `multiaddr`, it can be exchanged with other node owners to establish a mutual peer
connection. A recommended number of peer neighbors is 4-6 to get some degree of redundancy.

#### Finding neighbors

Join the official IOTA Discord server and join the `#nodesharing` channel and describe your node location (Europe /
Germany / Asia, etc.) with your allocated HW resources and ask for neighbors. Do not publicly disclose your
node `multiaddr`
to all readers but wait for an individual direct chat.

Peers can be added using the Hornet [dashboard](../post_installation/post_installation.md#dashboard) (simply go to `Peers` and click on `Add Peer`)
or the [peering.json](./peering.md) file.

You can change the path or name of the `peering.json` file by using the `-n` or `--peeringConfig` argument while
executing the `hornet` executable.

Example `peering.json`:

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

### Autopeering

Hornet also supports automatically finding peers through the autopeering module. We recommend to only use autopeering
if you have at least 4 static peers to minimize service distribution in case your autopeered peers are flaky.

Autopeering is enabled per default and your node will use the specified entry nodes under
`p2p.autopeering.entryNodes` to find new peers. `entryNodes` are also encoded as `multiaddr`, example:

```
/ip4/45.12.34.43/udp/14626/autopeering/8CZELJwB3aBzxJgnLMvvt1FirAwNN6jif9LavYTNHCty
```
where the `/autopeering` portion defines the base58 encoded Ed25519 public key.

Per default, Hornet will peer up to 4 autopeered peers and initiate a gossip protocol with them.
Autopeered peers are not subject to connection trimming, the same way as mutually tethered peers aren't either.

#### Entry node
If you want to run your own node as an autopeering entry node, configure `p2p.autopeering.runAsEntryNode` respectively.
The base58 encoded public key is in the output of the `p2pidentity` Hornet tool; alternatively, if you
already have an identity in a `./p2pstore`, use the `p2pidentityextract` to extract it.

### Low/High Watermark

The `p2p.connectionManager.highWatermark` and `p2p.connectionManager.lowWatermark` config options define
"watermark" points which can be thought of as a filling basin where if the `highWatermark` is reached, water is drained
until it reaches the `lowWatermark` again. Similarly, the connection manager within Hornet will start trimming away
connections to peers if `highWatermark` peers are connected until it reaches `lowWatermark` count of peers. These
watermarks exist for a certain buffer number of peers to be connected, to which not necessarily a gossip protocol is
being done.
# Post-installation

Once Hornet is deployed, all parameters are set via configuration files.

## Configuration

The most important ones are:

* `config.json`: includes all configuration flags and their values
* `peering.json`: includes all connection details to your static peers (neighbors)

> Hornet version 0.5.x targets legacy IOTA 1.0 network. Hornet version 1.x.x targets IOTA 1.5 network aka Chrysalis which is the focus of this documentation.

Depending on the installation path you selected, default configuration files may be also part of the installation
process and so you may see the following configuration files at your deployment directory:

```bash
config.json
config_chrysalis_testnet.json
peering.json
profiles.json
```

### Default configuration

By default, Hornet searches for configuration files in the working directory and expects default names, such
as `config.json` and `peering.json`.

This behavior can be changed by running Hornet with some altering arguments.

Please see the [config.json](./config.md) and [peering.json](./peering.md) chapters for more information regarding
the respective configuration files.

Once Hornet is executed, it outputs all loaded configuration parameters to `stdout` to show what configuration was
actually loaded (omitting values for things like passwords etc.).

All other altering command line parameters can be obtained by running `hornet --help` or with a more granular
output `hornet --help --full`.

## Dashboard

Per default an admin dashboard/web interface plugin is available on port 8081. It provides some useful information
regarding the node's health, peering/neighbors, overall network health and consumed system resources.

The dashboard plugin only listens on localhost:8081 per default. If you want to make it accessible from the Internet,
you will need to change the default configuration. It can be changed via the following `config.json` file section:

```json
"dashboard": {
  "bindAddress": "localhost:8081",
  "auth": {
    "sessionTimeout": "72h",
    "username": "admin",
    "passwordHash": "0000000000000000000000000000000000000000000000000000000000000000",
    "passwordSalt": "0000000000000000000000000000000000000000000000000000000000000000"
  }
}
```

Change `dashboard.bindAddress` to either `0.0.0.0:8081` to listen on all available interfaces, or the
specific interface address accordingly.

Even if accessible from the Internet, any visitor still needs a valid combination of the username and password to access
the management section of the dashboard.

The password hash and salt can be generated using the integrated `pwdhash` CLI tool:

```bash
./hornet tools pwdhash
```

Output example:

```plaintext
Enter a password:
Re-enter your password:
Success!
Your hash: 24c832e35dc542901b90888321dbfc4b1d9617332cbc124709204e6edf7e49f9
Your salt: 6c71f4753f6fb52d7a4bb5471281400c8fef760533f0589026a0e646bc03acd4
```

> `pwdhash` tool outputs the `passwordHash` and `passwordSalt` based on your input password

Copy both values to their corresponding configuration options: `dashboard.auth.passwordHash` and
`dashboard.auth.passwordSalt` respectively.

In order for the new pasword to take effect, you must restart Hornet.

## Peer neighbors

The IOTA network is a distributed network in which data is broadcasted among IOTA nodes through a gossip protocol. To be
able to participate in a network, each node has to establish a secure connection to other nodes in the network - to its
peer neighbors - and mutually exchange messages.

### Node identity

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
Your p2p private key:  7ea40ae657e2b8d46069f2ea6fe8f6ab209fb3f6f6630bc025a11a97e17e5d0675a575803660978d323fef05e871f54ecd94602b15181ba56183f9aba121ede7
Your p2p public key:  75a575803660978d323fef05e871f54ecd94602b15181ba56183f9aba121ede7
Your p2p PeerID:  12D3KooWHjcCgWPnUEP8wNdbL2fx63Cmosk16xyZ25iUZagxmHb4
```

Now simply copy the value of `Your p2p private key` to the `p2p.identityPrivateKey` configuration option.

Your Hornet node will now use the specified private key in `p2p.identityPrivateKey` to generate the `PeerId` (which will
ultimately be stored in `./p2pstore`).

> In case there already is a `./p2pstore` with another identity, Hornet will panic and tell you that you have a previous identity which does not match with what is defined via `p2p.identityPrivateKey` (
in that case either delete the `./p2pstore` or reset the `p2p.identityPrivateKey`).

More information regarding the `PeerId` is available on the [libp2p docs page](https://docs.libp2p.io/concepts/peer-id/)
.

### Addressing peer neighbors

In order to communicate to your peer neighbors, you also need an address to reach them. Hornet uses
the `MultiAddresses` format (also known as `multiaddr`) to achieve that.

`multiAddr` is a convention how to encode multiple layers of addressing information into a single path structure that is
future-proof. In other words, `multiaddr` is able to combine several different pieces of information in a single
human-readable and machine-optimized string, including network protocol and [`PeerId`](#node-identity).

For example, a node is reachable using IPv4 `100.1.1.1` using `TCP` on port `15600` and its `PeerId`
is `12D3KooWHjcCgWPnUEP8wNdbL2fx63Cmosk16xyZ25iUZagxmHb4`.

A `multiaddr` encoding such information would look like this:

```plaintext
/ip4/100.1.1.1/tcp/15600/p2p/12D3KooWHjcCgWPnUEP8wNdbL2fx63Cmosk16xyZ25iUZagxmHb4
```

> Note how `ip4` is used. A common mistake is to use `ipv4`.

If a node is reachable using a DNS name (for example `node01.iota.org`), then the given `multiaddr` would be:

```plaintext
/dns/node01.iota.org/tcp/15600/p2p/12D3KooWHjcCgWPnUEP8wNdbL2fx63Cmosk16xyZ25iUZagxmHb4
```

In order to find out your own `multiaddr` to give to your peers for neighboring, combine the `peerId` you have gotten
from the stdout when the Hornet node started up (or which was shown via the `p2pidentity` CLI tool) and your
configured `p2p.bindAddress`. Obviously replace the `/ip4/<ip_address>`/`/dns/<hostname>` segments with the actual
information.

More information about `multiaddr` is available at the [libp2p docs page](https://docs.libp2p.io/concepts/addressing/).

### Adding node peers

Once you know your node's own `multiaddr`, it can be exchanged with other node owners to establish a mutual peer
connection.

*Where to find neighbors?*

Join the official IOTA Discord server and join the `#fullnodes` channel and describe your node location (Europe /
Germany / Asia, etc.) with your allocated HW resources and ask for neighbors. Do not publicly disclose your
node `multiaddr`
to all readers but wait for an individual direct chat.

Each peer can then be added using the Hornet [dashboard](#dashboard) (admin section) or [peering.json](./peering.md)
file.

A recommended number of peer neighbors is 4-6 to get some degree of redundancy.

*Happy peering*

## Configuring HTTP REST API

One of the [tasks that the node is responsible for](../getting_started/nodes_101.md) is exposing a HTTP REST API for
clients that would like to interacts with the IOTA network, such as crypto wallets, exchanges, IoT devices, etc.

By default, the HTTP REST API is publicly exposed on port 14265 and ready to accept incoming connections from the
Internet.

Since offering the HTTP REST API to the public can consume resources of your node, there are options to restrict which
routes can be called and other request limitations.

HTTP REST API related options exists under the section `restAPI` within the `config.json` file:

```json
  "restAPI": {
    "jwtAuth": {
      "enabled": false,
      "salt": "HORNET"
    },
    "excludeHealthCheckFromAuth": false,
    "permittedRoutes": [
      "/health",
      "/mqtt",
      "/api/v1/info",
      "/api/v1/tips",
      "/api/v1/messages/:messageID",
      "/api/v1/messages/:messageID/metadata",
      "/api/v1/messages/:messageID/raw",
      "/api/v1/messages/:messageID/children",
      "/api/v1/messages",
      "/api/v1/transactions/:transactionID/included-message",
      "/api/v1/milestones/:milestoneIndex",
      "/api/v1/milestones/:milestoneIndex/utxo-changes",
      "/api/v1/outputs/:outputID",
      "/api/v1/addresses/:address",
      "/api/v1/addresses/:address/outputs",
      "/api/v1/addresses/ed25519/:address",
      "/api/v1/addresses/ed25519/:address/outputs",
      "/api/v1/treasury"
    ],
    "whitelistedAddresses": [
      "127.0.0.1",
      "::1"
    ],
    "bindAddress": "0.0.0.0:14265",
    "powEnabled": true,
    "powWorkerCount": 1,
    "limits": {
      "bodyLength": "1M",
      "maxResults": 1000
    }
  }
```

If you want to make the HTTP REST API only accessible from localhost, change the `restAPI.bindAddress` config option
accordingly.

`restAPI.permittedRoutes` defines which routes can be called from foreign addresses which are not defined under
`restAPI.whitelistedAddresses`.

If you are concerned with resource consumption, consider turning off `restAPI.powEnabled`, which makes it so that
clients must perform Proof-of-Work locally, before submitting a message for broadcast. In case you'd like to offer
Proof-of-Work for clients, consider upping `restAPI.powWorkerCount` to provide a faster message submission experience.

We suggest that you provide your HTTP REST API behind a reverse proxy, such as nginx or Traefik configured with TLS.

Please see some of our additional security recommendations [here](../getting_started/security_101.md).

Feel free to explore more details regarding different API calls
at the [IOTA client library documentation](https://chrysalis.docs.iota.org/libraries/client.html).

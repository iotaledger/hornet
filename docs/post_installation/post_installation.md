# Post-installation
Once Hornet is deployed, all parameters are set via configuration files.

## Configuration
The most important ones are:
* `config.json`: includes all configuration flags and their values
* `peering.json`: includes all connection details to your static peers (neighbors)

> Hornet version 0.5.x targets legacy IOTA 1.0 network. Hornet version 0.6.x targets IOTA 1.5 network aka Chrysalis which is the focus of this documentation

Depending on the installation path you selected, default configuration files may be also part of the installation process and so you may see the following configuration files at your deployment directory:
```bash
config.json
config_chrysalis_testnet.json
config_comnet.json
config_devnet.json
peering.json
profiles.json
```

### Default configuration
By default, Hornet searches for configuration files in the working directory and expects default names, such as `config.json` and `peering.json`.

This behavior can be changed by running Hornet with some altering arguments.

Please see the [config.json](./config.md) and [peering.json](./peering.md) chapters for more information regarding respective configuration files.

Once Hornet is executed, it outputs all loaded configuration parameters to `stdout` to show what configuration was actually loaded.

All other altering command line parameters can be obtained by running `hornet --help` or with more granular output `hornet --help --full`.


## Dashboard
There is an admin dashboard/web interface plugin available (port 8081) and it is enabled by default. It provides some useful information regarding the node, its internal activity and its consumed resources, such as allocated memory, peer activity, transactions per second rate, etc.

This plugin only listens on localhost:8081 per default. If you want to make it accessible from the Internet, you will need to change the default configuration. It can be changed via the following `config.json` file section:

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
* to enable Dashboard to be reachable from the Internet, it can be changed to `"bindAddress": "0.0.0.0:8081"`

Even if accessible from the Internet, any visitor still needs a valid combination of username and password to access a management section of the Dashboard.

It can be generated using integrated command-line Hornet tools, specifically `pwdhash`:
```bash
hornet tools pwdhash
```

Output example:
```plaintext
Enter a password:
Re-enter your password:
Success!
Your hash: 24c832e35dc542901b90888321dbfc4b1d9617332cbc124709204e6edf7e49f9
Your salt: 6c71f4753f6fb52d7a4bb5471281400c8fef760533f0589026a0e646bc03acd4
```
* `pwdhash` tool provides a combination of `passwordHash` and `passwordSalt` based on your input password
* both values should be copied to the respective configuration values above, specifically the following keys: `passwordHash` and `passwordSalt`

Once Hornet is restarted, then the Dashboard is protected by the given credentials.


## Peer neighbors
The IOTA network is a distributed network in which data is broadcasted among IOTA nodes. IOTA nodes broadcast messages to other IOTA nodes using a gossip protocol. To be able to participate in a network communication, each node has to establish a secure connection to some other nodes in the network - to its peer neighbors - and mutually exchange messages. This is the way how the data is spread within the IOTA network.

### Node identity
Each node is uniquely identified by a `peer identity`. `Peer identity` (also called `PeerId`) is represented by a public and private key pair. Then `PeerId` represents a verifiable link between the given peer and its public key, since `PeerId` is a cryptographic hash of peer's public key. It enables individual peers to establish a secure communication channel as the hash can be used to verify an identity of the peer.

Hornet, when started for the first time, generates `peerId` automatically and stores it to `./p2pstore` directory including generated private key. This `peerId` then serves as an unique `peer identity` of the given node.

This information is also reported in the log:
```plaintext
2021-04-19T14:27:55Z  INFO    P2P     never share your ./p2pstore folder as it contains your node's private key!
2021-04-19T14:27:55Z  INFO    P2P     generating a new peer identity...
2021-04-19T14:27:55Z  INFO    P2P     stored public key under p2pstore/key.pub
2021-04-19T14:27:55Z  INFO    P2P     peer configured, ID: 12D3KooWEWunsQWGvSWYN2VR7wNNoHgae4XikBqwSre8K8sVTefu
```
* `peerId` presented in the output is your node's `peerId` that is essential component when communicating with your peer neighbor(s)
* as a convention, it is usually refereed to in a form `/p2p/12D3KooWEWunsQWGvSWYN2VR7wNNoHgae4XikBqwSre8K8sVTefu`
* the given `peerId` is also visible in the Hornet [Dashboard](#dashboard)

Alternatively, you can also generate a new peer identity using hornet tools, specifically `p2pidentity`:
```bash
hornet tools p2pidentity
```

Sample output:
```plaintext
Your p2p private key:  7ea40ae657e2b8d46069f2ea6fe8f6ab209fb3f6f6630bc025a11a97e17e5d0675a575803660978d323fef05e871f54ecd94602b15181ba56183f9aba121ede7
Your p2p public key:  75a575803660978d323fef05e871f54ecd94602b15181ba56183f9aba121ede7
Your p2p PeerID:  12D3KooWHjcCgWPnUEP8wNdbL2fx63Cmosk16xyZ25iUZagxmHb4
```

The generated private key can then be injected to `identityPrivateKey` value in `config.json` file:
```json
"p2p": {
    "bindMultiAddresses": [
      "/ip4/0.0.0.0/tcp/15600"
    ],
    "connectionManager": {
      "highWatermark": 10,
      "lowWatermark": 5
    },
    "gossipUnknownPeersLimit": 4,
    "identityPrivateKey": "",
    "peerStore": {
      "path": "./p2pstore"
    },
    "reconnectInterval": "30s"
  }
```

More information regarding `peerId` is also available at [libp2p](https://docs.libp2p.io/concepts/peer-id/).

### Addressing peer neighbors
In order to communicate to your peer neighbors, you also need an address to reach them. Hornet uses `multiaddress` (also known as `multiaddr`) format to achieve that.

`Multiaddr` is a convention how to encode multiple layers of addressing information into a single path structure that is future-proof. In other words, `multiaddr` is able to combine several different pieces of information in a single human-readable and machine-optimized string, including network protocol and [peerId](#node-identity).

For example, a node is reachable using IPv4 `100.1.1.1` using `TCP` on port `15600` and its `peerId` is `12D3KooWHjcCgWPnUEP8wNdbL2fx63Cmosk16xyZ25iUZagxmHb4`.

Then the `multiaddr` is composed as:
```plaintext
/ip4/100.1.1.1/tcp/15600/p2p/12D3KooWHjcCgWPnUEP8wNdbL2fx63Cmosk16xyZ25iUZagxmHb4
```

If a node is reachable using DNS name (node01.iota.org) instead of IP address, then the given `multiaddr` would be:
```plaintext
/dns/node01.iota.org/tcp/15600/p2p/12D3KooWHjcCgWPnUEP8wNdbL2fx63Cmosk16xyZ25iUZagxmHb4
```

So node's `multiaddr` can be composed based on generated unique `peerId`, and `bindMultiAddresses` that exists in the `config.json` file under the `p2p` section :
```json
"p2p": {
    "bindMultiAddresses": [
      "/ip4/0.0.0.0/tcp/15600"
    ],
  }
```
* `/ip4/0.0.0.0/tcp/15600`: means it listens to your peer neighbors on IPv4 interface on port 15600 and accepts incoming requests from the Internet
* thanks to used `peer identity`, both parties are able to establish a secure communication channel in which both sides are cryptographically verified and so their mutual identities are confirmed

More information regarding `multiaddr` is also available at [libp2p](https://docs.libp2p.io/concepts/addressing/).

### Adding node peers
Once your node has an unique `multiaddr`, then it can be exchanged with other node owners to establish a mutual peer connection.

*Where to find your future neighbors?*

Go to the official IOTA Discord server and `#fullnodes` channel and describe your node location (Europe / Germany / Asia, etc.) with your allocated HW resources and ask for some neighbors. Do not publicly disclose your node `multiaddr` to all readers but wait for an individual direct chat.

Each peer can then be added using the Hornet [Dashboard](#dashboard) (admin section) or [peering.json](./peering.md) file.

A recommended number of peer neighbors is 4-6, since some of them can be offline from time to time.

*Happy peering*


## Configuring HTTP REST API
One of the [tasks that node is responsible for](../getting_started/nodes_101.md) is exposing HTTP REST API for clients that would like to interacts with the IOTA network, such as crypto wallets, exchanges, IoT devices, etc.

By default, HTTP REST API is publicly exposed on the port 14265 and ready to accept incoming connections from the Internet.

Since use of the given interface consumes resources of your node, there are plethora options how to control it.

REST-API-related options exists under the section `restAPI` in the `config.json` file:

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

First, there is a key named `bindAddress` that can be changed to, for example `localhost:14265`, meaning the given node accepts incoming connections from localhost only and so effectively turning the node into a private node which is completely valid use case.

Then, you can granularly control which REST API calls are accepted - key `permittedRoutes`.

REST API clients may also request your node to perform so called `proof of work` that is necessary when sending messages to the network.

It is a CPU-bound task that takes some time and may have negative impact on your available resources. So, one should consider whether to allow remote PoW to be performed on the node - key `powEnabled`. If enabled, then you can also control how many parallel workers are dedicated to PoW - key `powWorkerCount`.

Needless to say, Hornet supports standard HTTP REST API calls that can be also controlled by a reverse proxy. Please, see some of our additional security recommendations [here](../getting_started/security_101.md).

Feel free to explore more details regarding different API calls at [IOTA client library documentation](https://chrysalis.docs.iota.org/libraries/client.html).

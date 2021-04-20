# Post-installation steps
Once Hornet is deployed, all parameters are set via configuration files.

## Configuration
The most important ones are:
* `config.json`: includes all configuration flags and their values
* `peering.json`: includes all connection details to your static peers (neighbors)

Since the Hornet node software is able to power original IOTA 1.0 network as well as IOTA 1.5 (aka Chrysalis), it is important to use respective `config.json` file that targets the IOTA network that we want. All configuration files that targets respective networks are available in [source code repo](https://github.com/gohornet/hornet/tree/master) on GitHub.

Depending on the installation path you selected, default config files may be also be part of the installation process and so you may see the following configuration files at your deployment directory:
```bash
config.json
config_chrysalis_testnet.json
config_comnet.json
config_devnet.json
peering.json
profiles.json
```

### Default configuration
By default, Hornet searches for configuration files in a working directory and expects default names, such as `config.json` and `peering.json`.

This behavior can be changed by running Hornet with some altering arguments.

Please see [config.json](./config.md) and [peering.json](./peering.md) chapters for more information regarding respective configuration files.

Once Hornet is executed, it outputs all loaded configuration parameters to `stdout` to show what configuration was loaded.

All other altering command line parameters can be obtained by running `hornet --help` or more detailed `hornet --help --full`.

> Hornet version 0.5.x targets IOTA 1.0 mainnet network by default. Hornet version 0.6.x targets IOTA 1.5 (Chrysalis) mainnet network by default

### Identifying a configuration file based on particular use cases
There is a simple way how to recognize which configuration file targets which IOTA network:
```bash
cat config.json | jq "[.httpAPI?, .restAPI?]"
```
* `jq` command line json parser can be installed using `sudo apt install jq`

IOTA 1.5 (Chrysalis network) provides the following rest API endpoints:
```json
[
  null,
  {
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
]
```

IOTA 1.0 provides legacy rest API endpoints:
```json
[
  {
    "basicAuth": {
      "enabled": false,
      "username": "",
      "passwordHash": "",
      "passwordSalt": ""
    },
    "excludeHealthCheckFromAuth": false,
    "permitRemoteAccess": [
      "getNodeInfo",
      "getBalances",
      "checkConsistency",
      "getTipInfo",
      "getTransactionsToApprove",
      "getInclusionStates",
      "getNodeAPIConfiguration",
      "wereAddressesSpentFrom",
      "broadcastTransactions",
      "findTransactions",
      "storeTransactions",
      "getTrytes"
    ],
    "permittedRoutes": [
      "healthz"
    ],
    "whitelistedAddresses": [],
    "bindAddress": "0.0.0.0:14265",
    "limits": {
      "bodyLengthBytes": 1000000,
      "findTransactions": 1000,
      "getTrytes": 1000,
      "requestsList": 1000
    }
  },
  null
]
```

## Dashboard
There is an admin dashboard available in Hornet (port 8081) and it is enabled by default. It provides some useful information regarding the node, its internal activity and its resources, such as allocated memory, peer details, transactions per second rate, etc.

However, it is not listening to incoming requests from a public traffic to prevent access from a malicious actor. It is listening only to requests from `localhost` by default.

It can be configured via the following `config.json` file section:

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
* to enable Dashboard to be reachable from a public traffic, it can be changed to `"bindAddress": "0.0.0.0:8081"`

Even if enabled to public traffic, a visitor still needs a valid combination of username and password to access a management part of the dashboard.

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
* both values should be copied to the respective configuration values above

Once Hornet is restarted then the dashboard is accessible using the given credentials.

## Peer neighbors
IOTA network is a distributed network in which data is distributed among IOTA nodes. IOTA nodes broadcast messages to other IOTA nodes using a gossip protocol. To be able to participate in a network communication, each node has to establish a secure connection to some other nodes in network - to its peer neighbors - and mutually exchange messages. This is the way how the data is spread within IOTA network.

### Node identity
Each node is uniquely identified by a `peer identity`. `Peer identity` (also called `PeerId`) is represented by a public and private key pair. Then `PeerId` represents a verifiable link between a given peer and its public key, since `PeerId` is a cryptographic hash of peer's public key. It enables individual peers to establish a secure communication channel since the hash can be used to verify the identity of the peer.

Hornet, when starts for the first time, generates `peerId` automatically and stores it to `./p2pstore` directory including generated private key. This `peerId` then serves as an unique `peer identity` of the given node.

This information is also reported in log output:
```plaintext
2021-04-19T14:27:55Z  INFO    P2P     never share your ./p2pstore folder as it contains your node's private key!
2021-04-19T14:27:55Z  INFO    P2P     generating a new peer identity...
2021-04-19T14:27:55Z  INFO    P2P     stored public key under p2pstore/key.pub
2021-04-19T14:27:55Z  INFO    P2P     peer configured, ID: 12D3KooWEWunsQWGvSWYN2VR7wNNoHgae4XikBqwSre8K8sVTefu
```
* `peerId` presented in the output is your node's `peerId` that is essential component when communicating with your peer neighbor(s)
* as a convention, it is usually refereed to in a form `/p2p/12D3KooWEWunsQWGvSWYN2VR7wNNoHgae4XikBqwSre8K8sVTefu`
* the given `peerId` is also visible in Hornet [dashboard](#dashboard)

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

More information regarding `peerId` is available at [libp2p](https://docs.libp2p.io/concepts/peer-id/).

### Addressing peer neighbors
When communicating to your peer neighbors you also need an address to reach them. Hornet uses `multiaddress` (also known as `multiaddr`) format to achieve that.

`Multiaddr` is a convention how to encode multiple layers of addressing information into a single path structure that is future-proof. In other words, `multiaddr` is able to combine several different pieces of information in a single human-readable and machine-optimized string, including network protocol and `peerId`.

For example, a node is reachable using IPv4 `100.1.1.1` using `TCP` on port `15600` and its `peerId` is `12D3KooWHjcCgWPnUEP8wNdbL2fx63Cmosk16xyZ25iUZagxmHb4`, then `multiaddr` is composed as:
```plaintext
/ip4/100.1.1.1/tcp/15600/p2p/12D3KooWHjcCgWPnUEP8wNdbL2fx63Cmosk16xyZ25iUZagxmHb4
```

If a node is reachable using DNS name (node01.iota.org) instead of IP addr, then `multiaddr` would be:
```plaintext
/dns/node01.iota.org/tcp/15600/p2p/12D3KooWHjcCgWPnUEP8wNdbL2fx63Cmosk16xyZ25iUZagxmHb4
```

So node's `multiaddr` can be composed based on generated unique `peerId` and `bindMultiAddresses` based on `config.json`:
```json
"p2p": {
    "bindMultiAddresses": [
      "/ip4/0.0.0.0/tcp/15600"
    ],
  }
```
* `/ip4/0.0.0.0/tcp/15600`: it means it listens to your peer neighbors on IPv4 interface on port 15600 and accepts incoming public traffic
* thanks to used `peer identity`, both parties are able to establish a secure communication channel in which both sides are cryptographically verified and their identity is ensured

More information regarding `multiaddr` is available at [libp2p](https://docs.libp2p.io/concepts/addressing/).

### Adding node peers
Once your node has an unique `multiaddr`, then it can be exchanged with other node owners to establish a mutual peer connection. It has to be done manually until `autopeering` is enabled.

*Where to find your future neighbors?*

Goto official IOTA Discord space, specifically `#fullnodes` channel, describe your node location (Europe / Germany / Asia, etc.) with HW parameters and ask for some neighbors. Do not publicly disclose your node `multiaddr` to all readers but wait for some individual peer to peer chat.

Each peer can then be added using Hornet [dashboard](#dashboard) (admin section) or [peering.json](./peering.md).

A recommended number of peer neighbors is 4-6, since some of them can be offline from time to time.

*Happy peering*

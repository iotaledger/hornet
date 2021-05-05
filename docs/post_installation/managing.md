# Managing node
In this chapter, there is an overview of key concepts that are important to consider during a maintenance cycle of your node.

## Storage
Hornet uses embedded database engine that stores its data in a directory on file system. The location is controlled via `config.json` file under the section `db`, key `path`:

```json
"db": {
    "engine": "rocksdb",
    "path": "mainnetdb",
    "autoRevalidation": false
  }
```
There is a convention that the directory is named after the network type: `mainnet` vs `testnet`.

Another important directory is a directory dedicated to snapshots controlled via section `snapshots` of the `config.json`, specifically `fullPath` and `deltaPath` keys:

```json
"snapshots": {
    "interval": 50,
    "fullPath": "snapshots/mainnet/full_snapshot.bin",
    "deltaPath": "snapshots/mainnet/delta_snapshot.bin",
    "deltaSizeThresholdPercentage": 50.0,
    "downloadURLs": [
      {
        "full": "https://ls.manapotion.io/full_snapshot.bin",
        "delta": "https://ls.manapotion.io/delta_snapshot.bin"
      },
      {
        "full": "https://x-vps.com/full_snapshot.bin",
        "delta": "https://x-vps.com/delta_snapshot.bin"
      },
      {
        "full": "https://dbfiles.iota.org/mainnet/hornet/full_snapshot.bin",
        "delta": "https://dbfiles.iota.org/mainnet/hornet/delta_snapshot.bin"
      }
    ]
```
The same convention is applied and directories are named after the network type (`mainnet` vs `testnet`).

Here is the full overview of all files and directories that are leveraged by the Hornet:
```plaintext
.
├── config.json
├── hornet              <EXECUTABLE>
├── p2pstore
│   ├── [...files]
├── snapshots           <SNAPSHOT DIR>
│   └── testnet
│       ├── delta_snapshot.bin
│       └── full_snapshot.bin
└── testnetdb           <DB DIR>
    ├── [...db files]
```

## Plugins
Hornet can be extended by plugins. Plugins are controlled via section `node` of the `config.json` file, specifically `disablePlugins` and `enablePlugins` keys:
```json
"node": {
    "alias": "Mainnet",
    "profile": "auto",
    "disablePlugins": [],
    "enablePlugins": []
  },
```

Additionally, plugins can be controlled via [Dashboard/web interface](./post_installation.md).


## Spammer
Hornet integrates a lightweight spamming plugin that spams the network with messages. Since the IOTA network is based on Directed Acyclic Graph in which new incoming messages are connected to previous messages (tips), it is healthy for the network to maintain some level of message rate.

The Spammer plugin allows your node to send a number of data messages at regular interval. The interval is set in the `mpsRateLimit` key, which is the number of messages per second (TPS) that the plugin should try to send.

For example, value `"mpsRateLimit": 0.1` would mean to send 1 message every 10 seconds.

Needless to say, it is turned off by default:

```json
 "spammer": {
    "message": "Binary is the future.",
    "index": "HORNET Spammer",
    "indexSemiLazy": "HORNET Spammer Semi-Lazy",
    "cpuMaxUsage": 0.8,
    "mpsRateLimit": 0.0,
    "workers": 0,
    "autostart": false
  }
```

*This plugin can be also leveraged during a spamming event during which the community tests the throughput of the network.*

## Snapshots
Your node's ledger accumulates many messages, which uses a significant disk capacity over time. This topic discusses how to configure local snapshots to prune old transactions from your node's database and to create backup snapshot files.

```json
 "snapshots": {
    "interval": 50,
    "fullPath": "snapshots/mainnet/full_snapshot.bin",
    "deltaPath": "snapshots/mainnet/delta_snapshot.bin",
    "deltaSizeThresholdPercentage": 50.0,
    "downloadURLs": [
      {
        "full": "https://ls.manapotion.io/full_snapshot.bin",
        "delta": "https://ls.manapotion.io/delta_snapshot.bin"
      },
      {
        "full": "https://x-vps.com/full_snapshot.bin",
        "delta": "https://x-vps.com/delta_snapshot.bin"
      },
      {
        "full": "https://dbfiles.iota.org/mainnet/hornet/full_snapshot.bin",
        "delta": "https://dbfiles.iota.org/mainnet/hornet/delta_snapshot.bin"
      }
    ]
  },
  "pruning": {
    "enabled": true,
    "delay": 60480,
    "pruneReceipts": false
  }
```

### Snapshot pruning
During a snapshot, messages may be deleted from the ledger if they were confirmed by an old milestone. In other words, the term `pruning` means the deletion of the old history from the node database:
* To enable pruning, set the `pruning.enabled` key to enabled
* The `pruning.delay` defines how far back from the current confirmed milestone should be pruned

There are two types of snapshots:

**Delta snapshot**
A delta snapshot points to a specific full snapshot, ie. delta snapshot consists of the changes since the last full snapshot.

**Full snapshot**
The full snapshot includes the whole ledger state to a specific milestone and a solid entry point.


### How to work with snapshots
If your run Hornet node for the first time, you need to start it with a full-snapshot. *The Hornet downloads it for you automatically from trusted sources.*

Additionally, you can start it with an specific delta snapshot.

You can use the Hornet tools for that:
```bash
hornet tool
```
* snapgen: generates an initial snapshot for a private network
* snapmerge: merges a full and delta snapshot into an updated full snapshot
* snapinfo: outputs information about a snapshot file

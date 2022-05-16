---
description: Learn about key concepts to consider for Node maintenance. These include configuring storage, plugins, spammers, and how to work with snapshots.
image: /img/logo/HornetLogo.png
keywords:
- IOTA Node 
- Hornet Node
- storage
- configuration
- spammer
- snapshots
- reference
---


# Managing a Node
In this section, you can find an overview of the key concepts that you should consider during your node's maintenance cycle.

## Storage
Hornet uses an embedded database engine that stores its data in a directory in a file system. You can manage the location using the `config.json` file, under the `db` section, with the `path` key:

```json{3}
"db": {
    "engine": "rocksdb",
    "path": "mainnetdb",
    "autoRevalidation": false
  }
```

By convention, you should name that directory after the network type: `mainnet` or `testnet`.

Another important directory is the `snapshots` directory. You can control the `snapshots` in the `snapshots` section of the `config.json` file, specifically the `fullPath` and `deltaPath` keys:

```json
"snapshots": {
    "interval": 50,
    "fullPath": "snapshots/mainnet/full_snapshot.bin",
    "deltaPath": "snapshots/mainnet/delta_snapshot.bin",
    "deltaSizeThresholdPercentage": 50.0,
    "downloadURLs": [
      {
        "full": "https://chrysalis-dbfiles.iota.org/snapshots/hornet/latest-full_snapshot.bin",
        "delta": "https://chrysalis-dbfiles.iota.org/snapshots/hornet/latest-delta_snapshot.bin"
      },
      {
        "full": "https://cdn.tanglebay.com/snapshots/mainnet/full_snapshot.bin",
        "delta": "https://cdn.tanglebay.com/snapshots/mainnet/delta_snapshot.bin"
      }
    ]
```
You should apply the same convention you use for the database engine to name the directories after the network type: `mainnet` or `testnet`.

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
Hornet can be extended by plugins. You can control plugins using the `node` section in the `config.json` file, specifically `disablePlugins` and `enablePlugins` keys:

```json
"app": {
    "alias": "Mainnet",
    "profile": "auto",
    "disablePlugins": [],
    "enablePlugins": []
  },
```

You can also control plugins using the [Dashboard/web interface](https://wiki.iota.org/hornet/post_installation#dashboard).


## Spammer
Hornet integrates a lightweight spamming plugin that spams the network with messages. The IOTA network is based on a Directed Acyclic Graph. So, new incoming messages are connected to previous messages (tips). It is healthy for the network to maintain some level of message rate.

The Spammer plugin allows your node to send several data messages at regular intervals. You can set the interval with the `mpsRateLimit` key, which is the number of messages per second (TPS) that the plugin should try to send.

For example, value `"mpsRateLimit": 0.1` would mean to send 1 message every 10 seconds.

You can change the default configuration by enabling this plugin since it is usually disabled.

```json
 "spammer": {
    "message": "We are all made of stardust.",
    "index": "HORNET Spammer",
    "indexSemiLazy": "HORNET Spammer Semi-Lazy",
    "cpuMaxUsage": 0.8,
    "mpsRateLimit": 0.0,
    "workers": 0,
    "autostart": false
  }
```

:::note

This plugin can also be leveraged during a spamming event during which the community tests the throughput of the network.

:::

## Snapshots
Your node's ledger accumulates many messages, which uses a significant disk capacity over time. This section discusses configuring local snapshots to prune old transactions from your node's database and create backup snapshot files.

```json
 "snapshots": {
    "interval": 50,
    "fullPath": "snapshots/mainnet/full_snapshot.bin",
    "deltaPath": "snapshots/mainnet/delta_snapshot.bin",
    "deltaSizeThresholdPercentage": 50.0,
    "downloadURLs": [
      {
        "full": "https://chrysalis-dbfiles.iota.org/snapshots/hornet/latest-full_snapshot.bin",
        "delta": "https://chrysalis-dbfiles.iota.org/snapshots/hornet/latest-delta_snapshot.bin"
      },
      {
        "full": "https://cdn.tanglebay.com/snapshots/mainnet/full_snapshot.bin",
        "delta": "https://cdn.tanglebay.com/snapshots/mainnet/delta_snapshot.bin"
      }
    ]
  },
  "pruning": {
    "milestones": {
      "enabled": false,
      "maxMilestonesToKeep": 60480
    },
    "size": {
      "enabled": true,
      "targetSize": "30GB",
      "thresholdPercentage": 10.0,
      "cooldownTime": "5m"
    },
    "pruneReceipts": false
  },
```

### Snapshot Pruning
During a snapshot, Hornet may delete messages from the ledger if they were confirmed by an old milestone. In other words, the term _pruning_ means the deletion of the old history from the node database.

* If you want to enable pruning, you should set the `pruning.milestones.enabled` or `pruning.size.enabled` keys to _enabled_.
* The `pruning.milestones.maxMilestonesToKeep` defines how far back from the current confirmed milestone should be pruned.
* The `pruning.size.targetSize` defines the maximum database size. Old data will be pruned.

There are two types of snapshots:

#### Delta snapshots
A delta snapshot points to a specific full snapshot, meaning a delta snapshot consists of the changes since the last full snapshot.

#### Full snapshots
The full snapshot includes the whole ledger state to a specific milestone and a solid entry point.


### How to Work With Snapshots
If you are running a Hornet node for the first time, you will need to start it with a full snapshot. Hornet automatically downloads this from trusted sources.

Additionally, you can start Hornet with a specific delta snapshot using the `Hornet` tools:

```bash
hornet tool
```
- `snap-gen` Generates an initial snapshot for a private network.
- `snap-merge` Merges a full and delta snapshot into an updated full snapshot.
- `snap-info` Outputs information about a snapshot file.

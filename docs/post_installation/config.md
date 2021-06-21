# Core Configuration
Hornet uses a JSON standard format as a config file. If you are unsure about JSON syntax, you can find more information in the [official JSON specs](https://www.json.org).

The default config file is `config.json`. You can change the path or name of the config file by using the `-c` or `--config` argument while executing `hornet` executable.

For example:
```bash
hornet -c config_example.json
```

You can always get the most up-to-date description of the config parameters by running:

```bash
hornet -h --full
```

[comment]: <> (## Table of Contents)

[comment]: <> (- [Core Configuration]&#40;#core-configuration&#41;)

[comment]: <> (  - [Table of Contents]&#40;#table-of-contents&#41;)

[comment]: <> (  - [1. REST API]&#40;#1-rest-api&#41;)

[comment]: <> (    - [JWT Auth]&#40;#jwt-auth&#41;)

[comment]: <> (    - [Limits]&#40;#limits&#41;)

[comment]: <> (  - [2. Dashboard]&#40;#2-dashboard&#41;)

[comment]: <> (    - [Auth]&#40;#auth&#41;)

[comment]: <> (  - [3. DB]&#40;#3-db&#41;)

[comment]: <> (  - [4. Snapshots]&#40;#4-snapshots&#41;)

[comment]: <> (    - [DownloadURLs]&#40;#downloadurls&#41;)

[comment]: <> (  - [5. Pruning]&#40;#5-pruning&#41;)

[comment]: <> (    - [Milestones]&#40;#milestones&#41;)

[comment]: <> (    - [Size]&#40;#size&#41;)

[comment]: <> (  - [6. Protocol]&#40;#6-protocol&#41;)

[comment]: <> (    - [PublicKeyRanges]&#40;#publickeyranges&#41;)

[comment]: <> (  - [7. Proof of Work]&#40;#7-proof-of-work&#41;)

[comment]: <> (  - [8. Requests]&#40;#8-requests&#41;)

[comment]: <> (  - [9. Coordinator]&#40;#9-coordinator&#41;)

[comment]: <> (    - [Checkpoints]&#40;#checkpoints&#41;)

[comment]: <> (    - [Quorum]&#40;#quorum&#41;)

[comment]: <> (      - [Groups]&#40;#groups&#41;)

[comment]: <> (        - [{GROUP_NAME}]&#40;#group_name&#41;)

[comment]: <> (    - [Signing]&#40;#signing&#41;)

[comment]: <> (    - [Tipsel]&#40;#tipsel&#41;)

[comment]: <> (  - [10. Tangle]&#40;#10-tangle&#41;)

[comment]: <> (  - [11. Tipsel]&#40;#11-tipsel&#41;)

[comment]: <> (    - [NonLazy]&#40;#nonlazy&#41;)

[comment]: <> (    - [SemiLazy]&#40;#semilazy&#41;)

[comment]: <> (  - [12. Node]&#40;#12-node&#41;)

[comment]: <> (  - [13. P2P]&#40;#13-p2p&#41;)

[comment]: <> (    - [ConnectionManager]&#40;#connectionmanager&#41;)

[comment]: <> (    - [PeerStore]&#40;#peerstore&#41;)

[comment]: <> (  - [14. Logger]&#40;#14-logger&#41;)

[comment]: <> (  - [15. Warpsync]&#40;#15-warpsync&#41;)

[comment]: <> (  - [16. Spammer]&#40;#16-spammer&#41;)

[comment]: <> (  - [17. MQTT]&#40;#17-mqtt&#41;)

[comment]: <> (  - [18. Profiling]&#40;#18-profiling&#41;)

[comment]: <> (  - [19. Prometheus]&#40;#19-prometheus&#41;)

[comment]: <> (    - [FileServiceDiscovery]&#40;#fileservicediscovery&#41;)

[comment]: <> (  - [20. Gossip]&#40;#20-gossip&#41;)

[comment]: <> (  - [21. Debug]&#40;#21-debug&#41;)

[comment]: <> (  - [22. Legacy]&#40;#22-legacy&#41;)

[comment]: <> (  - [22.1 Migrator]&#40;#221-migrator&#41;)

[comment]: <> (  - [22.2 Receipts]&#40;#222-receipts&#41;)

[comment]: <> (    - [Backup]&#40;#backup&#41;)

[comment]: <> (    - [Validator]&#40;#validator&#41;)

[comment]: <> (      - [Api]&#40;#api&#41;)

[comment]: <> (      - [Coordinator]&#40;#coordinator&#41;)

[comment]: <> (* * *)

## 1. REST API

| Name                       | Description                                                                     | Type             |
| :------------------------- | :------------------------------------------------------------------------------ | :--------------- |
| [jwtAuth](#jwt-auth)       | Config for JWT auth                                                             | object           |
| permittedRoutes            | The allowed HTTP REST routes which can be called from non whitelisted addresses | array of strings |
| whitelistedAddresses       | The whitelist of addresses which are allowed to access the REST API             | array of strings |
| bindAddress                | The bind address on which the REST API listens on                               | string           |
| powEnabled                 | Whether the node does PoW if messages are received via API                      | bool             |
| powWorkerCount             | The amount of workers used for calculating PoW when issuing messages via API    | integer          |
| [limits](#limits)          | Configuration for api limits                                                    | object           |
| excludeHealthCheckFromAuth | Whether to allow the health check route anyways                                 | bool             |

### JWT Auth

| Name    | Description                                                                                                                             | Type   |
| :------ | :-------------------------------------------------------------------------------------------------------------------------------------- | :----- |
| enabled | Whether to use JWT auth for the REST API                                                                                                | bool   |
| salt    | Salt used inside the JWT tokens for the REST API. Change this to a different value to invalidate JWT tokens not matching this new value | string |


### Limits

| Name       | Description                                                               | Type    |
| :--------- | :------------------------------------------------------------------------ | :------ |
| bodyLength | The maximum number of characters that the body of an API call may contain | string  |
| maxResults | The maximum number of results that may be returned by an endpoint         | integer |

Example:

```json
  "restAPI": {
    "authEnabled": false,
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
    "powEnabled": false,
    "powWorkerCount": 1,
    "limits": {
      "bodyLength": "1M",
      "maxResults": 1000
    }
  },
```

## 2. Dashboard

| Name          | Description                                                | Type   |
| :------------ | :--------------------------------------------------------- | :----- |
| bindAddress   | The bind address on which the dashboard can be access from | string |
| dev           | Whether to run the dashboard in dev mode                   | bool   |
| [auth](#auth) | Configuration for dashboard auth                           | object |

### Auth

| Name           | Description                                           | Type   |
| :------------- | :---------------------------------------------------- | :----- |
| sessionTimeout | How long the auth session should last before expiring | string |
| username       | The auth username                                     | string |
| passwordHash   | The auth password+salt as a scrypt hash               | string |
| passwordSalt   | The auth salt used for hashing the password           | string |

Example:

```json
  "dashboard": {
    "bindAddress": "localhost:8081",
    "dev": false,
    "auth": {
      "sessionTimeout": "72h",
      "username": "admin",
      "passwordHash": "0000000000000000000000000000000000000000000000000000000000000000",
      "passwordSalt": "0000000000000000000000000000000000000000000000000000000000000000"
    }
  },
```

## 3. DB

| Name             | Description                                                                         | Type   |
| :--------------- | :---------------------------------------------------------------------------------- | :----- |
| engine           | The used database engine (pebble/bolt/rocksdb)                                      | string |
| path             | The path to the database folder                                                     | string |
| autoRevalidation | Whether to automatically start revalidation on startup if the database is corrupted | bool   |
| debug            | Ignore the check for corrupted databases (should only be used for debug reasons)    | bool   |

Example:

```json
  "db": {
    "engine": "rocksdb",
    "path": "mainnetdb",
    "autoRevalidation": false,
    "debug": false,
  },
```

## 4. Snapshots

| Name                          | Description                                                                                                                                                            | Type             |
| :---------------------------- | :--------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :--------------- |
| interval                      | Interval, in milestones, at which snapshot files are created (snapshots are only created if the node is synced)                                                        | integer          |
| depth                         | The depth, respectively the starting point, at which a snapshot of the ledger is generated                                                                             | integer          |
| fullPath                      | Path to the full snapshot file                                                                                                                                         | string           |
| deltaPath                     | Path to the delta snapshot file                                                                                                                                        | string           |
| deltaSizeThresholdPercentage  | Create a full snapshot if the size of a delta snapshot reaches a certain percentage of the full snapshot  (0.0 = always create delta snapshot to keep ms diff history) | float            |
| [downloadURLs](#downloadurls) | URLs to load the snapshot files from.                                                                                                                                  | array of objects |

### DownloadURLs

| Name  | Description                              | Type   |
| :---- | :--------------------------------------- | :----- |
| full  | Download link to the full snapshot file  | string |
| delta | Download link to the delta snapshot file | string |

Example:

```json
"snapshots": {
    "interval": 50,
    "depth": 50,
    "fullPath": "snapshots/mainnet/full_snapshot.bin",
    "deltaPath": "snapshots/mainnet/delta_snapshot.bin",
    "deltaSizeThresholdPercentage": 50.0,
    "downloadURLs": [
      {
        "full": "https://source1.example.com/full_snapshot.bin",
        "delta": "https://source1.example.com/delta_snapshot.bin"
      },
      {
        "full": "https://source2.example.com/full_snapshot.bin",
        "delta": "https://source2.example.com/delta_snapshot.bin"
      }
    ]
  },
```

## 5. Pruning

| Name                      | Description                                           | Type   |
| :------------------------ | :---------------------------------------------------- | :----- |
| [milestones](#Milestones) | Milestones based pruning                              | object |
| [size](#Size)             | Database size based pruning                           | object |
| pruneReceipts             | Whether to delete old receipts data from the database | bool   |

### Milestones

| Name                | Description                                                                              | Type    |
| :------------------ | :--------------------------------------------------------------------------------------- | :------ |
| enabled             | Whether to delete old message data from the database based on maximum milestones to keep | bool    |
| maxMilestonesToKeep | Maximum amount of milestone cones to keep in the database                                | integer |

### Size
| Name                | Description                                                                         | Type   |
| :------------------ | :---------------------------------------------------------------------------------- | :----- |
| enabled             | Whether to delete old message data from the database based on maximum database size | bool   |
| targetSize          | Target size of the database                                                         | string |
| thresholdPercentage | The percentage the database size gets reduced if the target size is reached         | float  |
| cooldownTime        | Cool down time between two pruning by database size events                           | string |

Example:

```json
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

## 6. Protocol

| Name                                | Description                                       | Type             |
| :---------------------------------- | :------------------------------------------------ | :--------------- |
| networkID                           | The network ID on which this node operates on     | string           |
| bech32HRP                           | The HRP which should be used for Bech32 addresses | string           |
| minPoWScore                         | The minimum PoW score required by the network     | float            |
| milestonePublicKeyCount             | The amount of public keys in a milestone          | integer          |
| [publicKeyRanges](#publickeyranges) | List of public key ranges from the coordinator    | array of objects |

### PublicKeyRanges

| Name  | Description           | Type    |
| :---- | :-------------------- | :------ |
| key   | Public key            | string  |
| start | Milestone start index | integer |
| end   | Milestone end index   | integer |

Example:

```json
  "protocol": {
    "networkID": "mainnet1",
    "bech32HRP": "iota",
    "minPoWScore": 4000,
    "milestonePublicKeyCount": 2,
    "publicKeyRanges": [
      {
        "key": "7205c145525cee64f1c9363696811d239919d830ad964b4e29359e6475848f5a",
        "start": 0,
        "end": 0
      },
      {
        "key": "e468e82df33d10dea3bd0eadcd7867946a674d207c39f5af4cc44365d268a7e6",
        "start": 0,
        "end": 0
      },
      {
        "key": "0758028d34508079ba1f223907ac3bb5ce8f6bdccc6b961c7c85a2f460b30c1d",
        "start": 0,
        "end": 0
      }
    ]
  },
```

## 7. Proof of Work

| Name                | Description                                                                                              | Type   |
| :------------------ | :------------------------------------------------------------------------------------------------------- | :----- |
| refreshTipsInterval | Interval for refreshing tips during PoW for spammer messages and messages passed without parents via API | string |

Example:

```json
  "pow": {
    "refreshTipsInterval": "5s"
  },
```

## 8. Requests

| Name                     | Description                                           | Type   |
| :----------------------- | :---------------------------------------------------- | :----- |
| discardOlderThan         | The maximum time a request stays in the request queue | string |
| pendingReEnqueueInterval | The interval the pending requests are re-enqueued     | string |

Example:

```json
  "requests": {
    "discardOlderThan": "15s",
    "pendingReEnqueueInterval": "5s"
  },
```

## 9. Coordinator

| Name                        | Description                                                                            | Type    |
| :-------------------------- | :------------------------------------------------------------------------------------- | :------ |
| [checkpoints](#checkpoints) | Configuration for checkpoints                                                          | object  |
| interval                    | The interval milestones are issued                                                     | string  |
| powWorkerCount              | The amount of workers used for calculating PoW when issuing checkpoints and milestones | integer |
| [quorum](#quorum)           | Configuration for quorum                                                                      | object  |
| [signing](#signing)         | Configuration for signing                                                                     | object  |
| stateFilePath               | The path to the state file of the coordinator                                          | string  |
| [tipsel](#tipsel)           | Configuration for tip selection                                                               | object  |

### Checkpoints

| Name               | Description                                                 | Type    |
| :----------------- | :---------------------------------------------------------- | :------ |
| maxTrackedMessages | Maximum amount of known messages for milestone tip selection | integer |

### Quorum

| Name              | Description                                                                           | Type                   |
| :---------------- | :------------------------------------------------------------------------------------ | :--------------------- |
| enabled           | Whether the coordinator quorum is enabled                                             | bool                   |
| [groups](#groups) | The quorum groups used to ask other nodes for correct ledger state of the coordinator | array of object arrays |
| timeout           | The timeout until a node in the quorum must have answered                             | string                 |

#### Groups

| Name                        | Description                                                                          | Type             |
| :-------------------------- | :----------------------------------------------------------------------------------- | :--------------- |
| [{GROUP_NAME}](#group_name) | The qourum group used to ask other nodes for correct ledger state of the coordinator | array of objects |

##### {GROUP_NAME}

| Name     | Description                           | Type   |
| :------- | :------------------------------------ | :----- |
| alias    | Alias of the quorum client (optional) | string |
| baseURL  | BaseURL of the quorum client          | string |
| userName | Username for basic auth (optional)    | string |
| password | Password for basic auth (optional)    | string |

### Signing

| Name          | Description                                                                  | Type   |
| :------------ | :--------------------------------------------------------------------------- | :----- |
| provider      | The signing provider the coordinator uses to sign a milestone (local/remote) | string |
| remoteAddress | The address of the remote signing provider (insecure connection!)            | string |

### Tipsel

| Name                                           | Description                                                       | Type    |
| :--------------------------------------------- | :---------------------------------------------------------------- | :------ |
| heaviestBranchSelectionTimeout                 | The maximum duration to select the heaviest branch tips           | string  |
| maxHeaviestBranchTipsPerCheckpoint             | Maximum amount of checkpoint messages with heaviest branch tips   | integer |
| minHeaviestBranchUnreferencedMessagesThreshold | Minimum threshold of unreferenced messages in the heaviest branch | integer |
| randomTipsPerCheckpoint                        | Amount of checkpoint messages with random tips                    | integer |

Example:

```json
  "coordinator": {
    "stateFilePath": "coordinator.state",
    "interval": "10s",
    "powWorkerCount": 15,
    "checkpoints": {
      "maxTrackedMessages": 10000
    },
    "tipsel": {
      "minHeaviestBranchUnreferencedMessagesThreshold": 20,
      "maxHeaviestBranchTipsPerCheckpoint": 10,
      "randomTipsPerCheckpoint": 3,
      "heaviestBranchSelectionTimeout": "100ms"
    },
    "signing": {
      "provider": "local",
      "remoteAddress": "localhost:12345"
    },
    "quorum": {
      "enabled": false,
      "groups": {
        "hornet": [
          {
            "alias": "hornet1",
            "baseURL": "http://hornet1.example.com:14265",
            "userName": "",
            "password": ""
          }
        ],
        "bee": [
          {
            "alias": "bee1",
            "baseURL": "http://bee1.example.com:14265",
            "userName": "",
            "password": ""
          }
        ]
      },
      "timeout": "2s"
    }
  },
```

## 10. Tangle

| Name             | Description                                                                       | Type   |
| :--------------- | :-------------------------------------------------------------------------------- | :----- |
| milestoneTimeout | The interval milestone timeout events are fired if no new milestones are received | string |

Example:

```json
  "tangle": {
    "milestoneTimeout": "30s"
  },
```

## 11. Tipsel

| Name                                  | Description                                                                                                             | Type    |
| :------------------------------------ | :---------------------------------------------------------------------------------------------------------------------- | :------ |
| maxDeltaMsgYoungestConeRootIndexToCMI | The maximum allowed delta value for the YCRI of a given message in relation to the current CMI before it gets lazy      | integer |
| maxDeltaMsgOldestConeRootIndexToCMI   | The maximum allowed delta value between OCRI of a given message in relation to the current CMI before it gets semi-lazy | integer |
| belowMaxDepth                         | The maximum allowed delta value for the OCRI of a given message in relation to the current CMI before it gets lazy      | integer |
| [nonLazy](#nonlazy)                   | Configuration for tips from the non-lazy pool                                                                                  | object  |
| [semiLazy](#semilazy)                 | Configuration for tips from the semi-lazy pool                                                                                 | object  |

### NonLazy

| Name                    | Description                                                                                               | Type    |
| :---------------------- | :-------------------------------------------------------------------------------------------------------- | :------ |
| retentionRulesTipsLimit | The maximum number of current tips for which the retention rules are checked (non-lazy)                   | integer |
| maxReferencedTipAge     | The maximum time a tip remains in the tip pool after it was referenced by the first message (non-lazy)    | string  |
| maxChildren             | The maximum amount of references by other messages before the tip is removed from the tip pool (non-lazy) | integer |
| spammerTipsThreshold    | The maximum amount of tips in a tip-pool (non-lazy) before the spammer tries to reduce these              | integer |

### SemiLazy

| Name                    | Description                                                                                                | Type    |
| :---------------------- | :--------------------------------------------------------------------------------------------------------- | :------ |
| retentionRulesTipsLimit | The maximum number of current tips for which the retention rules are checked (semi-lazy)                   | integer |
| maxReferencedTipAge     | The maximum time a tip remains in the tip pool after it was referenced by the first message (semi-lazy)    | string  |
| maxChildren             | The maximum amount of references by other messages before the tip is removed from the tip pool (semi-lazy) | integer |
| spammerTipsThreshold    | The maximum amount of tips in a tip-pool (semi-lazy) before the spammer tries to reduce these              | integer |

Example:

```json
  "tipsel": {
    "maxDeltaMsgYoungestConeRootIndexToCMI": 8,
    "maxDeltaMsgOldestConeRootIndexToCMI": 13,
    "belowMaxDepth": 15,
    "nonLazy": {
      "retentionRulesTipsLimit": 100,
      "maxReferencedTipAge": "3s",
      "maxChildren": 30,
      "spammerTipsThreshold": 0
    },
    "semiLazy": {
      "retentionRulesTipsLimit": 20,
      "maxReferencedTipAge": "3s",
      "maxChildren": 2,
      "spammerTipsThreshold": 30
    }
  },
```

## 12. Node

| Name           | Description                              | Type             |
| :------------- | :--------------------------------------- | :--------------- |
| alias          | The alias to identify a node             | string           |
| profile        | The profile the node runs with           | string           |
| disablePlugins | A list of plugins that shall be disabled | array of strings |
| enablePlugins  | A list of plugins that shall be enabled  | array of strings |

Example:

```json
  "node": {
    "alias": "Mainnet",
    "profile": "auto",
    "disablePlugins": [
      "Warpsync"
    ],
    "enablePlugins": [
      "Prometheus",
      "Spammer"
    ]
  },
```

## 13. P2P

| Name                                    | Description                                                                    | Type             |
| :-------------------------------------- | :----------------------------------------------------------------------------- | :--------------- |
| bindMultiAddresses                      | The bind addresses for this node                                               | array of strings |
| [connectionManager](#connectionmanager) | Configuration for connection manager                                                  | object           |
| gossipUnknownPeersLimit                 | maximum amount of unknown peers a gossip protocol connection is established to | integer          |
| identityPrivateKey                      | private key used to derive the node identity (optional)                        | string           |
| [peerStore](#peerstore)                 | Configuration for peer store                                                          | object           |
| reconnectInterval                       | The time to wait before trying to reconnect to a disconnected peer             | string           |

### ConnectionManager

| Name          | Description                                                                  | Type    |
| :------------ | :--------------------------------------------------------------------------- | :------ |
| highWatermark | The threshold up on which connections count truncates to the lower watermark | integer |
| lowWatermark  | The minimum connections count to hold after the high watermark was reached   | integer |

### PeerStore

| Name | Description                | Type   |
| :--- | :------------------------- | :----- |
| path | The path to the peer store | string |

Example:

```json
  "p2p": {
    "bindMultiAddresses": [
      "/ip4/127.0.0.1/tcp/15600"
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
  },
```

[//]: # "Not implemented yet. Don't forget to add entry number 8 in TOC if this gets implemented"
[//]: # "## 12. P2Pdisc"

[//]: # "| Name                      | Description                                                                              | Type    |"
[//]: # "| :------------------------ | :--------------------------------------------------------------------------------------- | :------ |"
[//]: # "| advertiseInterval         | The interval at which the node advertises itself on the DHT for peer discovery           | string  |"
[//]: # "| maxDiscoveredPeerConns    | The max. amount of peers to be connected to which were discovered via the DHT rendezvous | integer |"
[//]: # "| rendezvousPoint           | The rendezvous string for advertising on the DHT that the node wants to peer with others | string  |"
[//]: # "| routingTableRefreshPeriod | The routing table refresh period                                                         | string  |"

[//]: # "Example:"

[//]: # "```json"
[//]: # '  "p2pdisc": {'
[//]: # '    "advertiseInterval": "30s",'
[//]: # '    "maxDiscoveredPeerConns": 4,'
[//]: # '    "rendezvousPoint": "between-two-vertices",'
[//]: # '    "routingTableRefreshPeriod": "1m",'
[//]: # "  },"
[//]: # "```"

## 14. Logger

| Name          | Description                                                                                                       | Type             |
| :------------ | :---------------------------------------------------------------------------------------------------------------- | :--------------- |
| level         | The minimum enabled logging level. Valid values are: "debug", "info", "warn", "error", "dpanic", "panic", "fatal" | string           |
| disableCaller | Stops annotating logs with the calling function's file name and line number                                       | bool             |
| encoding      | Sets the logger's encoding. Valid values are "json" and "console"                                                 | string           |
| outputPaths   | A list of URLs, file paths or stdout/stderr to write logging output to                                            | array of strings |

Example:

```json
  "logger": {
    "level": "info",
    "disableCaller": true,
    "encoding": "console",
    "outputPaths": [
      "stdout",
      "hornet.log"
    ]
  },
```

## 15. Warpsync

| Name             | Description                                        | Type    |
| :--------------- | :------------------------------------------------- | :------ |
| advancementRange | The used advancement range per warpsync checkpoint | integer |

Example:

```json
  "warpsync": {
    "advancementRange": 150,
  }
```

## 16. Spammer

| Name          | Description                                                                         | Type    |
| :------------ | :---------------------------------------------------------------------------------- | :------ |
| message       | The message to embed within the spam messages                                       | string  |
| index         | The indexation of the message                                                       | string  |
| indexSemiLazy | The indexation of the message if the semi-lazy pool is used (uses "index" if empty) | string  |
| cpuMaxUsage   | Workers remains idle for a while when cpu usage gets over this limit (0 = disable)  | float   |
| mpsRateLimit  | The rate limit for the spammer (0 = no limit)                                       | float   |
| workers       | The amount of parallel running spammers                                             | integer |
| autostart     | Automatically start the spammer on node startup                                     | bool    |

Example:

```json
  "spammer": {
    "message": "Binary is the future.",
    "index": "HORNET Spammer",
    "indexSemiLazy": "HORNET Spammer Semi-Lazy",
    "cpuMaxUsage": 0.5,
    "mpsRateLimit": 0,
    "workers": 1,
    "autostart": false
  },
```

## 17. MQTT

| Name        | Description                                                         | Type    |
| :---------- | :------------------------------------------------------------------ | :------ |
| bindAddress | Bind address on which the MQTT broker listens on                    | string  |
| wsPort      | Port of the WebSocket MQTT broker                                   | integer |
| workerCount | Number of parallel workers the MQTT broker uses to publish messages | integer |

Example:

```json
  "mqtt": {
    "bindAddress": "localhost:1883",
    "wsPort": 1888,
    "workerCount": 100
  },
```

## 18. Profiling

| Name        | Description                                       | Type   |
| :---------- | :------------------------------------------------ | :----- |
| bindAddress | The bind address on which the profiler listens on | string |

Example:

```json
  "profiling": {
    "bindAddress": "localhost:6060"
  },
```

## 19. Prometheus

| Name                                          | Description                                                  | Type   |
| :-------------------------------------------- | :----------------------------------------------------------- | :----- |
| bindAddress                                   | The bind address on which the Prometheus exporter listens on | string |
| [fileServiceDiscovery](#fileservicediscovery) | Configuration for file service discovery                            | object |
| databaseMetrics                               | Include database metrics                                     | bool   |
| nodeMetrics                                   | Include node metrics                                         | bool   |
| gossipMetrics                                 | Include gossip metrics                                       | bool   |
| cachesMetrics                                 | Include caches metrics                                       | bool   |
| restAPIMetrics                                | Include restAPI metrics                                      | bool   |
| migrationMetrics                              | Include migration metrics                                    | bool   |
| coordinatorMetrics                            | Include coordinator metrics                                  | bool   |
| debugMetrics                                  | Include debug metrics                                        | bool   |
| goMetrics                                     | Include go metrics                                           | bool   |
| processMetrics                                | Include process metrics                                      | bool   |
| promhttpMetrics                               | Include promhttp metrics                                     | bool   |

### FileServiceDiscovery

| Name    | Description                                                 | Type   |
| :------ | :---------------------------------------------------------- | :----- |
| enabled | Whether the plugin should write a Prometheus 'file SD' file | bool   |
| path    | The path where to write the 'file SD' file to               | string |
| target  | The target to write into the 'file SD' file                 | string |

Example:

```json
  "prometheus": {
    "bindAddress": "localhost:9311",
    "fileServiceDiscovery": {
      "enabled": false,
      "path": "target.json",
      "target": "localhost:9311"
    },
    "databaseMetrics": true,
    "nodeMetrics": true,
    "gossipMetrics": true,
    "cachesMetrics": true,
    "restAPIMetrics": true,
    "migrationMetrics": true,
    "coordinatorMetrics": true,
    "debugMetrics": false,
    "goMetrics": false,
    "processMetrics": false,
    "promhttpMetrics": false
  }
```

## 20. Gossip

| Name               | Description                                       | Type   |
| :----------------- | :------------------------------------------------ | :----- |
| streamReadTimeout  | The read timeout for reads from the gossip stream | string |
| streamWriteTimeout | The write timeout for writes to the gossip stream | string |

Example:

```json
  "gossip": {
    "streamReadTimeout": "1m",
    "streamWriteTimeout": "10s",
  }
```

## 21. Debug

| Name                         | Description                                                                                              | Type   |
| :--------------------------- | :------------------------------------------------------------------------------------------------------- | :----- |
| whiteFlagParentsSolidTimeout | Defines the the maximum duration for the parents to become solid during white flag confirmation API call | string |

Example:

```json
  "debug": {
    "whiteFlagParentsSolidTimeout": "2s",
  }
```

## 22. Legacy

This is part the config used in the migration from IOTA 1.0 to IOTA 1.5 (Chrysalis)

## 22.1 Migrator

| Name                | Description                                            | Type    |
| :------------------ | :----------------------------------------------------- | :------ |
| queryCooldownPeriod | The cool down period of the service to ask for new data | string  |
| receiptMaxEntries   | The max amount of entries to embed within a receipt    | integer |
| stateFilePath       | Path to the state file of the migrator                 | string  |

Example:

```json
  "migrator": {
    "queryCooldownPeriod": "5s",
    "receiptMaxEntries": 110,
    "stateFilePath": "migrator.state",
  }
```

## 22.2 Receipts

| Name                    | Description          | Type   |
| :---------------------- | :------------------- | :----- |
| [backup](#backup)       | Configuration for backup    | object |
| [validator](#validator) | Configuration for validator | object |

### Backup

| Name    | Description                                     | Type   |
| :------ | :---------------------------------------------- | :----- |
| enabled | Whether to backup receipts in the backup folder | bool   |
| folder  | Path to the receipts backup folder              | string |

### Validator

| Name                        | Description                                                       | Type   |
| :-------------------------- | :---------------------------------------------------------------- | :----- |
| [api](#api)                 | Configuration for legacy API                                             | object |
| [coordinator](#coordinator) | Configuration for legacy Coordinator                                     | object |
| ignoreSoftErrors            | Whether to ignore soft errors and not panic if one is encountered | bool   |
| validate                    | Whether to validate receipts                                      | bool   |

#### Api

| Name    | Description                    | Type   |
| :------ | :----------------------------- | :----- |
| address | Address of the legacy node API | string |
| timeout | Timeout of API calls           | string |

#### Coordinator

| Name            | Description                                 | Type    |
| :-------------- | :------------------------------------------ | :------ |
| address         | Address of the legacy coordinator           | string  |
| merkleTreeDepth | Depth of the Merkle tree of the coordinator | integer |

Example:

```json
  "receipts": {
    "backup": {
      "enabled": false,
      "folder": "receipts",
    },
    "validator": {
      "api": {
        "address": "http://localhost:14266",
        "timeout": "5s",
      },
      "coordinator": {
        "address": "JFQ999DVN9CBBQX9DSAIQRAFRALIHJMYOXAQSTCJLGA9DLOKIWHJIFQKMCQ9QHWW9RXQMDBVUIQNIY9GZ",
        "merkleTreeDepth": 18,
      },
      "ignoreSoftErrors": false,
      "validate": false,
    },
  }
```

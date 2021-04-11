# config.json

Hornet uses the JSON standard as config file. If you are unsure about some syntax have a look at the official specs [here](https://www.json.org).
The default config file is `config.json`. You can change the path or name of the config file by using the `-c` or `--config` flag.
For Example: `hornet -c config_example.json`.
You can always get the most up-to-date description of the config parameters by running `hornet -h --full`.

## Table of content

- [Table of content](#table-of-content)
- [1. REST API](#1-rest-api)
  - [JWT Auth](#jwt-auth)
  - [Limits](#limits)
- [2. Dashboard](#2-dashboard)
  - [Auth](#auth)
- [3. DB](#3-db)
- [4. Snapshots](#4-snapshots)
  - [DownloadURLs](#downloadurls)
- [5. Pruning](#5-pruning)
- [6. Protocol](#6-protocol)
  - [PublicKeyRanges](#publickeyranges)
- [7. Proof of Work](#7-proof-of-work)
- [8. Requests](#8-requests)
- [9. Coordinator](#9-coordinator)
  - [Checkpoints](#checkpoints)
  - [Quorum](#quorum)
    - [Groups](#groups)
      - [{GROUP_NAME}](#group_name)
  - [Signing](#signing)
  - [Tipsel](#tipsel)
- [10. Tipsel](#10-tipsel)
  - [NonLazy](#nonlazy)
  - [SemiLazy](#semilazy)
- [11. Node](#11-node)
- [12. P2P](#12-p2p)
  - [ConnectionManager](#connectionmanager)
  - [PeerStore](#peerstore)
- [13. Logger](#13-logger)
- [14. Warpsync](#14-warpsync)
- [15. Spammer](#15-spammer)
- [16. MQTT](#16-mqtt)
- [17. Profiling](#17-profiling)
- [18. Prometheus](#18-prometheus)
  - [FileServiceDiscovery](#fileservicediscovery)
- [19. Gossip](#19-gossip)
- [20. Debug](#20-debug)
- [21. Legacy](#21-legacy)
- [21.1 Migrator](#211-migrator)
- [21.2 Receipts](#212-receipts)
  - [Backup](#backup)
  - [Validator](#validator)
    - [Api](#api)
    - [Coordinator](#coordinator)

* * *

## 1. REST API

| Name                       | Description                                                                     | Type             |
| :------------------------- | :------------------------------------------------------------------------------ | :--------------- |
| [jwtAuth](#jwt-auth)       | config for JWT auth                                                             | object           |
| permittedRoutes            | the allowed HTTP REST routes which can be called from non whitelisted addresses | array of strings |
| whitelistedAddresses       | the whitelist of addresses which are allowed to access the REST API             | array of strings |
| bindAddress                | the bind address on which the REST API listens on                               | string           |
| powEnabled                 | whether the node does PoW if messages are received via API                      | bool             |
| powWorkerCount             | the amount of workers used for calculating PoW when issuing messages via API    | integer          |
| [limits](#limits)          | config for api limits                                                           | object           |
| excludeHealthCheckFromAuth | whether to allow the health check route anyways                                 | bool             |

### JWT Auth

| Name    | Description                                                                                                                             | Type   |
| :------ | :-------------------------------------------------------------------------------------------------------------------------------------- | :----- |
| enabled | whether to use JWT auth for the REST API                                                                                                | bool   |
| salt    | salt used inside the JWT tokens for the REST API. Change this to a different value to invalidate JWT tokens not matching this new value | string |


### Limits

| Name       | Description                                                               | Type    |
| :--------- | :------------------------------------------------------------------------ | :------ |
| bodyLength | the maximum number of characters that the body of an API call may contain | string  |
| maxResults | the maximum number of results that may be returned by an endpoint         | integer |

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
| bindAddress   | the bind address on which the dashboard can be access from | string |
| dev           | whether to run the dashboard in dev mode                   | bool   |
| [auth](#auth) | config for dashboard auth                                  | object |

### Auth

| Name           | Description                                           | Type   |
| :------------- | :---------------------------------------------------- | :----- |
| sessionTimeout | how long the auth session should last before expiring | string |
| username       | the auth username                                     | string |
| passwordHash   | the auth password+salt as a scrypt hash               | string |
| passwordSalt   | the auth salt used for hashing the password           | string |

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
| engine           | the used database engine (pebble/bolt/rocksdb)                                      | string |
| path             | the path to the database folder                                                     | string |
| autoRevalidation | whether to automatically start revalidation on startup if the database is corrupted | bool   |
| debug            | ignore the check for corrupted databases (should only be used for debug reasons)    | bool   |

Example:

```json
  "db": {
    "engine": "pebble",
    "path": "mainnetdb",
    "autoRevalidation": false,
    "debug": false,
  },
```

## 4. Snapshots

| Name                          | Description                                                                                                                                                            | Type             |
| :---------------------------- | :--------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :--------------- |
| interval                      | interval, in milestones, at which snapshot files are created (snapshots are only created if the node is synced)                                                        | integer          |
| depth                         | the depth, respectively the starting point, at which a snapshot of the ledger is generated                                                                             | integer          |
| fullPath                      | path to the full snapshot file                                                                                                                                         | string           |
| deltaPath                     | path to the delta snapshot file                                                                                                                                        | string           |
| deltaSizeThresholdPercentage  | create a full snapshot if the size of a delta snapshot reaches a certain percentage of the full snapshot  (0.0 = always create delta snapshot to keep ms diff history) | float            |
| [downloadURLs](#downloadurls) | URLs to load the snapshot files from.                                                                                                                                  | array of objects |

### DownloadURLs

| Name  | Description                              | Type   |
| :---- | :--------------------------------------- | :----- |
| full  | download link to the full snapshot file  | string |
| delta | download link to the delta snapshot file | string |

Example:

```json
"snapshots": {
    "interval": 50,
    "depth": 50,
    "fullPath": "snapshots/mainnet/full_export.bin",
    "deltaPath": "snapshots/mainnet/delta_export.bin",
    "deltaSizeThresholdPercentage": 50.0,
    "downloadURLs": [
      {
        "full": "https://source1.example.com/full_export.bin",
        "delta": "https://source1.example.com/delta_export.bin"
      },
      {
        "full": "https://source2.example.com/full_export.bin",
        "delta": "https://source2.example.com/delta_export.bin"
      }
    ]
  },
```

## 5. Pruning

| Name          | Description                                           | Type    |
| :------------ | :---------------------------------------------------- | :------ |
| enabled       | whether to delete old message data from the database  | bool    |
| delay         | amount of milestone cones to keep in the database     | integer |
| pruneReceipts | whether to delete old receipts data from the database | bool    |

Example:

```json
  "pruning": {
    "enabled": true,
    "delay": 60480,
    "pruneReceipts": false
  },
```

## 6. Protocol

| Name                                | Description                                       | Type             |
| :---------------------------------- | :------------------------------------------------ | :--------------- |
| networkID                           | the network ID on which this node operates on     | string           |
| bech32HRP                           | the HRP which should be used for Bech32 addresses | string           |
| minPoWScore                         | the minimum PoW score required by the network     | float            |
| milestonePublicKeyCount             | the amount of public keys in a milestone          | integer          |
| [publicKeyRanges](#publickeyranges) | List of public key ranges from the coordinator    | array of objects |

### PublicKeyRanges

| Name  | Description           | Type    |
| :---- | :-------------------- | :------ |
| key   | public key            | string  |
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
| refreshTipsInterval | interval for refreshing tips during PoW for spammer messages and messages passed without parents via API | string |

Example:

```json
  "pow": {
    "refreshTipsInterval": "5s"
  },
```

## 8. Requests

| Name                     | Description                                           | Type   |
| :----------------------- | :---------------------------------------------------- | :----- |
| discardOlderThan         | the maximum time a request stays in the request queue | string |
| pendingReEnqueueInterval | the interval the pending requests are re-enqueued     | string |

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
| [checkpoints](#checkpoints) | config for checkpoints                                                                 | object  |
| interval                    | the interval milestones are issued                                                     | string  |
| powWorkerCount              | the amount of workers used for calculating PoW when issuing checkpoints and milestones | integer |
| [quorum](#quorum)           | config for quorum                                                                      | object  |
| [signing](#signing)         | config for signing                                                                     | object  |
| stateFilePath               | the path to the state file of the coordinator                                          | string  |
| [tipsel](#tipsel)           | config for tip selection                                                               | object  |

### Checkpoints

| Name               | Description                                                 | Type    |
| :----------------- | :---------------------------------------------------------- | :------ |
| maxTrackedMessages | maximum amount of known messages for milestone tipselection | integer |

### Quorum

| Name              | Description                                                                           | Type                   |
| :---------------- | :------------------------------------------------------------------------------------ | :--------------------- |
| enabled           | whether the coordinator quorum is enabled                                             | bool                   |
| [groups](#groups) | the quorum groups used to ask other nodes for correct ledger state of the coordinator | Array of object arrays |
| timeout           | the timeout until a node in the quorum must have answered                             | string                 |

#### Groups

| Name                        | Description                                                                          | Type             |
| :-------------------------- | :----------------------------------------------------------------------------------- | :--------------- |
| [{GROUP_NAME}](#group_name) | the qourum group used to ask other nodes for correct ledger state of the coordinator | Array of objects |

##### {GROUP_NAME}

| Name     | Description                           | Type   |
| :------- | :------------------------------------ | :----- |
| alias    | alias of the quorum client (optional) | string |
| baseURL  | baseURL of the quorum client          | string |
| userName | username for basic auth (optional)    | string |
| password | password for basic auth (optional)    | string |

### Signing

| Name          | Description                                                                  | Type   |
| :------------ | :--------------------------------------------------------------------------- | :----- |
| provider      | the signing provider the coordinator uses to sign a milestone (local/remote) | string |
| remoteAddress | the address of the remote signing provider (insecure connection!)            | string |

### Tipsel

| Name                                           | Description                                                       | Type    |
| :--------------------------------------------- | :---------------------------------------------------------------- | :------ |
| heaviestBranchSelectionTimeout                 | the maximum duration to select the heaviest branch tips           | string  |
| maxHeaviestBranchTipsPerCheckpoint             | maximum amount of checkpoint messages with heaviest branch tips   | integer |
| minHeaviestBranchUnreferencedMessagesThreshold | minimum threshold of unreferenced messages in the heaviest branch | integer |
| randomTipsPerCheckpoint                        | amount of checkpoint messages with random tips                    | integer |

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

## 10. Tipsel

| Name                                  | Description                                                                                                             | Type    |
| :------------------------------------ | :---------------------------------------------------------------------------------------------------------------------- | :------ |
| maxDeltaMsgYoungestConeRootIndexToCMI | the maximum allowed delta value for the YCRI of a given message in relation to the current CMI before it gets lazy      | integer |
| maxDeltaMsgOldestConeRootIndexToCMI   | the maximum allowed delta value between OCRI of a given message in relation to the current CMI before it gets semi-lazy | integer |
| belowMaxDepth                         | the maximum allowed delta value for the OCRI of a given message in relation to the current CMI before it gets lazy      | integer |
| [nonLazy](nonlazy)                    | config for tips from the non-lazy pool                                                                                  | object  |
| [semiLazy](semilazy)                  | config for tips from the semi-lazy pool                                                                                 | object  |

### NonLazy

| Name                    | Description                                                                                               | Type    |
| :---------------------- | :-------------------------------------------------------------------------------------------------------- | :------ |
| retentionRulesTipsLimit | the maximum number of current tips for which the retention rules are checked (non-lazy)                   | integer |
| maxReferencedTipAge     | the maximum time a tip remains in the tip pool after it was referenced by the first message (non-lazy)    | string  |
| maxChildren             | the maximum amount of references by other messages before the tip is removed from the tip pool (non-lazy) | integer |
| spammerTipsThreshold    | the maximum amount of tips in a tip-pool (non-lazy) before the spammer tries to reduce these              | integer |

### SemiLazy

| Name                    | Description                                                                                                | Type    |
| :---------------------- | :--------------------------------------------------------------------------------------------------------- | :------ |
| retentionRulesTipsLimit | the maximum number of current tips for which the retention rules are checked (semi-lazy)                   | integer |
| maxReferencedTipAge     | the maximum time a tip remains in the tip pool after it was referenced by the first message (semi-lazy)    | string  |
| maxChildren             | the maximum amount of references by other messages before the tip is removed from the tip pool (semi-lazy) | integer |
| spammerTipsThreshold    | the maximum amount of tips in a tip-pool (semi-lazy) before the spammer tries to reduce these              | integer |

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

## 11. Node

| Name           | Description                              | Type             |
| :------------- | :--------------------------------------- | :--------------- |
| alias          | the alias to identify a node             | string           |
| profile        | the profile the node runs with           | string           |
| disablePlugins | a list of plugins that shall be disabled | array of strings |
| enablePlugins  | a list of plugins that shall be enabled  | array of strings |

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

## 12. P2P

| Name                                    | Description                                                                    | Type             |
| :-------------------------------------- | :----------------------------------------------------------------------------- | :--------------- |
| bindMultiAddresses                      | the bind addresses for this node                                               | array of strings |
| [connectionManager](#connectionmanager) | config for connection manager                                                  | object           |
| gossipUnknownPeersLimit                 | maximum amount of unknown peers a gossip protocol connection is established to | integer          |
| identityPrivateKey                      | private key used to derive the node identity (optional)                        | string           |
| [peerStore](#peerstore)                 | config for peer store                                                          | object           |
| reconnectInterval                       | the time to wait before trying to reconnect to a disconnected peer             | string           |

### ConnectionManager

| Name          | Description                                                                  | Type    |
| :------------ | :--------------------------------------------------------------------------- | :------ |
| highWatermark | the threshold up on which connections count truncates to the lower watermark | integer |
| lowWatermark  | the minimum connections count to hold after the high watermark was reached   | integer |

### PeerStore

| Name | Description                | Type   |
| :--- | :------------------------- | :----- |
| path | the path to the peer store | string |

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
[//]: # "| advertiseInterval         | the interval at which the node advertises itself on the DHT for peer discovery           | string  |"
[//]: # "| maxDiscoveredPeerConns    | the max. amount of peers to be connected to which were discovered via the DHT rendezvous | integer |"
[//]: # "| rendezvousPoint           | the rendezvous string for advertising on the DHT that the node wants to peer with others | string  |"
[//]: # "| routingTableRefreshPeriod | the routing table refresh period                                                         | string  |"

[//]: # "Example:"

[//]: # "```json"
[//]: # '  "p2pdisc": {'
[//]: # '    "advertiseInterval": "30s",'
[//]: # '    "maxDiscoveredPeerConns": 4,'
[//]: # '    "rendezvousPoint": "between-two-vertices",'
[//]: # '    "routingTableRefreshPeriod": "1m",'
[//]: # "  },"
[//]: # "```"

## 13. Logger

| Name          | Description                                                                                                       | Type             |
| :------------ | :---------------------------------------------------------------------------------------------------------------- | :--------------- |
| level         | the minimum enabled logging level. Valid values are: "debug", "info", "warn", "error", "dpanic", "panic", "fatal" | string           |
| disableCaller | stops annotating logs with the calling function's file name and line number                                       | bool             |
| encoding      | sets the logger's encoding. Valid values are "json" and "console"                                                 | string           |
| outputPaths   | a list of URLs, file paths or stdout/stderr to write logging output to                                            | array of strings |

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

## 14. Warpsync

| Name             | Description                                        | Type    |
| :--------------- | :------------------------------------------------- | :------ |
| advancementRange | the used advancement range per warpsync checkpoint | integer |

Example:

```json
  "warpsync": {
    "advancementRange": 150,
  }
```

## 15. Spammer

| Name          | Description                                                                         | Type    |
| :------------ | :---------------------------------------------------------------------------------- | :------ |
| message       | the message to embed within the spam messages                                       | string  |
| index         | the indexation of the message                                                       | string  |
| indexSemiLazy | the indexation of the message if the semi-lazy pool is used (uses "index" if empty) | string  |
| cpuMaxUsage   | workers remains idle for a while when cpu usage gets over this limit (0 = disable)  | float   |
| mpsRateLimit  | the rate limit for the spammer (0 = no limit)                                       | float   |
| workers       | the amount of parallel running spammers                                             | integer |
| autostart     | automatically start the spammer on node startup                                     | bool    |

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

## 16. MQTT

| Name        | Description                                                         | Type    |
| :---------- | :------------------------------------------------------------------ | :------ |
| bindAddress | bind address on which the MQTT broker listens on                    | string  |
| wsPort      | port of the WebSocket MQTT broker                                   | integer |
| workerCount | number of parallel workers the MQTT broker uses to publish messages | integer |

Example:

```json
  "mqtt": {
    "bindAddress": "localhost:1883",
    "wsPort": 1888,
    "workerCount": 100
  },
```

## 17. Profiling

| Name        | Description                                       | Type   |
| :---------- | :------------------------------------------------ | :----- |
| bindAddress | the bind address on which the profiler listens on | string |

Example:

```json
  "profiling": {
    "bindAddress": "localhost:6060"
  },
```

## 18. Prometheus

| Name                                          | Description                                                  | Type   |
| :-------------------------------------------- | :----------------------------------------------------------- | :----- |
| bindAddress                                   | the bind address on which the Prometheus exporter listens on | string |
| [fileServiceDiscovery](#fileservicediscovery) | config for file service discovery                            | object |
| databaseMetrics                               | include database metrics                                     | bool   |
| nodeMetrics                                   | include node metrics                                         | bool   |
| gossipMetrics                                 | include gossip metrics                                       | bool   |
| cachesMetrics                                 | include caches metrics                                       | bool   |
| restAPIMetrics                                | include restAPI metrics                                      | bool   |
| migrationMetrics                              | include migration metrics                                    | bool   |
| coordinatorMetrics                            | include coordinator metrics                                  | bool   |
| debugMetrics                                  | include debug metrics                                        | bool   |
| goMetrics                                     | include go metrics                                           | bool   |
| processMetrics                                | include process metrics                                      | bool   |
| promhttpMetrics                               | include promhttp metrics                                     | bool   |

### FileServiceDiscovery

| Name    | Description                                                 | Type   |
| :------ | :---------------------------------------------------------- | :----- |
| enabled | whether the plugin should write a Prometheus 'file SD' file | bool   |
| path    | the path where to write the 'file SD' file to               | string |
| target  | the target to write into the 'file SD' file                 | string |

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

## 19. Gossip

| Name               | Description                                       | Type   |
| :----------------- | :------------------------------------------------ | :----- |
| streamReadTimeout  | the read timeout for reads from the gossip stream | string |
| streamWriteTimeout | the write timeout for writes to the gossip stream | string |

Example:

```json
  "gossip": {
    "streamReadTimeout": "1m",
    "streamWriteTimeout": "10s",
  }
```

## 20. Debug

| Name                         | Description                                                                                              | Type   |
| :--------------------------- | :------------------------------------------------------------------------------------------------------- | :----- |
| whiteFlagParentsSolidTimeout | defines the the maximum duration for the parents to become solid during white flag confirmation API call | string |

Example:

```json
  "debug": {
    "whiteFlagParentsSolidTimeout": "2s",
  }
```

## 21. Legacy

This is part the config used in the migration from IOTA 1.0 to IOTA 1.5 (Chrysalis)

## 21.1 Migrator

| Name                | Description                                            | Type    |
| :------------------ | :----------------------------------------------------- | :------ |
| queryCooldownPeriod | the cooldown period of the service to ask for new data | string  |
| receiptMaxEntries   | the max amount of entries to embed within a receipt    | integer |
| stateFilePath       | path to the state file of the migrator                 | string  |

Example:

```json
  "migrator": {
    "queryCooldownPeriod": "5s",
    "receiptMaxEntries": 110,
    "stateFilePath": "migrator.state",
  }
```

## 21.2 Receipts

| Name                    | Description          | Type   |
| :---------------------- | :------------------- | :----- |
| [backup](#backup)       | config for backup    | object |
| [validator](#validator) | config for validator | object |

### Backup

| Name    | Description                                     | Type   |
| :------ | :---------------------------------------------- | :----- |
| enabled | whether to backup receipts in the backup folder | bool   |
| folder  | path to the receipts backup folder              | string |

### Validator

| Name                        | Description                                                       | Type   |
| :-------------------------- | :---------------------------------------------------------------- | :----- |
| [api](#api)                 | config for legacy API                                             | object |
| [coordinator](#coordinator) | config for legacy Coordinator                                     | object |
| ignoreSoftErrors            | whether to ignore soft errors and not panic if one is encountered | bool   |
| validate                    | whether to validate receipts                                      | bool   |

#### Api

| Name    | Description                    | Type   |
| :------ | :----------------------------- | :----- |
| address | address of the legacy node API | string |
| timeout | timeout of API calls           | string |

#### Coordinator

| Name            | Description                                 | Type    |
| :-------------- | :------------------------------------------ | :------ |
| address         | address of the legacy coordinator           | string  |
| merkleTreeDepth | depth of the Merkle tree of the coordinator | integer |

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

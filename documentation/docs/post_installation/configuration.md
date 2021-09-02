---
keywords:
- IOTA Node 
- Hornet Node
- Configuration
- JSON
- Customize
- Config
description: This section describes the configuration parameters and their types for your Hornet node.
image: /img/logo/HornetLogo.png
---


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

## 1. REST API

| Name                       | Description                                                                     | Type             |
| :------------------------- | :------------------------------------------------------------------------------ | :--------------- |
| bindAddress                | The bind address on which the REST API listens on                               | string           |
| [jwtAuth](#jwt-auth)       | Config for JWT auth                                                             | object           |
| excludeHealthCheckFromAuth | Whether to allow the health check route anyways                                 | bool             |
| permittedRoutes            | The allowed HTTP REST routes which can be called from non whitelisted addresses | array of strings |
| whitelistedAddresses       | The whitelist of addresses which are allowed to access the REST API             | array of strings |
| powEnabled                 | Whether the node does PoW if messages are received via API                      | bool             |
| powWorkerCount             | The amount of workers used for calculating PoW when issuing messages via API    | integer          |
| [limits](#limits)          | Configuration for api limits                                                    | object           |

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
    "bindAddress": "0.0.0.0:14265",
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
    "powEnabled": true,
    "powWorkerCount": 1,
    "limits": {
      "bodyLength": "1M",
      "maxResults": 1000
    }
  },
```

## 2. Dashboard

| Name          | Description                                                  | Type   |
| :------------ | :----------------------------------------------------------- | :----- |
| bindAddress   | The bind address on which the dashboard can be accessed from | string |
| dev           | Whether to run the dashboard in dev mode                     | bool   |
| [auth](#auth) | Configuration for dashboard auth                             | object |

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
| engine           | The used database engine (pebble/rocksdb)                                           | string |
| path             | The path to the database folder                                                     | string |
| autoRevalidation | Whether to automatically start revalidation on startup if the database is corrupted | bool   |

Example:

```json
  "db": {
    "engine": "rocksdb",
    "path": "mainnetdb",
    "autoRevalidation": false
  },
```

## 4. Snapshots

| Name                          | Description                                                                                                                                                            | Type             |
| :---------------------------- | :--------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :--------------- |
| depth                         | The depth, respectively the starting point, at which a snapshot of the ledger is generated                                                                             | integer          |
| interval                      | Interval, in milestones, at which snapshot files are created (snapshots are only created if the node is synced)                                                        | integer          |
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
    "depth": 50,
    "interval": 200,
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
| cooldownTime        | Cool down time between two pruning by database size events                          | string |

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
    "networkID": "chrysalis-mainnet",
    "bech32HRP": "iota",
    "minPoWScore": 4000.0,
    "milestonePublicKeyCount": 2,
    "publicKeyRanges": [
      {
        "key": "a9b46fe743df783dedd00c954612428b34241f5913cf249d75bed3aafd65e4cd",
        "start": 0,
        "end": 777600
      },
      {
        "key": "365fb85e7568b9b32f7359d6cbafa9814472ad0ecbad32d77beaf5dd9e84c6ba",
        "start": 0,
        "end": 1555200
      },
      {
        "key": "ba6d07d1a1aea969e7e435f9f7d1b736ea9e0fcb8de400bf855dba7f2a57e947",
        "start": 552960,
        "end": 2108160
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
| stateFilePath               | The path to the state file of the coordinator                                          | string  |
| interval                    | The interval milestones are issued                                                     | string  |
| powWorkerCount              | The amount of workers used for calculating PoW when issuing checkpoints and milestones | integer |
| [checkpoints](#checkpoints) | Configuration for checkpoints                                                          | object  |
| [tipsel](#tipsel)           | Configuration for tip selection                                                        | object  |
| [signing](#signing)         | Configuration for signing                                                              | object  |
| [quorum](#quorum)           | Configuration for quorum                                                               | object  |

### Checkpoints

| Name               | Description                                                  | Type    |
| :----------------- | :----------------------------------------------------------- | :------ |
| maxTrackedMessages | Maximum amount of known messages for milestone tip selection | integer |

### Tipsel

| Name                                           | Description                                                       | Type    |
| :--------------------------------------------- | :---------------------------------------------------------------- | :------ |
| minHeaviestBranchUnreferencedMessagesThreshold | Minimum threshold of unreferenced messages in the heaviest branch | integer |
| maxHeaviestBranchTipsPerCheckpoint             | Maximum amount of checkpoint messages with heaviest branch tips   | integer |
| randomTipsPerCheckpoint                        | Amount of checkpoint messages with random tips                    | integer |
| heaviestBranchSelectionTimeout                 | The maximum duration to select the heaviest branch tips           | string  |

### Signing

| Name          | Description                                                                  | Type    |
| :------------ | :--------------------------------------------------------------------------- | :------ |
| provider      | The signing provider the coordinator uses to sign a milestone (local/remote) | string  |
| remoteAddress | The address of the remote signing provider (insecure connection!)            | string  |
| retryAmount   | Number of signing retries to perform before shutting down the node           | integer |
| retryTimeout  | The timeout between signing retries                                          | string  |

### Quorum

| Name              | Description                                                                           | Type                   |
| :---------------- | :------------------------------------------------------------------------------------ | :--------------------- |
| enabled           | Whether the coordinator quorum is enabled                                             | bool                   |
| [groups](#groups) | The quorum groups used to ask other nodes for correct ledger state of the coordinator | array of object arrays |
| timeout           | The timeout until a node in the quorum must have answered                             | string                 |

#### Groups

| Name                        | Description                                                                          | Type             |
| :-------------------------- | :----------------------------------------------------------------------------------- | :--------------- |
| [{GROUP_NAME}](#group_name) | The quorum group used to ask other nodes for correct ledger state of the coordinator | array of objects |

##### {GROUP_NAME}

| Name     | Description                           | Type   |
| :------- | :------------------------------------ | :----- |
| alias    | Alias of the quorum client (optional) | string |
| baseURL  | BaseURL of the quorum client          | string |
| userName | Username for basic auth (optional)    | string |
| password | Password for basic auth (optional)    | string |

Example:

```json
  "coordinator": {
    "stateFilePath": "coordinator.state",
    "interval": "10s",
    "powWorkerCount": 0,
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
      "remoteAddress": "localhost:12345",
      "retryAmount": 10,
      "retryTimeout": "2s"
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

## 10. Migrator

This part is used in the migration from IOTA 1.0 to IOTA 1.5 (Chrysalis)

| Name                | Description                                             | Type    |
| :------------------ | :------------------------------------------------------ | :------ |
| stateFilePath       | Path to the state file of the migrator                  | string  |
| receiptMaxEntries   | The max amount of entries to embed within a receipt     | integer |
| queryCooldownPeriod | The cool down period of the service to ask for new data | string  |

Example:

```json
  "migrator": {
    "stateFilePath": "migrator.state",
    "receiptMaxEntries": 110,
    "queryCooldownPeriod": "5s"
  },
```

## 11. Receipts

This part is used in the migration from IOTA 1.0 to IOTA 1.5 (Chrysalis)

| Name                    | Description                 | Type   |
| :---------------------- | :-------------------------- | :----- |
| [backup](#backup)       | Configuration for backup    | object |
| [validator](#validator) | Configuration for validator | object |

### Backup

| Name    | Description                                     | Type   |
| :------ | :---------------------------------------------- | :----- |
| enabled | Whether to backup receipts in the backup folder | bool   |
| path    | Path to the receipts backup folder              | string |

### Validator

| Name                        | Description                                                       | Type   |
| :-------------------------- | :---------------------------------------------------------------- | :----- |
| validate                    | Whether to validate receipts                                      | bool   |
| ignoreSoftErrors            | Whether to ignore soft errors and not panic if one is encountered | bool   |
| [api](#api)                 | Configuration for legacy API                                      | object |
| [coordinator](#coordinator) | Configuration for legacy Coordinator                              | object |

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
      "path": "receipts"
    },
    "validator": {
      "validate": false,
      "ignoreSoftErrors": false,
      "api": {
        "address": "http://localhost:14266",
        "timeout": "5s"
      },
      "coordinator": {
        "address": "UDYXTZBE9GZGPM9SSQV9LTZNDLJIZMPUVVXYXFYVBLIEUHLSEWFTKZZLXYRHHWVQV9MNNX9KZC9D9UZWZ",
        "merkleTreeDepth": 24
      }
    }
  },
```

## 12. Tangle

| Name             | Description                                                                       | Type   |
| :--------------- | :-------------------------------------------------------------------------------- | :----- |
| milestoneTimeout | The interval milestone timeout events are fired if no new milestones are received | string |

Example:

```json
  "tangle": {
    "milestoneTimeout": "30s"
  },
```

## 13. Tipsel

| Name                                  | Description                                                                                                             | Type    |
| :------------------------------------ | :---------------------------------------------------------------------------------------------------------------------- | :------ |
| maxDeltaMsgYoungestConeRootIndexToCMI | The maximum allowed delta value for the YCRI of a given message in relation to the current CMI before it gets lazy      | integer |
| maxDeltaMsgOldestConeRootIndexToCMI   | The maximum allowed delta value between OCRI of a given message in relation to the current CMI before it gets semi-lazy | integer |
| belowMaxDepth                         | The maximum allowed delta value for the OCRI of a given message in relation to the current CMI before it gets lazy      | integer |
| [nonLazy](#nonlazy)                   | Configuration for tips from the non-lazy pool                                                                           | object  |
| [semiLazy](#semilazy)                 | Configuration for tips from the semi-lazy pool                                                                          | object  |

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

## 14. Node

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

## 15. P2P

| Name                                    | Description                                                        | Type             |
| :-------------------------------------- | :----------------------------------------------------------------- | :--------------- |
| bindMultiAddresses                      | The bind addresses for this node                                   | array of strings |
| [connectionManager](#connectionmanager) | Configuration for connection manager                               | object           |
| [gossip](#gossip)                       | Configuration for gossip protocol                                  | object           |
| identityPrivateKey                      | private key used to derive the node identity (optional)            | string           |
| [db](#database)                         | Configuration for p2p database                                     | object           |
| reconnectInterval                       | The time to wait before trying to reconnect to a disconnected peer | string           |
| [autopeering](#autopeering)             | Configuration for autopeering                                      | object           |

### ConnectionManager

| Name          | Description                                                                  | Type    |
| :------------ | :--------------------------------------------------------------------------- | :------ |
| highWatermark | The threshold up on which connections count truncates to the lower watermark | integer |
| lowWatermark  | The minimum connections count to hold after the high watermark was reached   | integer |

### Gossip

| Name               | Description                                                                    | Type    |
| :----------------- | :----------------------------------------------------------------------------- | :------ |
| unknownPeersLimit  | maximum amount of unknown peers a gossip protocol connection is established to | integer |
| streamReadTimeout  | The read timeout for subsequent reads from the gossip stream                   | string  |
| streamWriteTimeout | The write timeout for writes to the gossip stream                              | string  |

### Database

| Name | Description                  | Type   |
| :--- | :--------------------------- | :----- |
| path | The path to the p2p database | string |

### Autopeering

| Name                 | Description                                                      | Type             |
| :------------------- | :--------------------------------------------------------------- | :--------------- |
| bindAddress          | The bind address on which the autopeering module listens on      | string           |
| entryNodes           | The list of autopeering entry nodes to use                       | array of strings |
| entryNodesPreferIPv6 | Defines if connecting over IPv6 is preferred for entry nodes     | bool             |
| runAsEntryNode       | Defines whether the node should act as an autopeering entry node | bool             |

Example:

```json
  "p2p": {
    "bindMultiAddresses": [
      "/ip4/0.0.0.0/tcp/15600",
      "/ip6/::/tcp/15600"
    ],
    "connectionManager": {
      "highWatermark": 10,
      "lowWatermark": 5
    },
    "gossip": {
      "unknownPeersLimit": 4,
      "streamReadTimeout": "1m0s",
      "streamWriteTimeout": "10s"
    },
    "identityPrivateKey": "",
    "db": {
      "path": "p2pstore"
    },
    "reconnectInterval": "30s",
    "autopeering": {
      "bindAddress": "0.0.0.0:14626",
      "entryNodes": [
        "/dns/lucamoser.ch/udp/14826/autopeering/4H6WV54tB29u8xCcEaMGQMn37LFvM1ynNpp27TTXaqNM",
        "/dns/entry-hornet-0.h.chrysalis-mainnet.iotaledger.net/udp/14626/autopeering/iotaPHdAn7eueBnXtikZMwhfPXaeGJGXDt4RBuLuGgb",
        "/dns/entry-hornet-1.h.chrysalis-mainnet.iotaledger.net/udp/14626/autopeering/iotaJJqMd5CQvv1A61coSQCYW9PNT1QKPs7xh2Qg5K2",
        "/dns/entry-mainnet.tanglebay.com/udp/14626/autopeering/iot4By1FD4pFLrGJ6AAe7YEeSu9RbW9xnPUmxMdQenC"
      ],
      "entryNodesPreferIPv6": false,
      "runAsEntryNode": false
    }
  },
```

## 16. Logger

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

## 17. Warpsync

| Name             | Description                                        | Type    |
| :--------------- | :------------------------------------------------- | :------ |
| advancementRange | The used advancement range per warpsync checkpoint | integer |

Example:

```json
  "warpsync": {
    "advancementRange": 150
  },
```

## 18. Spammer

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
    "message": "IOTA - A new dawn",
    "index": "HORNET Spammer",
    "indexSemiLazy": "HORNET Spammer Semi-Lazy",
    "cpuMaxUsage": 0.8,
    "mpsRateLimit": 0.0,
    "workers": 0,
    "autostart": false
  },
```

## 19. Faucet

| Name                | Description                                                                                                                  | Type    |
| :------------------ | :--------------------------------------------------------------------------------------------------------------------------- | :------ |
| amount              | The amount of funds the requester receives                                                                                   | integer |
| smallAmount         | The amount of funds the requester receives if the target address has more funds than the faucet amount and less than maximum | integer |
| maxAddressBalance   | The maximum allowed amount of funds on the target address                                                                    | integer |
| maxOutputCount      | The maximum output count per faucet message                                                                                  | integer |
| indexationMessage   | The faucet transaction indexation payload                                                                                    | string  |
| batchTimeout        | The maximum duration for collecting faucet batches                                                                           | string  |
| powWorkerCount      | The amount of workers used for calculating PoW when issuing faucet messages                                                  | integer |
| [website](#website) | Configuration for the faucet website                                                                                         | object  |

### Website

| Name        | Description                                                       | Type   |
| :---------- | :---------------------------------------------------------------- | :----- |
| bindAddress | The bind address on which the faucet website can be accessed from | string |
| enabled     | Whether to host the faucet website                                | bool   |

Example:

```json
  "faucet": {
    "amount": 10000000,
    "smallAmount": 1000000,
    "maxAddressBalance": 20000000,
    "maxOutputCount": 127,
    "indexationMessage": "HORNET FAUCET",
    "batchTimeout": "2s",
    "powWorkerCount": 0,
    "website": {
      "bindAddress": "localhost:8091",
      "enabled": true
    }
  },
```

## 20. MQTT

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

## 21. Profiling

| Name        | Description                                       | Type   |
| :---------- | :------------------------------------------------ | :----- |
| bindAddress | The bind address on which the profiler listens on | string |

Example:

```json
  "profiling": {
    "bindAddress": "localhost:6060"
  },
```

## 22. Prometheus

| Name                                          | Description                                                  | Type   |
| :-------------------------------------------- | :----------------------------------------------------------- | :----- |
| bindAddress                                   | The bind address on which the Prometheus exporter listens on | string |
| [fileServiceDiscovery](#fileservicediscovery) | Configuration for file service discovery                     | object |
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
  },
```

## 23. Debug

| Name                         | Description                                                                                              | Type   |
| :--------------------------- | :------------------------------------------------------------------------------------------------------- | :----- |
| whiteFlagParentsSolidTimeout | Defines the the maximum duration for the parents to become solid during white flag confirmation API call | string |

Example:

```json
  "debug": {
    "whiteFlagParentsSolidTimeout": "2s"
  },
```

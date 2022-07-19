---
description: This section describes the configuration parameters and their types for your Hornet node.
image: /img/Banner/banner_hornet_configuration.png
keywords:
- IOTA Node 
- Hornet Node
- Configuration
- JSON
- Customize
- Config
- reference
---


# Configuration

![Hornet Node Configuration](/img/Banner/banner_hornet_configuration.png)

HORNET uses a JSON standard format as a config file. If you are unsure about JSON syntax, you can find more information in the [official JSON specs](https://www.json.org).

You can change the path of the config file by using the `-c` or `--config` argument while executing `hornet` executable.

For example:
```bash
hornet -c config_example.json
```

You can always get the most up-to-date description of the config parameters by running:

```bash
hornet -h --full
```

## <a id="app"></a> 1. Application

| Name            | Description                                                                                            | Type    | Default value |
| --------------- | ------------------------------------------------------------------------------------------------------ | ------- | ------------- |
| checkForUpdates | Whether to check for updates of the application or not                                                 | boolean | true          |
| stopGracePeriod | The maximum time to wait for background processes to finish during shutdown before terminating the app | string  | "5m"          |

Example:

```json
  {
    "app": {
      "checkForUpdates": true,
      "stopGracePeriod": "5m"
    }
  }
```

## <a id="node"></a> 2. Node

| Name    | Description                     | Type   | Default value |
| ------- | ------------------------------- | ------ | ------------- |
| profile | The profile the node runs with  | string | "auto"        |
| alias   | Set an alias to identify a node | string | "HORNET node" |

Example:

```json
  {
    "node": {
      "profile": "auto",
      "alias": "HORNET node"
    }
  }
```

## <a id="protocol"></a> 3. Protocol

| Name                                         | Description                                             | Type   | Default value     |
| -------------------------------------------- | ------------------------------------------------------- | ------ | ----------------- |
| targetNetworkName                            | The initial network name on which this node operates on | string | "testnet"         |
| milestonePublicKeyCount                      | The amount of public keys in a milestone                | int    | 7                 |
| [baseToken](#protocol_basetoken)             | Configuration for baseToken                             | object |                   |
| [publicKeyRanges](#protocol_publickeyranges) | Configuration for publicKeyRanges                       | array  | see example below |

### <a id="protocol_basetoken"></a> BaseToken

| Name            | Description                           | Type    | Default value |
| --------------- | ------------------------------------- | ------- | ------------- |
| name            | The base token name                   | string  | "Shimmer"     |
| tickerSymbol    | The base token ticker symbol          | string  | "SMR"         |
| unit            | The base token unit                   | string  | "SMR"         |
| subunit         | The base token subunit                | string  | "glow"        |
| decimals        | The base token amount of decimals     | uint    | 6             |
| useMetricPrefix | The base token uses the metric prefix | boolean | false         |

### <a id="protocol_publickeyranges"></a> PublicKeyRanges

| Name       | Description                                                     | Type   | Default value                                                      |
| ---------- | --------------------------------------------------------------- | ------ | ------------------------------------------------------------------ |
| key        | The ed25519 public key of the coordinator in hex representation | string | "0000000000000000000000000000000000000000000000000000000000000000" |
| startIndex | The start milestone index of the public key                     | uint   | 0                                                                  |
| endIndex   | The end milestone index of the public key                       | uint   | 0                                                                  |

Example:

```json
  {
    "protocol": {
      "targetNetworkName": "testnet",
      "milestonePublicKeyCount": 7,
      "baseToken": {
        "name": "Shimmer",
        "tickerSymbol": "SMR",
        "unit": "SMR",
        "subunit": "glow",
        "decimals": 6,
        "useMetricPrefix": false
      },
      "publicKeyRanges": [
        {
          "key": "13ccdc2f5d3d9a3ebe06074c6b49b49090dd79ca72e04abf20f10f871ad8293b",
          "start": 0,
          "end": 0
        },
        {
          "key": "f18f3f6a2d940b9bacd3084713f6877db22064ada4335cb53ae1da75044f978d",
          "start": 0,
          "end": 0
        },
        {
          "key": "b3b4c920909720ba5f7c30dddc0f9169bf8243b529b601fc4776b8cb0a8ca253",
          "start": 0,
          "end": 0
        },
        {
          "key": "bded01e93adf7a623118fd375fd93dc7d7ddf222324239cae33e4e4c47ec3b0e",
          "start": 0,
          "end": 0
        },
        {
          "key": "488ac3fb1b8df5ef8c4acb4ef1f3e3d039c5d7197db87094a61af66320722313",
          "start": 0,
          "end": 0
        },
        {
          "key": "61f95fed30b6e9bf0b2d03938f56d35789ff7f0ea122d01c5c1b7e869525e218",
          "start": 0,
          "end": 0
        },
        {
          "key": "4587040de05907b70806c8725bdae1f7370785993b2a139208e247885d4ed1f8",
          "start": 0,
          "end": 0
        },
        {
          "key": "aa6b36116206cc7d6c8f688e22113aa46f0de88d51aa7acf881ec2bd9d015f62",
          "start": 0,
          "end": 0
        },
        {
          "key": "ede9760c7f2aaa4618a58a1357705cdc1874962ad369309543230394bb77548b",
          "start": 0,
          "end": 0
        },
        {
          "key": "98d1f907caa99f9320f0e0eb64a5cf208751c2171c7938da5659328061e82a8e",
          "start": 0,
          "end": 0
        }
      ]
    }
  }
```

## <a id="db"></a> 4. Database

| Name             | Description                                                                         | Type    | Default value      |
| ---------------- | ----------------------------------------------------------------------------------- | ------- | ------------------ |
| engine           | The used database engine (pebble/rocksdb/mapdb)                                     | string  | "rocksdb"          |
| path             | The path to the database folder                                                     | string  | "testnet/database" |
| autoRevalidation | Whether to automatically start revalidation on startup if the database is corrupted | boolean | false              |

Example:

```json
  {
    "db": {
      "engine": "rocksdb",
      "path": "testnet/database",
      "autoRevalidation": false
    }
  }
```

## <a id="pow"></a> 5. Proof of Work

| Name                | Description                                                                       | Type   | Default value |
| ------------------- | --------------------------------------------------------------------------------- | ------ | ------------- |
| refreshTipsInterval | Interval for refreshing tips during PoW for blocks passed without parents via API | string | "5s"          |

Example:

```json
  {
    "pow": {
      "refreshTipsInterval": "5s"
    }
  }
```

## <a id="p2p"></a> 6. Peer to Peer

| Name                                        | Description                                                        | Type   | Default value                                |
| ------------------------------------------- | ------------------------------------------------------------------ | ------ | -------------------------------------------- |
| bindMultiAddresses                          | The bind addresses for this node                                   | array  | /ip4/0.0.0.0/tcp/15600<br/>/ip6/::/tcp/15600 |
| [connectionManager](#p2p_connectionmanager) | Configuration for connectionManager                                | object |                                              |
| identityPrivateKey                          | Private key used to derive the node identity (optional)            | string | ""                                           |
| [db](#p2p_db)                               | Configuration for Database                                         | object |                                              |
| reconnectInterval                           | The time to wait before trying to reconnect to a disconnected peer | string | "30s"                                        |
| [gossip](#p2p_gossip)                       | Configuration for gossip                                           | object |                                              |
| [autopeering](#p2p_autopeering)             | Configuration for autopeering                                      | object |                                              |

### <a id="p2p_connectionmanager"></a> ConnectionManager

| Name          | Description                                                                  | Type | Default value |
| ------------- | ---------------------------------------------------------------------------- | ---- | ------------- |
| highWatermark | The threshold up on which connections count truncates to the lower watermark | int  | 10            |
| lowWatermark  | The minimum connections count to hold after the high watermark was reached   | int  | 5             |

### <a id="p2p_db"></a> Database

| Name | Description                  | Type   | Default value      |
| ---- | ---------------------------- | ------ | ------------------ |
| path | The path to the p2p database | string | "testnet/p2pstore" |

### <a id="p2p_gossip"></a> Gossip

| Name               | Description                                                                    | Type   | Default value |
| ------------------ | ------------------------------------------------------------------------------ | ------ | ------------- |
| unknownPeersLimit  | Maximum amount of unknown peers a gossip protocol connection is established to | int    | 4             |
| streamReadTimeout  | The read timeout for reads from the gossip stream                              | string | "1m"          |
| streamWriteTimeout | The write timeout for writes to the gossip stream                              | string | "10s"         |

### <a id="p2p_autopeering"></a> Autopeering

| Name                 | Description                                                  | Type    | Default value                                                                                                                                                                                                                         |
| -------------------- | ------------------------------------------------------------ | ------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| enabled              | Whether the autopeering plugin is enabled                    | boolean | false                                                                                                                                                                                                                                 |
| bindAddress          | Bind address for autopeering                                 | string  | "0.0.0.0:14626"                                                                                                                                                                                                                       |
| entryNodes           | List of autopeering entry nodes to use                       | array   | /dns/entry-hornet-0.h.testnet.shimmer.network/udp/14626/autopeering/ANrRwJv2xs1S7TonyenM9qzkB8hfZ4Y6Gg2xsNUGozTJ<br/>/dns/entry-hornet-1.h.testnet.shimmer.network/udp/14626/autopeering/3bTUFwKXzhHSv2kBs6gja8BbeawHNzMwUdJraXWmLkNk |
| entryNodesPreferIPv6 | Defines if connecting over IPv6 is preferred for entry nodes | boolean | false                                                                                                                                                                                                                                 |
| runAsEntryNode       | Whether the node should act as an autopeering entry node     | boolean | false                                                                                                                                                                                                                                 |

Example:

```json
  {
    "p2p": {
      "bindMultiAddresses": [
        "/ip4/0.0.0.0/tcp/15600",
        "/ip6/::/tcp/15600"
      ],
      "connectionManager": {
        "highWatermark": 10,
        "lowWatermark": 5
      },
      "identityPrivateKey": "",
      "db": {
        "path": "testnet/p2pstore"
      },
      "reconnectInterval": "30s",
      "gossip": {
        "unknownPeersLimit": 4,
        "streamReadTimeout": "1m",
        "streamWriteTimeout": "10s"
      },
      "autopeering": {
        "enabled": false,
        "bindAddress": "0.0.0.0:14626",
        "entryNodes": [
          "/dns/entry-hornet-0.h.testnet.shimmer.network/udp/14626/autopeering/ANrRwJv2xs1S7TonyenM9qzkB8hfZ4Y6Gg2xsNUGozTJ",
          "/dns/entry-hornet-1.h.testnet.shimmer.network/udp/14626/autopeering/3bTUFwKXzhHSv2kBs6gja8BbeawHNzMwUdJraXWmLkNk"
        ],
        "entryNodesPreferIPv6": false,
        "runAsEntryNode": false
      }
    }
  }
```

## <a id="requests"></a> 7. Requests

| Name                     | Description                                           | Type   | Default value |
| ------------------------ | ----------------------------------------------------- | ------ | ------------- |
| discardOlderThan         | The maximum time a request stays in the request queue | string | "15s"         |
| pendingReEnqueueInterval | The interval the pending requests are re-enqueued     | string | "5s"          |

Example:

```json
  {
    "requests": {
      "discardOlderThan": "15s",
      "pendingReEnqueueInterval": "5s"
    }
  }
```

## <a id="tangle"></a> 8. Tangle

| Name                                    | Description                                                                                                           | Type   | Default value |
| --------------------------------------- | --------------------------------------------------------------------------------------------------------------------- | ------ | ------------- |
| milestoneTimeout                        | The interval milestone timeout events are fired if no new milestones are received                                     | string | "30s"         |
| maxDeltaBlockYoungestConeRootIndexToCMI | The maximum allowed delta value for the YCRI of a given block in relation to the current CMI before it gets lazy      | int    | 8             |
| maxDeltaBlockOldestConeRootIndexToCMI   | The maximum allowed delta value between OCRI of a given block in relation to the current CMI before it gets semi-lazy | int    | 13            |
| whiteFlagParentsSolidTimeout            | Defines the the maximum duration for the parents to become solid during white flag confirmation API or INX call       | string | "2s"          |

Example:

```json
  {
    "tangle": {
      "milestoneTimeout": "30s",
      "maxDeltaBlockYoungestConeRootIndexToCMI": 8,
      "maxDeltaBlockOldestConeRootIndexToCMI": 13,
      "whiteFlagParentsSolidTimeout": "2s"
    }
  }
```

## <a id="snapshots"></a> 9. Snapshots

| Name                                    | Description                                                                                                                                                           | Type   | Default value                          |
| --------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ | -------------------------------------- |
| depth                                   | The depth, respectively the starting point, at which a snapshot of the ledger is generated                                                                            | int    | 50                                     |
| interval                                | Interval, in milestones, at which snapshot files are created (snapshots are only created if the node is synced)                                                       | int    | 200                                    |
| fullPath                                | Path to the full snapshot file                                                                                                                                        | string | "testnet/snapshots/full_snapshot.bin"  |
| deltaPath                               | Path to the delta snapshot file                                                                                                                                       | string | "testnet/snapshots/delta_snapshot.bin" |
| deltaSizeThresholdPercentage            | Create a full snapshot if the size of a delta snapshot reaches a certain percentage of the full snapshot (0.0 = always create delta snapshot to keep ms diff history) | float  | 50.0                                   |
| deltaSizeThresholdMinSize               | The minimum size of the delta snapshot file before the threshold percentage condition is checked (below that size the delta snapshot is always created)               | string | "50M"                                  |
| [downloadURLs](#snapshots_downloadurls) | Configuration for downloadURLs                                                                                                                                        | array  | see example below                      |

### <a id="snapshots_downloadurls"></a> DownloadURLs

| Name  | Description                    | Type   | Default value |
| ----- | ------------------------------ | ------ | ------------- |
| full  | URL of the full snapshot file  | string | ""            |
| delta | URL of the delta snapshot file | string | ""            |

Example:

```json
  {
    "snapshots": {
      "depth": 50,
      "interval": 200,
      "fullPath": "testnet/snapshots/full_snapshot.bin",
      "deltaPath": "testnet/snapshots/delta_snapshot.bin",
      "deltaSizeThresholdPercentage": 50,
      "deltaSizeThresholdMinSize": "50M",
      "downloadURLs": [
        {
          "full": "https://files.testnet.shimmer.network/snapshots/latest-full_snapshot.bin",
          "delta": "https://files.testnet.shimmer.network/snapshots/latest-delta_snapshot.bin"
        }
      ]
    }
  }
```

## <a id="pruning"></a> 10. Pruning

| Name                              | Description                                           | Type    | Default value |
| --------------------------------- | ----------------------------------------------------- | ------- | ------------- |
| [milestones](#pruning_milestones) | Configuration for milestones                          | object  |               |
| [size](#pruning_size)             | Configuration for size                                | object  |               |
| pruneReceipts                     | Whether to delete old receipts data from the database | boolean | false         |

### <a id="pruning_milestones"></a> Milestones

| Name                | Description                                                                            | Type    | Default value |
| ------------------- | -------------------------------------------------------------------------------------- | ------- | ------------- |
| enabled             | Whether to delete old block data from the database based on maximum milestones to keep | boolean | false         |
| maxMilestonesToKeep | Maximum amount of milestone cones to keep in the database                              | int     | 60480         |

### <a id="pruning_size"></a> Size

| Name                | Description                                                                       | Type    | Default value |
| ------------------- | --------------------------------------------------------------------------------- | ------- | ------------- |
| enabled             | Whether to delete old block data from the database based on maximum database size | boolean | true          |
| targetSize          | Target size of the database                                                       | string  | "30GB"        |
| thresholdPercentage | The percentage the database size gets reduced if the target size is reached       | float   | 10.0          |
| cooldownTime        | Cooldown time between two pruning by database size events                         | string  | "5m"          |

Example:

```json
  {
    "pruning": {
      "milestones": {
        "enabled": false,
        "maxMilestonesToKeep": 60480
      },
      "size": {
        "enabled": true,
        "targetSize": "30GB",
        "thresholdPercentage": 10,
        "cooldownTime": "5m"
      },
      "pruneReceipts": false
    }
  }
```

## <a id="profiling"></a> 11. Profiling

| Name        | Description                                       | Type    | Default value    |
| ----------- | ------------------------------------------------- | ------- | ---------------- |
| enabled     | Whether the profiling plugin is enabled           | boolean | false            |
| bindAddress | The bind address on which the profiler listens on | string  | "localhost:6060" |

Example:

```json
  {
    "profiling": {
      "enabled": false,
      "bindAddress": "localhost:6060"
    }
  }
```

## <a id="restapi"></a> 12. RestAPI

| Name                        | Description                                                                                    | Type    | Default value                                                                                                                                                                                                                                                                                                                                                                                                |
| --------------------------- | ---------------------------------------------------------------------------------------------- | ------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| enabled                     | Whether the REST API plugin is enabled                                                         | boolean | true                                                                                                                                                                                                                                                                                                                                                                                                         |
| bindAddress                 | The bind address on which the REST API listens on                                              | string  | "0.0.0.0:14265"                                                                                                                                                                                                                                                                                                                                                                                              |
| publicRoutes                | The HTTP REST routes which can be called without authorization. Wildcards using \* are allowed  | array   | /health<br/>/api/routes<br/>/api/core/v2/info<br/>/api/core/v2/tips<br/>/api/core/v2/blocks\*<br/>/api/core/v2/transactions\*<br/>/api/core/v2/milestones\*<br/>/api/core/v2/outputs\*<br/>/api/core/v2/treasury<br/>/api/core/v2/receipts\*<br/>/api/debug/v1/\*<br/>/api/indexer/v1/\*<br/>/api/mqtt/v1<br/>/api/participation/v1/events\*<br/>/api/participation/v1/outputs\*<br/>/api/participation/v1/addresses\* |
| protectedRoutes             | The HTTP REST routes which need to be called with authorization. Wildcards using \* are allowed | array   | /api/\*                                                                                                                                                                                                                                                                                                                                                                                                       |
| [jwtAuth](#restapi_jwtauth) | Configuration for JWT Auth                                                                     | object  |                                                                                                                                                                                                                                                                                                                                                                                                              |
| [pow](#restapi_pow)         | Configuration for Proof of Work                                                                | object  |                                                                                                                                                                                                                                                                                                                                                                                                              |
| [limits](#restapi_limits)   | Configuration for limits                                                                       | object  |                                                                                                                                                                                                                                                                                                                                                                                                              |

### <a id="restapi_jwtauth"></a> JWT Auth

| Name | Description                                                                                                                             | Type   | Default value |
| ---- | --------------------------------------------------------------------------------------------------------------------------------------- | ------ | ------------- |
| salt | Salt used inside the JWT tokens for the REST API. Change this to a different value to invalidate JWT tokens not matching this new value | string | "HORNET"      |

### <a id="restapi_pow"></a> Proof of Work

| Name        | Description                                                                | Type    | Default value |
| ----------- | -------------------------------------------------------------------------- | ------- | ------------- |
| enabled     | Whether the node does PoW if blocks are received via API                   | boolean | false         |
| workerCount | The amount of workers used for calculating PoW when issuing blocks via API | int     | 1             |

### <a id="restapi_limits"></a> Limits

| Name          | Description                                                               | Type   | Default value |
| ------------- | ------------------------------------------------------------------------- | ------ | ------------- |
| maxBodyLength | The maximum number of characters that the body of an API call may contain | string | "1M"          |
| maxResults    | The maximum number of results that may be returned by an endpoint         | int    | 1000          |

Example:

```json
  {
    "restAPI": {
      "enabled": true,
      "bindAddress": "0.0.0.0:14265",
      "publicRoutes": [
        "/health",
        "/api/routes",
        "/api/core/v2/info",
        "/api/core/v2/tips",
        "/api/core/v2/blocks*",
        "/api/core/v2/transactions*",
        "/api/core/v2/milestones*",
        "/api/core/v2/outputs*",
        "/api/core/v2/treasury",
        "/api/core/v2/receipts*",
        "/api/debug/v1/*",
        "/api/indexer/v1/*",
        "/api/mqtt/v1",
        "/api/participation/v1/events*",
        "/api/participation/v1/outputs*",
        "/api/participation/v1/addresses*"
      ],
      "protectedRoutes": [
        "/api/*"
      ],
      "jwtAuth": {
        "salt": "HORNET"
      },
      "pow": {
        "enabled": false,
        "workerCount": 1
      },
      "limits": {
        "maxBodyLength": "1M",
        "maxResults": 1000
      }
    }
  }
```

## <a id="warpsync"></a> 13. WarpSync

| Name             | Description                                        | Type    | Default value |
| ---------------- | -------------------------------------------------- | ------- | ------------- |
| enabled          | Whether the warpsync plugin is enabled             | boolean | true          |
| advancementRange | The used advancement range per warpsync checkpoint | int     | 150           |

Example:

```json
  {
    "warpsync": {
      "enabled": true,
      "advancementRange": 150
    }
  }
```

## <a id="tipsel"></a> 14. Tipselection

| Name                         | Description                                | Type    | Default value |
| ---------------------------- | ------------------------------------------ | ------- | ------------- |
| enabled                      | Whether the tipselection plugin is enabled | boolean | true          |
| [nonLazy](#tipsel_nonlazy)   | Configuration for nonLazy                  | object  |               |
| [semiLazy](#tipsel_semilazy) | Configuration for semiLazy                 | object  |               |

### <a id="tipsel_nonlazy"></a> NonLazy

| Name                    | Description                                                                                             | Type   | Default value |
| ----------------------- | ------------------------------------------------------------------------------------------------------- | ------ | ------------- |
| retentionRulesTipsLimit | The maximum number of current tips for which the retention rules are checked (non-lazy)                 | int    | 100           |
| maxReferencedTipAge     | The maximum time a tip remains in the tip pool after it was referenced by the first block (non-lazy)    | string | "3s"          |
| maxChildren             | The maximum amount of references by other blocks before the tip is removed from the tip pool (non-lazy) | uint   | 30            |

### <a id="tipsel_semilazy"></a> SemiLazy

| Name                    | Description                                                                                              | Type   | Default value |
| ----------------------- | -------------------------------------------------------------------------------------------------------- | ------ | ------------- |
| retentionRulesTipsLimit | The maximum number of current tips for which the retention rules are checked (semi-lazy)                 | int    | 20            |
| maxReferencedTipAge     | The maximum time a tip remains in the tip pool after it was referenced by the first block (semi-lazy)    | string | "3s"          |
| maxChildren             | The maximum amount of references by other blocks before the tip is removed from the tip pool (semi-lazy) | uint   | 2             |

Example:

```json
  {
    "tipsel": {
      "enabled": true,
      "nonLazy": {
        "retentionRulesTipsLimit": 100,
        "maxReferencedTipAge": "3s",
        "maxChildren": 30
      },
      "semiLazy": {
        "retentionRulesTipsLimit": 20,
        "maxReferencedTipAge": "3s",
        "maxChildren": 2
      }
    }
  }
```

## <a id="receipts"></a> 15. Receipts

| Name                             | Description                            | Type    | Default value |
| -------------------------------- | -------------------------------------- | ------- | ------------- |
| enabled                          | Whether the receipts plugin is enabled | boolean | true          |
| [backup](#receipts_backup)       | Configuration for backup               | object  |               |
| [validator](#receipts_validator) | Configuration for validator            | object  |               |

### <a id="receipts_backup"></a> Backup

| Name    | Description                                     | Type    | Default value |
| ------- | ----------------------------------------------- | ------- | ------------- |
| enabled | Whether to backup receipts in the backup folder | boolean | false         |
| path    | Path to the receipts backup folder              | string  | "receipts"    |

### <a id="receipts_validator"></a> Validator

| Name                                           | Description                                                       | Type    | Default value |
| ---------------------------------------------- | ----------------------------------------------------------------- | ------- | ------------- |
| validate                                       | Whether to validate receipts                                      | boolean | false         |
| ignoreSoftErrors                               | Whether to ignore soft errors and not panic if one is encountered | boolean | false         |
| [api](#receipts_validator_api)                 | Configuration for API                                             | object  |               |
| [coordinator](#receipts_validator_coordinator) | Configuration for coordinator                                     | object  |               |

### <a id="receipts_validator_api"></a> API

| Name    | Description                    | Type   | Default value            |
| ------- | ------------------------------ | ------ | ------------------------ |
| address | Address of the legacy node API | string | "http://localhost:14266" |
| timeout | Timeout of API calls           | string | "5s"                     |

### <a id="receipts_validator_coordinator"></a> Coordinator

| Name            | Description                                 | Type   | Default value                                                                       |
| --------------- | ------------------------------------------- | ------ | ----------------------------------------------------------------------------------- |
| address         | Address of the legacy coordinator           | string | "UDYXTZBE9GZGPM9SSQV9LTZNDLJIZMPUVVXYXFYVBLIEUHLSEWFTKZZLXYRHHWVQV9MNNX9KZC9D9UZWZ" |
| merkleTreeDepth | Depth of the Merkle tree of the coordinator | int    | 24                                                                                  |

Example:

```json
  {
    "receipts": {
      "enabled": true,
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
    }
  }
```

## <a id="prometheus"></a> 16. Prometheus

| Name                                                     | Description                                                  | Type    | Default value    |
| -------------------------------------------------------- | ------------------------------------------------------------ | ------- | ---------------- |
| enabled                                                  | Whether the prometheus plugin is enabled                     | boolean | false            |
| bindAddress                                              | The bind address on which the Prometheus exporter listens on | string  | "localhost:9311" |
| [fileServiceDiscovery](#prometheus_fileservicediscovery) | Configuration for fileServiceDiscovery                       | object  |                  |
| databaseMetrics                                          | Whether to include database metrics                          | boolean | true             |
| nodeMetrics                                              | Whether to include node metrics                              | boolean | true             |
| gossipMetrics                                            | Whether to include gossip metrics                            | boolean | true             |
| cachesMetrics                                            | Whether to include caches metrics                            | boolean | true             |
| restAPIMetrics                                           | Whether to include restAPI metrics                           | boolean | true             |
| inxMetrics                                               | Whether to include INX metrics                               | boolean | true             |
| migrationMetrics                                         | Whether to include migration metrics                         | boolean | true             |
| debugMetrics                                             | Whether to include debug metrics                             | boolean | false            |
| goMetrics                                                | Whether to include go metrics                                | boolean | false            |
| processMetrics                                           | Whether to include process metrics                           | boolean | false            |
| promhttpMetrics                                          | Whether to include promhttp metrics                          | boolean | false            |

### <a id="prometheus_fileservicediscovery"></a> FileServiceDiscovery

| Name    | Description                                                 | Type    | Default value    |
| ------- | ----------------------------------------------------------- | ------- | ---------------- |
| enabled | Whether the plugin should write a Prometheus 'file SD' file | boolean | false            |
| path    | The path where to write the 'file SD' file to               | string  | "target.json"    |
| target  | The target to write into the 'file SD' file                 | string  | "localhost:9311" |

Example:

```json
  {
    "prometheus": {
      "enabled": false,
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
      "inxMetrics": true,
      "migrationMetrics": true,
      "debugMetrics": false,
      "goMetrics": false,
      "processMetrics": false,
      "promhttpMetrics": false
    }
  }
```

## <a id="inx"></a> 17. INX

| Name            | Description                                            | Type    | Default value    |
| --------------- | ------------------------------------------------------ | ------- | ---------------- |
| enabled         | Whether the INX plugin is enabled                      | boolean | false            |
| bindAddress     | The bind address on which the INX can be accessed from | string  | "localhost:9029" |
| [pow](#inx_pow) | Configuration for Proof of Work                        | object  |                  |

### <a id="inx_pow"></a> Proof of Work

| Name        | Description                                                                                                     | Type | Default value |
| ----------- | --------------------------------------------------------------------------------------------------------------- | ---- | ------------- |
| workerCount | The amount of workers used for calculating PoW when issuing blocks via INX. (use 0 to use the maximum possible) | int  | 0             |

Example:

```json
  {
    "inx": {
      "enabled": false,
      "bindAddress": "localhost:9029",
      "pow": {
        "workerCount": 0
      }
    }
  }
```

## <a id="debug"></a> 18. Debug

| Name    | Description                         | Type    | Default value |
| ------- | ----------------------------------- | ------- | ------------- |
| enabled | Whether the debug plugin is enabled | boolean | false         |

Example:

```json
  {
    "debug": {
      "enabled": false
    }
  }
```


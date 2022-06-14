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


# Core Configuration

![Hornet Node Configuration](/img/Banner/banner_hornet_configuration.png)

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

## <a id="app"></a> 1. Application

| Name            | Description                                                                                            | Type    | Default value |
| --------------- | ------------------------------------------------------------------------------------------------------ | ------- | ------------- |
| checkForUpdates | Whether to check for updates of the application or not                                                 | boolean | true          |
| disablePlugins  | A list of plugins that shall be disabled                                                               | array   |               |
| enablePlugins   | A list of plugins that shall be enabled                                                                | array   |               |
| stopGracePeriod | The maximum time to wait for background processes to finish during shutdown before terminating the app | string  | "5m"          |

Example:

```json
  {
    "app": {
      "checkForUpdates": true,
      "disablePlugins": [],
      "enablePlugins": [],
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

| Name                                         | Description                              | Type   | Default value     |
| -------------------------------------------- | ---------------------------------------- | ------ | ----------------- |
| [parameters](#protocol_parameters)           | Configuration for parameters             | object |                   |
| milestonePublicKeyCount                      | The amount of public keys in a milestone | int    | 2                 |
| [baseToken](#protocol_basetoken)             | Configuration for baseToken              | object |                   |
| [publicKeyRanges](#protocol_publickeyranges) | Configuration for publicKeyRanges        | array  | see example below |

### <a id="protocol_parameters"></a> Parameters

| Name            | Description                                                                                                      | Type   | Default value       |
| --------------- | ---------------------------------------------------------------------------------------------------------------- | ------ | ------------------- |
| version         | The protocol version this node supports                                                                          | uint   | 2                   |
| networkName     | The network ID on which this node operates on                                                                    | string | "chrysalis-mainnet" |
| bech32HRP       | The HRP which should be used for Bech32 addresses                                                                | string | "iota"              |
| minPoWScore     | The minimum PoW score required by the network                                                                    | float  | 4000.0              |
| belowMaxDepth   | The maximum allowed delta value for the OCRI of a given block in relation to the current CMI before it gets lazy | uint   | 15                  |
| vByteCost       | The vByte cost used for the storage deposit                                                                      | uint   | 500                 |
| vByteFactorData | The vByte factor used for data fields                                                                            | uint   | 1                   |
| vByteFactorKey  | The vByte factor used for key fields                                                                             | uint   | 10                  |
| tokenSupply     | The token supply of the native protocol token                                                                    | uint   | 2779530283277761    |

### <a id="protocol_basetoken"></a> BaseToken

| Name            | Description                           | Type    | Default value |
| --------------- | ------------------------------------- | ------- | ------------- |
| name            | The base token name                   | string  | "IOTA"        |
| tickerSymbol    | The base token ticker symbol          | string  | "MIOTA"       |
| unit            | The base token unit                   | string  | "i"           |
| subunit         | The base token subunit                | string  | ""            |
| decimals        | The base token amount of decimals     | uint    | 0             |
| useMetricPrefix | The base token uses the metric prefix | boolean | true          |

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
      "parameters": {
        "version": 2,
        "networkName": "chrysalis-mainnet",
        "bech32HRP": "iota",
        "minPoWScore": 4000,
        "belowMaxDepth": 15,
        "vByteCost": 500,
        "vByteFactorData": 1,
        "vByteFactorKey": 10,
        "tokenSupply": 2779530283277761
      },
      "milestonePublicKeyCount": 2,
      "baseToken": {
        "name": "IOTA",
        "tickerSymbol": "MIOTA",
        "unit": "i",
        "subunit": "",
        "decimals": 0,
        "useMetricPrefix": true
      },
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
        },
        {
          "key": "760d88e112c0fd210cf16a3dce3443ecf7e18c456c2fb9646cabb2e13e367569",
          "start": 1333460,
          "end": 2888660
        },
        {
          "key": "7bac2209b576ea2235539358c7df8ca4d2f2fc35a663c760449e65eba9f8a6e7",
          "start": 2108160,
          "end": 3359999
        },
        {
          "key": "edd9c639a719325e465346b84133bf94740b7d476dd87fc949c0e8df516f9954",
          "start": 2888660,
          "end": 3359999
        },
        {
          "key": "47a5098c696e0fb53e6339edac574be4172cb4701a8210c2ae7469b536fd2c59",
          "start": 3360000,
          "end": 0
        },
        {
          "key": "ae4e03072b4869e87dd4cd59315291a034493a8c202b43b257f9c07bc86a2f3e",
          "start": 3360000,
          "end": 0
        }
      ]
    }
  }
```

## <a id="db"></a> 4. Database

| Name             | Description                                                                         | Type    | Default value |
| ---------------- | ----------------------------------------------------------------------------------- | ------- | ------------- |
| engine           | The used database engine (pebble/rocksdb/mapdb)                                     | string  | "rocksdb"     |
| path             | The path to the database folder                                                     | string  | "mainnetdb"   |
| autoRevalidation | Whether to automatically start revalidation on startup if the database is corrupted | boolean | false         |

Example:

```json
  {
    "db": {
      "engine": "rocksdb",
      "path": "mainnetdb",
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

| Name                                        | Description                                                        | Type   | Default value                               |
| ------------------------------------------- | ------------------------------------------------------------------ | ------ | ------------------------------------------- |
| bindMultiAddresses                          | The bind addresses for this node                                   | array  | /ip4/0.0.0.0/tcp/15600<br/>/ip6/::/tcp/15600 |
| [connectionManager](#p2p_connectionmanager) | Configuration for connectionManager                                | object |                                             |
| identityPrivateKey                          | Private key used to derive the node identity (optional)            | string | ""                                          |
| [db](#p2p_db)                               | Configuration for Database                                         | object |                                             |
| reconnectInterval                           | The time to wait before trying to reconnect to a disconnected peer | string | "30s"                                       |
| [gossip](#p2p_gossip)                       | Configuration for gossip                                           | object |                                             |
| [autopeering](#p2p_autopeering)             | Configuration for autopeering                                      | object |                                             |

### <a id="p2p_connectionmanager"></a> ConnectionManager

| Name          | Description                                                                  | Type | Default value |
| ------------- | ---------------------------------------------------------------------------- | ---- | ------------- |
| highWatermark | The threshold up on which connections count truncates to the lower watermark | int  | 10            |
| lowWatermark  | The minimum connections count to hold after the high watermark was reached   | int  | 5             |

### <a id="p2p_db"></a> Database

| Name | Description                  | Type   | Default value |
| ---- | ---------------------------- | ------ | ------------- |
| path | The path to the p2p database | string | "p2pstore"    |

### <a id="p2p_gossip"></a> Gossip

| Name               | Description                                                                    | Type   | Default value |
| ------------------ | ------------------------------------------------------------------------------ | ------ | ------------- |
| unknownPeersLimit  | Maximum amount of unknown peers a gossip protocol connection is established to | int    | 4             |
| streamReadTimeout  | The read timeout for reads from the gossip stream                              | string | "1m"          |
| streamWriteTimeout | The write timeout for writes to the gossip stream                              | string | "10s"         |

### <a id="p2p_autopeering"></a> Autopeering

| Name                 | Description                                                  | Type    | Default value   |
| -------------------- | ------------------------------------------------------------ | ------- | --------------- |
| bindAddress          | Bind address for autopeering                                 | string  | "0.0.0.0:14626" |
| entryNodes           | List of autopeering entry nodes to use                       | array   |                 |
| entryNodesPreferIPv6 | Defines if connecting over IPv6 is preferred for entry nodes | boolean | false           |
| runAsEntryNode       | Whether the node should act as an autopeering entry node     | boolean | false           |

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
        "path": "p2pstore"
      },
      "reconnectInterval": "30s",
      "gossip": {
        "unknownPeersLimit": 4,
        "streamReadTimeout": "1m",
        "streamWriteTimeout": "10s"
      },
      "autopeering": {
        "bindAddress": "0.0.0.0:14626",
        "entryNodes": [],
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
| fullPath                                | Path to the full snapshot file                                                                                                                                        | string | "snapshots/mainnet/full_snapshot.bin"  |
| deltaPath                               | Path to the delta snapshot file                                                                                                                                       | string | "snapshots/mainnet/delta_snapshot.bin" |
| deltaSizeThresholdPercentage            | Create a full snapshot if the size of a delta snapshot reaches a certain percentage of the full snapshot (0.0 = always create delta snapshot to keep ms diff history) | float  | 50.0                                   |
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
      "fullPath": "snapshots/mainnet/full_snapshot.bin",
      "deltaPath": "snapshots/mainnet/delta_snapshot.bin",
      "deltaSizeThresholdPercentage": 50,
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

| Name        | Description                                       | Type   | Default value    |
| ----------- | ------------------------------------------------- | ------ | ---------------- |
| bindAddress | The bind address on which the profiler listens on | string | "localhost:6060" |

Example:

```json
  {
    "profiling": {
      "bindAddress": "localhost:6060"
    }
  }
```

## <a id="restapi"></a> 12. RestAPI

| Name                        | Description                                                                                    | Type   | Default value                                                                                                                                                                                                                                                                                                                                                                          |
| --------------------------- | ---------------------------------------------------------------------------------------------- | ------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| bindAddress                 | The bind address on which the REST API listens on                                              | string | "0.0.0.0:14265"                                                                                                                                                                                                                                                                                                                                                                        |
| publicRoutes                | The HTTP REST routes which can be called without authorization. Wildcards using * are allowed  | array  | /health<br/>/api/v2/info<br/>/api/v2/tips<br/>/api/v2/blocks*<br/>/api/v2/transactions*<br/>/api/v2/milestones*<br/>/api/v2/outputs*<br/>/api/v2/treasury<br/>/api/v2/receipts*<br/>/api/plugins/debug/v1/*<br/>/api/plugins/indexer/v1/*<br/>/api/plugins/mqtt/v1<br/>/api/plugins/participation/v1/events*<br/>/api/plugins/participation/v1/outputs*<br/>/api/plugins/participation/v1/addresses* |
| protectedRoutes             | The HTTP REST routes which need to be called with authorization. Wildcards using * are allowed | array  | /api/v2/*<br/>/api/plugins/*                                                                                                                                                                                                                                                                                                                                                            |
| [jwtAuth](#restapi_jwtauth) | Configuration for JWT Auth                                                                     | object |                                                                                                                                                                                                                                                                                                                                                                                        |
| [pow](#restapi_pow)         | Configuration for Proof of Work                                                                | object |                                                                                                                                                                                                                                                                                                                                                                                        |
| [limits](#restapi_limits)   | Configuration for limits                                                                       | object |                                                                                                                                                                                                                                                                                                                                                                                        |

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
      "bindAddress": "0.0.0.0:14265",
      "publicRoutes": [
        "/health",
        "/api/v2/info",
        "/api/v2/tips",
        "/api/v2/blocks*",
        "/api/v2/transactions*",
        "/api/v2/milestones*",
        "/api/v2/outputs*",
        "/api/v2/treasury",
        "/api/v2/receipts*",
        "/api/plugins/debug/v1/*",
        "/api/plugins/indexer/v1/*",
        "/api/plugins/mqtt/v1",
        "/api/plugins/participation/v1/events*",
        "/api/plugins/participation/v1/outputs*",
        "/api/plugins/participation/v1/addresses*"
      ],
      "protectedRoutes": [
        "/api/v2/*",
        "/api/plugins/*"
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

| Name             | Description                                        | Type | Default value |
| ---------------- | -------------------------------------------------- | ---- | ------------- |
| advancementRange | The used advancement range per warpsync checkpoint | int  | 150           |

Example:

```json
  {
    "warpsync": {
      "advancementRange": 150
    }
  }
```

## <a id="tipsel"></a> 14. Tipselection

| Name                         | Description                | Type   | Default value |
| ---------------------------- | -------------------------- | ------ | ------------- |
| [nonLazy](#tipsel_nonlazy)   | Configuration for nonLazy  | object |               |
| [semiLazy](#tipsel_semilazy) | Configuration for semiLazy | object |               |

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

## <a id="dashboard"></a> 15. Dashboard

| Name                    | Description                                                  | Type    | Default value    |
| ----------------------- | ------------------------------------------------------------ | ------- | ---------------- |
| bindAddress             | The bind address on which the dashboard can be accessed from | string  | "localhost:8081" |
| dev                     | Whether to run the dashboard in dev mode                     | boolean | false            |
| [auth](#dashboard_auth) | Configuration for auth                                       | object  |                  |

### <a id="dashboard_auth"></a> Auth

| Name           | Description                                           | Type   | Default value                                                      |
| -------------- | ----------------------------------------------------- | ------ | ------------------------------------------------------------------ |
| sessionTimeout | How long the auth session should last before expiring | string | "72h"                                                              |
| username       | The auth username (max 25 chars)                      | string | "admin"                                                            |
| passwordHash   | The auth password+salt as a scrypt hash               | string | "0000000000000000000000000000000000000000000000000000000000000000" |
| passwordSalt   | The auth salt used for hashing the password           | string | "0000000000000000000000000000000000000000000000000000000000000000" |

Example:

```json
  {
    "dashboard": {
      "bindAddress": "localhost:8081",
      "dev": false,
      "auth": {
        "sessionTimeout": "72h",
        "username": "admin",
        "passwordHash": "0000000000000000000000000000000000000000000000000000000000000000",
        "passwordSalt": "0000000000000000000000000000000000000000000000000000000000000000"
      }
    }
  }
```

## <a id="receipts"></a> 16. Receipts

| Name                             | Description                 | Type   | Default value |
| -------------------------------- | --------------------------- | ------ | ------------- |
| [backup](#receipts_backup)       | Configuration for backup    | object |               |
| [validator](#receipts_validator) | Configuration for validator | object |               |

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

## <a id="prometheus"></a> 17. Prometheus

| Name                                                     | Description                                                  | Type    | Default value    |
| -------------------------------------------------------- | ------------------------------------------------------------ | ------- | ---------------- |
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

## <a id="inx"></a> 18. INX

| Name            | Description                                            | Type   | Default value    |
| --------------- | ------------------------------------------------------ | ------ | ---------------- |
| bindAddress     | The bind address on which the INX can be accessed from | string | "localhost:9029" |
| [pow](#inx_pow) | Configuration for Proof of Work                        | object |                  |

### <a id="inx_pow"></a> Proof of Work

| Name        | Description                                                                                                     | Type | Default value |
| ----------- | --------------------------------------------------------------------------------------------------------------- | ---- | ------------- |
| workerCount | The amount of workers used for calculating PoW when issuing blocks via INX. (use 0 to use the maximum possible) | int  | 0             |

Example:

```json
  {
    "inx": {
      "bindAddress": "localhost:9029",
      "pow": {
        "workerCount": 0
      }
    }
  }
```

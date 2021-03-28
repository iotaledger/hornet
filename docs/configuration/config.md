# config.json

Hornet uses the JSON standard as config file. If you are unsure about some syntax have a look at the official specs [here](https://www.json.org).
The default config file is `config.json`. You can change the path or name of the config file by using the `-c` or `--config` flag. 
For Example: `hornet -c config_example.json`

## Table of content

- ["restAPI"](#1-rest_api)
  - [Basic auth](#basic-auth)
  - [Limits](#limits)
- ["dashboard"](#2-dashboard)
- ["db"](#3-db)
- ["snapshots"](#4-snapshots)
- ["pruning"](#5-pruning)
- ["node"](#6-node)
- ["p2p"](#7-p2p)
- ["p2pdisc"](#8-p2pdisc)
- ["logger"](#9-logger)
- ["spammer"](#10-spammer)
- ["mqtt"](#11-mqtt)
- ["profiling"](#12-profiling)
- ["prometheus"](#13-prometheus)


---

## 1. REST API

|            Name            |                                   Description                                   |       Type       |
| :------------------------: | :-----------------------------------------------------------------------------: | :--------------: |
|  [basicAuth](#basic-auth)  |                              config for basic auth                              |      object      |
|       permittedRoutes      | the allowed HTTP REST routes which can be called from non whitelisted addresses | array of strings |
|    whitelistedAddresses    |       the whitelist of addresses which are allowed to access the REST API       | array of strings |
|         bindAddress        |                the bind address on which the REST API listens on                |      string      |
|         powEnabled         |            whether the node does PoW if messages are received via API           |       bool       |
|       powWorkerCount       |   the amount of workers used for calculating PoW when issuing messages via API  |      integer     |
|      [limits](#limits)     |                              config for api limits                              |      object      |
| excludeHealthCheckFromAuth |                 whether to allow the health check route anyways                 |       bool       |

### Basic auth

|     Name     |                       Description                      |  Type  |
| :----------: | :----------------------------------------------------: | :----: |
|    enabled   |     whether to use HTTP basic auth for the REST API    |  bool  |
|   userName   |           the username of the HTTP basic auth          | string |
| passwordHash |   the HTTP basic auth password+salt as a scrypt hash   | string |
| passwordSalt | the HTTP basic auth salt used for hashing the password | string |

### Limits

|    Name    |                                Description                                |   Type  |
| :--------: | :-----------------------------------------------------------------------: | :-----: |
| bodyLength | the maximum number of characters that the body of an API call may contain |  string |
| maxResults |     the maximum number of results that may be returned by an endpoint     | integer |

Example:
```json
  "restAPI": {
    "basicAuth": {
      "enabled": false,
      "userName": "admin",
      "passwordHash": "0000000000000000000000000000000000000000000000000000000000000000",
      "passwordSalt": "0000000000000000000000000000000000000000000000000000000000000000"
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
  },
```

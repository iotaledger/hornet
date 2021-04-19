# Post-installation steps
Once the Hornet is deployed, all parameters are set via configuration files.

## Configuration
The most important ones are:
* `config.json`: include all configuration flags and their values
* `peering.json`: include all connection details to your static peers (neighbors)

Since the Hornet node software is able to power original IOTA 1.0 network as well as IOTA 1.5 (aka Chrysalis), it is important to use respective `config.json` file that targets the IOTA network that we want. All configuration files that targets respective networks are available in [source code repo](https://github.com/gohornet/hornet/tree/master) on GitHub.

Depending on installation path you selected, default config files may be also a part of the installation experience and so you may see the following configuration files at your deployment directory:
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

This behavior can be changed by running Hornet with some altering arguments. The default directory with configuration files can be changed by running `hornet --config-dir` argument.

Please see [config.json](./config.md) and [peering.json](./peering.md) chapters for more information regarding respective configuration files.

Once Hornet is executed, it outputs all loaded configuration parameters to `stdout` to be sure what configuration was loaded.

All other altering command line parameters can be obtained by running `hornet --help`.

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
There is an admin dashboard available in Hornet (on port 8081) and it is enabled by default.

However it is not listening to incoming requests from a public traffic to prevent access from a malicious actor. It is listening only to requests from `localhost` by default.

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
* please make sure a strong password is chosen before opened to a public traffic


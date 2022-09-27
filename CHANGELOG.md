# HORNET Changelog

All notable changes to this project will be documented in this file.

## [2.0.0-rc.2] - 27.09.2022

### Added
    - Added healthcheck to Dockerfile (#1773)

### Fixed
    - Do not pass cached objects to inx workers to avoid leaks (#1774)


## [2.0.0-rc.1] - 26.09.2022

### Release notes

- First version targeted for the Shimmer network.
- Implemented full Stardust protocol according to the [Tangle Improvement Proposals](https://github.com/iotaledger/tips).
- Introduced new `INX` (**I**OTA **N**ode e**X**tension) feature. This allows the core HORNET node to be extended with plugins written in any language supporting gRPC.
- Several plugins were extracted as INX extensions (Dashboard, MQTT, Coordinator, Participation, Faucet, Spammer).
- Removed `app.enablePlugins` and `app.disablePlugins` and replaced them with plugin-specific `enabled` settings.
- Config files now always need to be specified with `-c` flag (even `config.json`), otherwise default parameters are used.
- Private Tangle is now an easy-to-use docker compose setup.
- The ledger state is no longer checked per default at startup.
- Snapshots are now disabled by default.
- Added hornet-nest docker for developers.
- Added the possibility to log all REST API requests.
- Added per endpoint prometheus metrics for the REST API.
- Added network-bootstrap tool.
- Block tips are now only refreshed during remote PoW if no parents were given.
- Added mnemonic parameter to ed25519 key tool.
- Adapted ed25519-key tool to print mnemonic and derived keys using slip10.


## [1.2.1] - 29.04.2022

### Added
    - Added mnemonic parameter to ed25519 key tool (#1449) 

### Changed
    - Added `privateKey` to ed25519 key tool json output (#1449) 
 
### Fixed
    - Fixed high CPU usage due to missing check if the external PoW context got canceled (#1469)


## [1.2.0] - 13.04.2022

_**Note**: due to changes to the internal database structure it is recommended to remove the old `participation` database._

### Fixed
    - Fixed missing error returns in REST API (#1316)
    - Fixed various memory leaks that could crash the node with OOM when using autopeering or the spammer plugins (#1335, #1372)
    - Fixed several race conditions in the participation endpoints (#1349)
    - Fixed dangling open connection to peer which prevented reconnect (#1398)
    - Fixed version check to only check for updates in the same major version (#1382)
    - Fixed deadlock while shutting down the node (#1392)
    - Fixed request queue behavior and test cases (#1403)
    - Fixed temporary increased memory usage after snapshot creation (#1403)

### Changed
    - Updated to Go 1.18
    - Changed how the participation plugin calculates staking rewards. This improves the performance of the node when the plugin is enabled (#1350)
    - Changed the COO key validity indicator logic (#1373)
    - Adapted ed25519-key tool to print mnemonic and derived keys using slip10 (#1409)

### Added
    - Added support for multi-arch docker images (amd64+arm64) (#1240, #1301)
    - Added JSON output support for the tools and refactoring the tool cli-flag handling (#1289, #1300)
    - Added `transactionIdSpent` and `milestoneIndexSpent` to the outputs API endpoint (#1302)
    - Added various new tools to work with database backups (#1326, #1328, #1331, #1332, #1348, #1355)
    - Added new COO keys (#1375)
    - Added creating backups of the coordinator and migrator state files (#1381)
    - Added support for mapdb (in-memory) (#1306)

### Removed
    - Removed comnet configuration (#1374)

### Cleanups
    - Added traverser storage interfaces (#1319)
    - Cleanup cache object names and comments (#1323)
    - Update KVStore interface (#1391)
    - Update gommon and remove replace in go.mod (#1408) 

### Config file changes

`config.json`
```diff
  "protocol": {
    ...
    "publicKeyRanges": [
      ...
      {
        "key": "7bac2209b576ea2235539358c7df8ca4d2f2fc35a663c760449e65eba9f8a6e7",
        "start": 2108160,
-       "end": 3666260
+       "end": 3359999
      },
      {
        "key": "edd9c639a719325e465346b84133bf94740b7d476dd87fc949c0e8df516f9954",
        "start": 2888660,
-       "end": 4443860
+       "end": 3359999
+     },
+     {
+       "key": "47a5098c696e0fb53e6339edac574be4172cb4701a8210c2ae7469b536fd2c59",
+       "start": 3360000,
+       "end": 0
+     },
+     {
+       "key": "ae4e03072b4869e87dd4cd59315291a034493a8c202b43b257f9c07bc86a2f3e",
+       "start": 3360000,
+       "end": 0
      }
    ]
  },
```


## [1.1.3] - 29.12.2021

### Added
    - Add new db-health tool. (#1255)

### Config file changes

`config.json`
```diff
  "protocol": {
    ...
    "publicKeyRanges": [
      ...
      {
          "key": "7bac2209b576ea2235539358c7df8ca4d2f2fc35a663c760449e65eba9f8a6e7",
-         "start": 2111060,
+         "start": 2108160,
          "end": 3666260
      },
      ...
    ]
  },
```


## [1.1.2] - 22.12.2021

### Changed
    - Update tangle bay entry nodes (#1249)
    
### Fixed
    - Increase the maxWebsocketMessageSize to account for longer dashboard usernames. (#1251)

### Chore
    - Update dashboard (#1252)

### Config file changes

`config.json`
```diff
  "p2p": {
    "autopeering": {
      "entryNodes": [
        ...
-        "/dns/entry-mainnet.tanglebay.com/udp/14626/autopeering/iot4By1FD4pFLrGJ6AAe7YEeSu9RbW9xnPUmxMdQenC"
+        "/dns/entry-0.mainnet.tanglebay.com/udp/14626/autopeering/iot4By1FD4pFLrGJ6AAe7YEeSu9RbW9xnPUmxMdQenC",
+        "/dns/entry-1.mainnet.tanglebay.com/udp/14636/autopeering/CATsx21mFVvQQPXeDineGs9DDeKvoBBQdzcmR6ffCkVA"
      ],
    }
```


## [1.1.1] - 22.12.2021

### Changed
    - Added ulimit and stop_grace_period to docker-compose.yml and documentation (#1242)

### Fixed
    - Fixed WebSocket disconnecting on Safari browsers (#1243)
    - Fixed MQTT memory leak (#1246)

### Config file changes

`config.json`
```diff
    "prometheus": {
       ...
       "coordinatorMetrics": true,
+       "mqttBrokerMetrics": true,
       "debugMetrics": false,
       ...
    },
```


## [1.1.0] - 10.12.2021

### Added
    - Add participation plugin (#1204, #1207, #1208, #1209, #1210, #1212, #1215, #1218, #1221, #1231, #1232, #1233, 1234)
    - Add participation-cli tool (#1206, #1219)
    - Add rocksdb static binaries for macOS (#1192)
    - Add config_devnet.json (#1183)
    - Add snap-hash tool to calculate the ledger state hash of a snapshot (#1184)
    - Add db-hash tool to calculate the ledger state hash of a database (#1184)
    - Add coo-fix-state tool (#1185)
    - Add p2pstore to docker docs (#1177)

### Changed
    - Refactor the JWT auth for the API (#1191)
    - Separate UTXO database (#1201, #1205)
    - Only accept bech32 addresses with the correct prefix in the rest API (#1197)
    - Use target milestone timestamp for snapshots timestamps (#1184)
    - Expose the enabled HTTP plugins (Faucet, Debug, Participation) as features in the info endpoint (#1208)
    - Change MQTT topic subscription log to debug level (#1195)
    - Add JSON output to some of the tools (#1199)

### Fixed
    - Optimize RocksDB level compaction (#1223)
    - Improve search missing milestones (#1196) 
    - Ignore autopeering peers in unknownPeersLimit (#1227)
    - Include autopeered and unknown peers in connected count of heartbeat messages (#1179)
    - Fix/faucet conflicting tx (#1222, #1235)
    - Fix warpsync milestone deadlock (re-verify known milestone payloads) (#1186)
    - Use integer instead of strings for ulimits in docker-compose file
    - Fix mqtt port in private tangle scripts (#1186)

### Removed
    - Remove config_chrysalis_testnet.json (#1183)
    - Remove powsrv.io integration (#1229) 

### Chore
    - Updated dependencies and Go 1.17 (#1193)
    - Updated cross compiler to latest version (#1224)
    - Updated deps to latest versions (#1192, #1217, #1182, #1225)

### Cleanups
    - Use contexts to cancel instead of signal channels (#1195, #1198)
    - Rename Snapshot to SnapshotManager (#1184)
    - Rename UTXO to to UTXOManager (#1184)
    - Rename Manager to to PeeringManager (#1184)
    - Rename Service to GossipService (#1184)
    - Move autopeering logic to AutopeeringManager (#1178)
    - Move sync status logic to SyncManager (#1184)
    - Move DatabaseSize function from storage to database  (#1184)
    - Move Milestone validation logic to MilestoneManager (#1184)
    - Refactor snapshot package (#1184)
    - Add WrappedLogger (#1228)

### Workflows
    - Run snyk test in a schedule instead on every PR (#1200)
    - Updated CodeQL workflow according to generator from github (#1194)
    - Enable Dependabot (#1194)
    - Added twitter bot notification on release (#1216)

### Config file changes

`config.json`
```diff
    "jwtAuth": {
-      "enabled": false,
      "salt": "HORNET"
    },
-    "excludeHealthCheckFromAuth": false,
-    "permittedRoutes": [
+    "publicRoutes": [
      "/health",
      "/mqtt",
      "/api/v1/info",
      "/api/v1/tips",
-      "/api/v1/messages/:messageID",
-      "/api/v1/messages/:messageID/metadata",
-      "/api/v1/messages/:messageID/raw",
-      "/api/v1/messages/:messageID/children",
-      "/api/v1/messages",
+      "/api/v1/messages*",
-      "/api/v1/transactions/:transactionID/included-message",
+      "/api/v1/transactions*",
-      "/api/v1/milestones/:milestoneIndex",
-      "/api/v1/milestones/:milestoneIndex/utxo-changes",
+      "/api/v1/milestones*",
-      "/api/v1/outputs/:outputID",
+      "/api/v1/outputs*",
-      "/api/v1/addresses/:address",
-      "/api/v1/addresses/:address/outputs",
-      "/api/v1/addresses/ed25519/:address",
-      "/api/v1/addresses/ed25519/:address/outputs",
+      "/api/v1/addresses*",
      "/api/v1/treasury"
+      "/api/v1/receipts*",
+      "/api/plugins/faucet/*",
+      "/api/plugins/participation/events*",
+      "/api/plugins/participation/outputs*",
+      "/api/plugins/participation/addresses*"
    ],
+    "protectedRoutes": [
+      "/api/v1/*",
+      "/api/plugins/*"
+    ],
-    "whitelistedAddresses": [
-      "127.0.0.1",
-      "::1"
-    ],
```


## [1.0.5] - 02.09.2021

### Added
    - Add faucet plugin
    - Add faucet website
    - Add database migration tool
    - Add io utils to read/write a TOML file
    - Add private tangle scripts to deb/rpm files
    - New public keys applicable for milestone ranges between 1333460-4443860

### Changed
    - Move p2p identity private key to PEM file
    - Run autopeering entry node in standalone mode
    - Merge peerstore and autopeering db path in config
    - Expose autopeering port in dockerfile
    - Add detailed reason for snapshot download failure
    - Print bech32 address in toolset
    - Use kvstore for libp2p peerstore and autopeering database
    - Use internal bech32 HRP instead of additional config parameter
    - Update go version in workflows and dockerfile
    - Update github workflow actions
    - Update go modules
    - Update libp2p
    - Autopeering entry nodes for mainnet
    - Snapshot download sources for mainnet/comnet

### Fixed
    - Fix data size units according to SI units and IEC 60027
    - Fix possible jwt token vulnerabilities
    - Fix nil pointer exception in optimalSnapshotType
    - Set missing config default values for mainnet
    - Do not disable plugins for the entry nodes in the integration tests
    - Fix dependency injection of global config pars if pluggables are disabled by adding a InitConfigPars stage
    - Do not panic in MilestoneRetrieverFromStorage
    - Fix logger init in test cases

### Removed
    - Remove support for boltdb

### Cleanups
    - Refactor node/pluggable logger interface
    - Check returned error at the initialization of new background workers
    - Fixed return values order of StreamSnapshotDataTo
    - Do not use unbuffered channels for signal.Notify
    - Change some variable names according to linter suggestions
    - Remove duplicate imports
    - Remove dead code
    - Cleanup toolset listTools func
    - Replace deprecated prometheus functions
    - Fix comments
    - Remove 'Get' from getter function names
    - Add error return value to SetConfirmedMilestoneIndex
    - Add error return value to SetSnapshotMilestone
    - Add error return value to database health functions
    - Add error return value to loadSnaphotInfo
    - Replace http.Get with client.Do and a context
    - Add missing checks for returned errors
    - Add linter ignore rules
    - Cleanup unused parameters and return values
    - Fix some printouts and move hornet logo to startup
    - Return error if database init fails
    - Reduce dependencies between plugins/core modules
    - Replace deprecated libp2p crypto Bytes function
    - Add error return value to loadSolidEntryPoints
    - Rename util files to utils
    - Move database events to package
    - Move graceful shutdown logic to package
    - Move autopeering local to pkg
    - Add error return value to authorization init functions

### Config file changes

`config.json`
```diff
  "snapshots": {
    "downloadURLs": [
      {
-        "full": "https://mainnet.tanglebay.com/ls/full_snapshot.bin",
+        "full": "https://cdn.tanglebay.com/snapshots/mainnet/full_snapshot.bin",
-        "delta": "https://mainnet.tanglebay.com/ls/delta_snapshot.bin"
+        "delta": "https://cdn.tanglebay.com/snapshots/mainnet/delta_snapshot.bin"
      }
    ]
  },
  "protocol": {
    "publicKeyRanges": [
      {
        "key": "ba6d07d1a1aea969e7e435f9f7d1b736ea9e0fcb8de400bf855dba7f2a57e947",
        "start": 552960,
        "end": 2108160
-      }
+      },
+      {
+        "key": "760d88e112c0fd210cf16a3dce3443ecf7e18c456c2fb9646cabb2e13e367569",
+        "start": 1333460,
+        "end": 2888660
+      },
+      {
+        "key": "7bac2209b576ea2235539358c7df8ca4d2f2fc35a663c760449e65eba9f8a6e7",
+        "start": 2111060,
+        "end": 3666260
+      },
+      {
+        "key": "edd9c639a719325e465346b84133bf94740b7d476dd87fc949c0e8df516f9954",
+        "start": 2888660,
+        "end": 4443860
+      }
    ]
  },
  "p2p": {
-    "identityPrivateKey": "",
-    "peerStore": {
-      "path": "./p2pstore"
-    }
+    "db": {
+      "path": "p2pstore"
+    },
    "autopeering": {
-      "db": {
-        "path": "./p2pstore"
-      },
      "entryNodes": [
-        "/dns/lucamoser.ch/udp/14926/autopeering/4H6WV54tB29u8xCcEaMGQMn37LFvM1ynNpp27TTXaqNM",
+        "/dns/lucamoser.ch/udp/14826/autopeering/4H6WV54tB29u8xCcEaMGQMn37LFvM1ynNpp27TTXaqNM",
+        "/dns/entry-hornet-0.h.chrysalis-mainnet.iotaledger.net/udp/14626/autopeering/iotaPHdAn7eueBnXtikZMwhfPXaeGJGXDt4RBuLuGgb",
+        "/dns/entry-hornet-1.h.chrysalis-mainnet.iotaledger.net/udp/14626/autopeering/iotaJJqMd5CQvv1A61coSQCYW9PNT1QKPs7xh2Qg5K2",
         "/dns/entry-mainnet.tanglebay.com/udp/14626/autopeering/iot4By1FD4pFLrGJ6AAe7YEeSu9RbW9xnPUmxMdQenC"
      ],
    }
  },
+  "faucet": {
+    "amount": 10000000,
+    "smallAmount": 1000000,
+    "maxAddressBalance": 20000000,
+    "maxOutputCount": 127,
+    "indexationMessage": "HORNET FAUCET",
+    "batchTimeout": "2s",
+    "powWorkerCount": 0,
+    "website": {
+      "bindAddress": "localhost:8091",
+      "enabled": true
+    },
  }
```


## [1.0.4] - 04.08.2021

### Added
    - Autopeering with two default entry nodes (disabled by default)
    - Default config for community network

### Changed
    - Reduces the default WarpSync advancement range back to 150 as the previous value was only a workaround.
    - Cleaned up parameter names and values in all config files
    - Rate limit for dashboard login changed from 1 request every 5 minutes to 5 requests every minute

### Config file changes

`config.json`
```diff
  "receipts": {
    "backup": {
-      "folder": "receipts"
+      "path": "receipts"
    },
  }
  "p2p": {
    "bindMultiAddresses": [
      "/ip4/0.0.0.0/tcp/15600",
+      "/ip6/::/tcp/15600"
    ],
-    "gossipUnknownPeersLimit": 4,
+    "gossip": {
+      "unknownPeersLimit": 4,
+      "streamReadTimeout": "1m0s",
+      "streamWriteTimeout": "10s"
+    },
  "autopeering": {
-    "dirPath": "./p2pstore"
+    "db": {
+      "path": "./p2pstore"
+    },
  }
  "warpsync": {
-    "advancementRange": 10000
+    "advancementRange": 150
  },
  "spammer": {
-    "mpsRateLimit": 5.0,
+    "mpsRateLimit": 0.0,
  },
```


## [1.0.3] - 02.06.2021

### Added
    - A new public key applicable for milestone ranges between 552960-2108160

### Changed
    - Increases the WarpSync advancement range to 10k which allows nodes to resync even if
      they already stored the milestone for which they lacked the applicable public key beforehand.


## [1.0.2] - 28.05.2021

### Added
    - p2pidentity-extract tool (#1090)
    - tool to generate JWT token for REST API (#1085)
    - Add database pruning based on database size (#1115)

### Changed
    - Improved documentation (#1060 + #1083 + #1087 + #1090)
    - Build Dockerfile with rocksdb (#1077)
    - Default DB engine changed to rocksdb (#1078)
    - Renamed alphanet scripts to private_tangle (#1078)
    - Updated rocksdb (#1080)
    - Updated go modules, containers and dashboard (#1103)
    - Check if files exist in the p2pstore directory (needed for docker) (#1084)
    - Disabled MQTT http port (#1094)
    - Download the latest snapshots from the given targets (#1097)
    - Adds "ledgerIndex" field to some REST HTTP and MQTT API responses (#1106)
    - Add delta snapshots to control endpoint (#1039)
    - Changed node control endpoints to use POST (#1039)
    - Expose MQTT port. Remove no longer needed ports (#1105)
    - Re-add private Tangle doc (#1113)

### Fixed
    - Added workdir to docker/Dockerfile. (#1068)
    - JWT subject verification (#1076)
    - Send on closed channel in coo quorum (#1082)
    - Database revalidation (#1096)
    - Mask sensitive config parameters in log (#1100)
    - Fix ulimits and dependencies at node startup (#1107)
    - Do not print API JWT auth tokens to the log (unsafe) (#1039)
    - Check if node is busy before accepting snapshot commands via API (#1039)

### Config file changes

`config.json`
```diff
  "pruning": {
-    "enabled": true,
-    "delay": 60480,
+    "milestones": {
+      "enabled": false,
+      "maxMilestonesToKeep": 60480
+    },
+    "size": {
+      "enabled": true,
+      "targetSize": "30GB",
+      "thresholdPercentage": 10.0,
+      "cooldownTime": "5m"
+    },
    "pruneReceipts": false
  },
```


## [1.0.1] - 28.04.2021

### Fixed

    - Receipt validation deadlock
    - Docker Image Build Tags for RocksDB


## [1.0.0] - 28.04.2021

### Changed

    - IOTA: A new dawn.

# HORNET Changelog

All notable changes to this project will be documented in this file.

## [2.0.0-beta.3] - 26.07.2022

:warning: :warning: :warning:
This release contains a breaking change in the Rest API, so you need to update your clients to connect to this node version.
:warning: :warning: :warning:

### Changed
    - Updated iota.go with changes to the REST api models.

## [2.0.0-beta.2] - 21.07.2022

### Fixed
    - Updated iota.go with fixes for transaction validation


## [2.0.0-beta.1] - 18.07.2022

### Changed
    - Initial Stardust beta release.


## [2.0.0-alpha.25] - 13.07.2022

### Fixed
    - Fixed `final ledger index does not match target index` (#1620)

### Changed
    - Removed `app.enablePlugins` and `app.disablePlugins` and replaced them with plugin-specific `enabled` settings (#1617)

## [2.0.0-alpha.24] - 12.07.2022

### Added
    - Added Foundry, NativeTokens and NFT whiteflag tests (#1607)

### Fixed
    - Do not check storage deposits of full snapshot outputs (#1610)
    - Avoid locking of the p2p Manager while connection attempts are performed (#1614)

### Changed
    - Renamed `config.json` to `config_alphanet.json` and added `config_defaults.json` (#1613)
    - Renamed `-docker-example.tar.gz` release to `-docker.tar.gz` (#1615)
    - Block static peers in autopeering module (#1608)

### Chore
    -  Update modules (#1606)

## [2.0.0-alpha.23] - 07.07.2022

:warning: :warning: :warning: 
This release contains a breaking change in the snapshot format.
:warning: :warning: :warning: 

### Added
    - Add network-bootstrap tool (#1583)
    - Add pruning manager (#1591) 
    - Add snapshot importer (#1593) 
    - Storing whiteFlagIndex in the BlockMetadata for each referenced block (#1603)

### Changed
    - Move UTXO tests to own package (#1589)
    - Rename fields in Output and Spent types (#1590)
    - Move test util functions to tpkg (#1596)
    - Store protocol parameter updates for known tangle history (#1600) 
    - Reorganize documentation (#1597)
    - Update the Docker example README with a bit more information (#1592) 
    - Update snapshot format (#1594) 
    - Improve snapshot tools #1605 

### Cleanups
    - Replaced milestone.Index with iotago.MilestoneIndex (#1602) 

### Config file changes

`config.json`
```diff
  "protocol": {
+    "targetNetworkName": "alphanet-8",
-    "parameters": {
-      "version": 2,
-      "networkName": "alphanet-7",
-      "bech32HRP": "rms",
-      "minPoWScore": 1000.0,
-      "belowMaxDepth": 15,
-      "vByteCost": 500,
-      "vByteFactorData": 1,
-      "vByteFactorKey": 10,
-      "tokenSupply": 2779530283277761
-    },
    ...
  },
  ...
  "snapshots": {
    ...
     "deltaSizeThresholdPercentage": 50,
+    "deltaSizeThresholdMinSize": "50M",
    ...
  },
```


## [2.0.0-alpha.22] - 23.06.2022

### Fixed
    - Fixed a wrong minimum deposit calculation in iota.go after removing the milestone index from timelock and expiration unlock conditions.


## [2.0.0-alpha.21] - 21.06.2022

### Changed
    - Restructures REST API endpoints (#1577)
    - Removed milestone index from timelock and expiration unlock conditions (#1572)
    - Disable certain REST endpoints if an unsupported protocol upgrade is detected (#1571)
    - Changed the docker-example to only require 1 domain


## [2.0.0-alpha20] - 17.06.2022

### Fixed
    - Removes synchronous event handlers between peer manager to gossip service (#1565)

### Changed
    - Moved dashboard to INX (#1568, #1566)
    - Adds tip score updates stream to INX (#1563)


## [2.0.0-alpha19] - 14.06.2022

### Fixed
    - Fix INX deadlock #1560
    - Fix possible data races in INX #1561

### Changed
    - Adds supported protocol versions to REST API and INX (#1552)


## [2.0.0-alpha18] - 08.06.2022

### Fixed
    - Fix JWT skipping for indexer routes via dashboard API
    - Fix INX ReadBlockMetadata for solid entry points (#1534)
    - Send correct error message if a block is submitted without parents and no PoW is enabled (#1536)

### Changed
    - Rename files from message->block
    - Remove outdated routes
    - Move dashboard repository to iotaledger
    - Update go modules


## [2.0.0-alpha17] - 01.06.2022

### Fixed
    - Fixed libp2p connection issue (#1533)


## [2.0.0-alpha16] - 31.05.2022

### Fixed
    - Removed children API calls from dashboard
    - Fixed inx-spammer not working in the docker-example


## [2.0.0-alpha15] - 31.05.2022

### Changed
    - Migrated repositories to the iotaledger organization (#1528)
    - Update INX modules (#1530) 
    - Updated dashboard (#1527)
    - Updated protocol parameters data types (#1529)


## [2.0.0-alpha14] - 24.05.2022

### Changed
    - Update INX modules (#1516) 
    - Remove spammer plugin (#1515)
    - Feat/inx improvements (#1514)
    - Extended whiteflag MerkleHasher to do proofs (#1513)


## [2.0.0-alpha13] - 19.05.2022

### Changed
    - Decrease private tangle milestone interval to 5s

### Fixed
    - Fix snapshot SEP producer deadlock
    - Fix inx-participation routes

### Documentation
    - Change `messages` to `blocks` in documentation


## [2.0.0-alpha12] - 19.05.2022

### Changed
    - Don't mount profiles.json in docker-compose.yml
    - Output metadata & iota.go updates (#1494) 
    - Move several packages to hive.go (#1495)
    - Do not load messages on pruning (#1499)
    - Remove participation plugin (#1501)
    - Add inx-participation (#1502)
    - Add reflection based config (#1506)
    - Rename Messages to Blocks (#1508) 
    - Use iotago.BlockID instead of hornet.BlockID (#1509) 
    - Add INX milestone cone functions (#1511)
    - Code cleanup (#1510)
    - Moved INX examples to iotaledger/inx repository

### Documentation
    - Removed APT page (#1490)
    - Updated API reference links (#1491)


## [2.0.0-alpha11] - 03.05.2022

### Changed
    - Added remote PoW metrics to Prometheus. (#1481)
    - Updated the whiteflag conflict reasons. (#1484, #1485)


## [2.0.0-alpha10] - 29.04.2022

### Changed
    - AliasID and NFTID are now 32 bytes long instead of 20. (#1475)
    - Prometheus is not exposing prometheus metrics from other INX plugins anymore. (#1475)
    - TransactionEssence networkID check is now done during syntactical validation. (#1475)

### Fixed
    - Check if the external PoW context got cancelled. (#1470)
    - Fixed dashboard milestone topic not sending the latest milestones. (#1474)
    - Always allow remote PoW over INX. (#1475)
    - Do not fully initialize the node (including the Participation database) if the snapshot download and import fails. (#1475)


## [2.0.0-alpha9] - 27.04.2022

### Changed
    - Adapt storage layer to new milestone ID based logic. (#1454)
    - Handle SolidEntryPoints in solidification logic. (#1454)
    - Enforce milestone msg nonce zero in attacher. (#1454)
    - Add new milestone routes. (#1454)
    - Adapt node info endpoint to latest changes. (#1454)
    - Added BaseToken to config, restapi and INX. (#1462)
    - Add check to verify that milestone index and timestamp have increased. (#1463)

### Fixed
    - Only refresh tips on remote pow if no parents were given. (#1460) 
    - Fix database tool getMilestonePayloadViaAPI. (#1454)

### Cleanup
    - Chore/remove deprecated code. (#1461) 


## [2.0.0-alpha8] - 26.04.2022

### Changed
    - Extracted Faucet into a new INX module. (#1451)
    - Add own DbVersion for every database. (#1446)
    - Add mnemonic parameter to ed25519 key tool. (#1450)
    - Updated RocksDB to 7.1.2. (#1455)
    - Updated Milestone payloads according to TIP-29. (#1456)
    - Added new protocol parameters to info endpoint and INX. (#1456)

### Fixed
    - Adapt pruning and snapshotting to new milestone logic. (#1442)


## [2.0.0-alpha7] - 21.04.2022

### Changed
    - Milestone payloads now contain an optional metadata field according to TIP-29. (#1404)
    - Now using `rms` bech32 prefix according to TIP-31.
    - Private tangle improvements. (#1390, #1395)
    - Adapted ed25519-key tool to print mnemonic and derived keys using slip10. (#1407)
    - Removed coordinator plugin. (#1404)
    - Encode Messages and Outputs based on the given Accept Header MIME type. (#1427)

### Fixed
    - Fixed deadlock while shutting down the node. (#1394)
    - Fixed stale connections to peers when the initial stream establishment fails. (#1395)
    - Fixed request queue behavior and test cases. (#1412)
    - Fix wrong timestamp data type. (#1438)


## [2.0.0-alpha6] - 30.03.2022

### Changed
    - Removed built-in INX extensions support. Indexer and MQTT are now external dependencies. Use `docker-compose.yml` as a guide on how to setup your node.
    - Foundry output supply fields are now part of the token scheme.
    - Updated dashboard with an initial version supporting the new API routes.
    - Added various tools ported from mainnet branch.

### Fixed
    - Ported memory leak fixes from mainnet branch.


## [2.0.0-alpha5] - 17.03.2022

### Changed
    - Changed the snapshot format to simplify parsing for Bee.
    - Using default keepalive parameters for the Indexer to avoid disconnections due to too many pings.


## [2.0.0-alpha4] - 17.03.2022

### Changed
    - API now using hex-representation for bytes and uint256. Note: `0x` prefix is now required.
    - Adapted to latest `FoundryOutput` changes according to TIP-18.
    - Introduced new `INX` (IOTA Node Extension) feature. This allows the core HORNET node to be extended with plugins written in any language supporting gRPC.
    - Re-implemented Indexer and MQTT plugins as INX extensions. (Note: MQTT over WebSockets is now available under `/api/plugins/mqtt/v1` instead of `/mqtt`)
    - Implemented new MQTT topics acccording to TIP-28.
    - Added two new endpoints to the API `/api/v2/outputs/raw` to fetch the raw bytes of an output, and `/api/v2/outputs/metadata` to fetch metadata only.


## [2.0.0-alpha3] - 25.02.2022

### Changed
    - Everything, since we are all stardust. 


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

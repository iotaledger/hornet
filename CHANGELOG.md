# HORNET Changelog

All notable changes to this project will be documented in this file.

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
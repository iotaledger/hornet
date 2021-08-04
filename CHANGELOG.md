# HORNET Changelog

All notable changes to this project will be documented in this file.

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
    - p2pidentityextract tool (#1090)
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
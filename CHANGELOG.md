# HORNET Changelog

All notable changes to this project will be documented in this file.

## [0.5.6] - 15.11.2020

### Changed

    - Updated to Go 1.15.5 (security fixes)

### Fixed

    - Store milestone after it was found in serachMissingMilestone

## [0.5.5] - 13.11.2020

### Changed

    - traverse the cone of a solid milestone to reapply former milestones if they were missing in the database

### Fixed

    - update iota.go to fix finalizing of bundles

## [0.5.4] - 13.11.2020

### Changed

    - use iota.go/curl and iota.go/curl/bct instead of the much less performant hive.go versions
    - migrate batchhasher from hive.go into hornet
    - optimize batchhasher and add testcases
    - iota.go now ignores 243rd trit in signature fragments

### Fixed

    - coordinator startup race condition
    - improve tryte distribution of tools/rand-seed
    - do not change default GOMAXPROCS value
    - several CLI flag problems
    - delete invalid milestones (2272660, 2272661) that could not be solidified after COO crashed

## [0.5.3] - 14.09.2020

### Added

    - testsuite for easier test case creation
    - ported integration test framework from goshimmer

### Changed

    - added environment variables to /etc/default/hornet for easier node configuration
    - livefeed transactions are now filtered before they are rate limited (dashboard)
    - reduce websocket traffic by adding new message for sync status (dashboard)
    - config files can now be passed with file extension
    - autopeers are now ignored in "maxPeers" check
    - "tag"-Trytes are now padded in "findTransactions" webapi call
    - heartbeats are now used to detect dead peers
    - add write timeout to managed connection

### Fixed

    - connection to peers never closed on error ("waiting" state in dashboard)
    - incorrect alias of peers for incoming connections
    - pruning did not collect all transactions that were referenced by a milestone
    - deadlock if a peer connected during shutdown
    - "node is not synced" glitches in some webapi calls
    - race condition in autopeering shutdown
    - error not handled in SelectSpammerTips
    - pruning routine was exited if there was not enough history at the beginning
    - rpm package upgrade command
    - panic in websocket during connection establishment

## [0.5.2] - 01.09.2020

### Fixed

    - findTransactions was not working properly if not all search criteria were given

### Config file changes

`config.json`

```diff
  "snapshots": {
    "pruning": {
-      "delay": 15000
+      "delay": 60480
    }
  }
```

## [0.5.1] - 27.08.2020

### Added

    - getSpammerTips webapi call
    - webapi route to control the spammer plugin
    - autostart to the spammer config

### Changed

    - save memory in ApproversTraverser if walkAlreadyDiscovered is set
    - findTransaction now returns an intersection of the search criteria
    - show database size in GB if size > 1 GB (dashboard)
    - moved dashboard frontend code to [another git repo](https://github.com/iotaledger/node-dashboard)
    - spammer doesn't start automatically at node startup
    - getNodeInfo now returns the connected peers count
    - re-add logger settings to mainnet config file

### Fixed

    - warpsync milestone requesting
    - OOM bug in the future cone solidifier
    - map concurrent write/read panic in websocket
    - bundle creation race condition in coordinator
    - fix MQTT panic

### Config file changes

`config.json`

```diff
  "httpAPI": {
+    "permittedRoutes": [
+      "healthz"
+    ],
  }
+  "logger": {
+    "level": "info",
+    "disableCaller": true,
+    "encoding": "console",
+    "outputPaths": [
+      "stdout"
+    ]
+  },
  "spammer": {
-    "semiLazyTipsLimit": 30
+    "autostart": false
  },
```

## [0.5.0] - 19.08.2020

:warning: **Breaking change:** :warning:
Please update to this version as soon as the mainnet coordinator got upgraded.
Instructions on how to update to this version will be posted in due time.
The old HORNET versions won't be functional within the mainnet anymore!

### Added

    - White-Flag confirmation
    - Weighted uniform random tipselection for nodes
    - Adaptive heaviest branch tipselection for coordinator
    - Optional powsrv.io PoW support
    - LMI and neighbor counts to dashboard
    - getTipInfo API call
    - "isHealthy" to getNodeInfo
    - conf_trytes (confirmed trytes) ZMQ topic
    - conf_trytes (confirmed trytes) MQTT topic
    - Conflicting badge to the transaction explorer
    - Add autopeering rule to drop neighbors with LSMI below our pruning index
    - Automatic dashboard websocket reconnect
    - Database tainted flag for coordinator

### Changed

    - Request tx from all neighbors that could have the data
    - Bump protocol feature set for whiteflag (breaking protocol change)
    - Reduced dashboard traffic by introducing subscriptions to topics
    - Store binary trunk, branch, and bundle hashes in metadata to reduce load on IO
    - Improve caching strategy in solidification and confirmation
    - Improve caching in the dag helpers
    - Reduce recursion in the future cone solidifier
    - Use stack based DFS in TraverseApprovees
    - Use stack based BFS in TraverseApprovers
    - Return false for conflicting tx in the getInclusionStates web api call
    - Coordinator now waits until the milestone is solid
    - Set higher default verticesLimit in visualizer
    - Update to Go 1.15

### Fixed

    - Database revalidation
    - Missing byte to trytes conversion in some error messages
    - Make code more testable
    - Adding autopeered neighbors as static neighbors
    - CTPS calculation
    - Spikes in conf.rate calculation
    - Pruning
    - Do not drop autopeering and "acceptAny" peers on peering.json change
    - Index out of range in attachToTangle
    - Deadlock in snapshotting and pruning
    - Error reason for connection abort is now shown
    - Peering configs now recognized via CLI
    - Coordinator bootstrapping
    - Milestone missing / Milestone updated panics
    - Hash conflicts in the visualizer
    - SupportedFeatureSets logic in handshake
    - Websocket/dashboard deadlock
    - Dashboard visualizer re-rendered to often
    - Do not panic if snapshot creation is aborted
    - Fix solid entry point indexes

### Removed

    - Unused defaults from config.json
    - Graph plugin
    - Monitor plugin
    - Legacy gossip protocol
    - Genesis tx special case
    - RefsInvalidBundles cache

### Config file changes

Please use the new config.json and transfer values from your current config.json over to the new one, as a lot of keys have changed or got removed (instead of mutating your current one).

## [0.4.2] - 22.07.2020

### Added

    - Snapshot download fallback sources (#568)

### Changed

    - Using --version instead of --help for checking if the docker image works
    - Update autopeering entry nodes

### Fixed

    - Hiding of all non essential config flags
    - Ignoring all entry nodes if one is not found
    - Explicit --help/-h flag to print the help instead of using the built-in missing help flag error handling of pflag
    - Missing trytes convertion to ledger panic logs

### Config file changes

`config.json`

```diff
- "downloadURL": "https://ls.manapotion.io/export.bin"
+ "downloadURLs": [
+   "https://ls.manapotion.io/export.bin",
+   "https://x-vps.com/export.bin",
+   "https://dbfiles.iota.org/mainnet/hornet/latest-export.bin"
+ ]

  "entryNodes": [
-   "46CstniGgfWMdAySiWuS7bVfugwuHZCUQKVaC4Y34EYJ@enter.hornet.zone:14626",
+   "FvfwJuCMoWJvcJLSYww7whPxouZ9WFJ55uyxTxKxJ1ez@enter.hornet.zone:14626",
-   "2GHfjJhTqRaKCGBJJvS5RWty61XhjX7FtbVDhg7s8J1x@entrynode.tanglebay.org:14626",
-   "iotaMk9Rg8wWo1DDeG7fwV9iJ41hvkwFX8w6MyTQgDu@enter.thetangle.org:14627"
+   "iotaMk9Rg8wWo1DDeG7fwV9iJ41hvkwFX8w6MyTQgDu@enter.thetangle.org:14627",
+   "12w9FrzMdDQ42aBgFrv1siHuJMhuZ4SMVHRFSS7Zb72W@entrynode.iotatoken.nl:14626",
+   "DboTc1v61Xdyvggj8VRszy92ScUTLgfwZaHvXsU8zr7e@entrynode.tanglebay.org:14626"
  ],
```

`config_comnet.json`

```diff
- "downloadURL": "https://ls.tanglebay.org/comnet/export.bin"
+ "downloadURLs": [
+   "https://ls.manapotion.io/comnet/export.bin"
+ ]

  "entryNodes": [
-   "7Y1GSTTwJLMPCffNJhWggZPtwVce5hsgAVcHanNa6HXh@entrynode.comnet.tanglebay.org:14636",
-   "FPE6kHwZhvw8g163faJwTaPzYePbYtaXhwpWxFKuJfEY@enter.comnet.hornet.zone:14627"
+   "GLZAWBGqvm6ZRT7jGMFAKyUJNPdvx4i5A1GPRZbGS6C9@enter.comnet.hornet.zone:14627",
+   "J1Hn5r9pS5FkLeYqXWstC2Zyjxj73grEWvjuene3qjM9@entrynode.comnet.tanglebay.org:14636"
  ],
```

`config_devnet.json`

```diff
- "downloadURL": "https://dbfiles.iota.org/devnet/hornet/latest-export.bin"
+ "downloadURLs": [
+   "https://dbfiles.iota.org/devnet/hornet/latest-export.bin"
+ ]

  "entryNodes": [
-   "iotaDvNxMP5EPQPbHNzMTZK5ipd4BGZfjZBomenmyk3@enter.devnet.thetangle.org:14637"
+   "iotaDvNxMP5EPQPbHNzMTZK5ipd4BGZfjZBomenmyk3@enter.devnet.thetangle.org:14637",
+   "BqXajrWBFGYcJduK7kxiSMW3hv9fXRLzt9jK7JZZPAzp@entrynode.devnet.tanglebay.org:14646"
  ],
```

## [0.4.1] - 30.06.2020

### Added

    - Config opts modifiable via CLI and env variables
    - Config setting for warpsync advancement range
    - Snapshots dir
    - Dockerfile to build a local dev image
    - Ability to let the Prometheus plugin create a 'file service discovery' file
    - Snapshot index and Pruning index to Prometheus
    - Flag to force load a global snapshot if db exists

### Changed

    - Comnet coo address
    - Make database revalidation abortable
    - Replace ComputeIfAbsent with Store to reduce IO pressure
    - Updated mqtt lib
    - Updated hive.go
    - Wait until all txs of coo bundles are processed in the storage layer
    - Use new merkle package from iota.go incl. "Shake" key derivation
    - Updated rpm package
    - Detach events
    - README
    - Bump to Go 1.14.4
    - Coo plugin milestone validation
    - Only check for pre-release updates if a pre-release version is running
    - Autopeering seed encoding to base58
    - Use tanglebay for comnet snapshots
    - Updated external libs
    - Increase shutdown time for big databases

### Fixed

    - Race condition in tryConstructBundle
    - Remove unused modules (Dashboard)
    - Missing tryte conversion
    - Ignored autopeering max peers
    - Dashboard issues
    - IsStaticallyPeered check
    - Missing ca-certificates in Docker files
    - Check to prevent pruning of genesis tx
    - Below max depth for SolidEntryPoints
    - Gossip bind address for IPv6
    - Synchronization with IRI nodes

### Config file changes

`config.json`

```diff
  "snapshots": {
      "local": {
-        "path": "export.bin",
+        "path": "snapshots/mainnet/export.bin",
  },
  "autopeering": {
        "entryNodes": [
-        "LehlDBPJ6kfcfLOK6kAU4nD7B/BdR7SJhai7yFCbCCM=@enter.hornet.zone:14626",
-        "zEiNuQMDfZ6F8QDisa1ndX32ykBTyYCxbtkO0vkaWd0=@enter.manapotion.io:18626",
-        "EsY+zvaselQYA33AVNzrYIGLplboIh4r8oO+vLKQAVM=@entrynode.tanglebay.org:14626"
+        "46CstniGgfWMdAySiWuS7bVfugwuHZCUQKVaC4Y34EYJ@enter.hornet.zone:14626",
+        "EkSLZ4uvSTED1x6KaGzqxoGxjbytt2rPVfbJk1LRLCGL@enter.manapotion.io:18626",
+        "2GHfjJhTqRaKCGBJJvS5RWty61XhjX7FtbVDhg7s8J1x@entrynode.tanglebay.org:14626",
+        "iotaMk9Rg8wWo1DDeG7fwV9iJ41hvkwFX8w6MyTQgDu@enter.thetangle.org:14627"
        ],
  },
+ "warpsync": {
+     "advancementRange": 200
+ },
```

`config_comnet.json`

```diff
  "snapshots": {
      "local": {
-        "path": "export_comnet.bin",
-        "downloadURL": "https://ls.manapotion.io/comnet/export.bin"
+        "path": "snapshots/comnet/export.bin",
+        "downloadURL": "https://ls.tanglebay.org/comnet/export.bin"
  },
  "coordinator": {
-    "address": "BODHQPXSMDNHBWVZHVATBAHQGZSKWQLXYZNOXMKNUCOZCPTWHHNFBBHFOEGPTWGGUVDJPZAYZIMXIIGVD",
+    "address": "YBWDHGHUEB9KSOPONTLTOSSKITIBE9MXPASCLREDNV9HEABYBPTHRQGWNJWQFSYAYZRDXXIOZHWBC9DWC",
  },
  "autopeering": {
      "entryNodes": [
-        "TANGLEleGqaMFFSTiyAV/vvdING/xuJNTDW16oCXZbo=@enter.comnet.thetangle.org:14641",
-        "YRdteHJeawDw6UMw22yePwiQYlc1CsrmWhVljzfc6uw=@entrynode.comnet.tanglebay.org:14636",
-        "1bU0uI+apA7YRna530e3SYfTDtUsobrLObt58pe5c5E=@enter.comnet.hornet.zone:14627"
+        "iotaCrvEWGfaeA1HutcULjD4uZnPhEnD5xNGfGs8vhe@enter.comnet.thetangle.org:14647",
+        "7Y1GSTTwJLMPCffNJhWggZPtwVce5hsgAVcHanNa6HXh@entrynode.comnet.tanglebay.org:14636",
+        "FPE6kHwZhvw8g163faJwTaPzYePbYtaXhwpWxFKuJfEY@enter.comnet.hornet.zone:14627"
      ],
  },
+ "warpsync": {
+   "advancementRange": 50
+ },
```

## [0.4.1-rc4] - 23.06.2020

### Changed

    - Use tanglebay for comnet snapshots
    - Updated external libs
    - Increase shutdown time for big databases

### Fixed

    - Slow sync
    - Pruning

### Config file changes

`config_comnet.json`

```diff
 "snapshots": {
-    "downloadURL": "https://ls.manapotion.io/comnet/export.bin"
+    "downloadURL": "https://ls.tanglebay.org/comnet/export.bin"
 }
```

## [0.4.1-rc3] - 19.06.2020

### Fixed

    - Entry node public keys in config files

### Config file changes

`config.json`

```diff
"entryNodes": [
-  "46CstniGgfWMdAySiWuS7bVfugwuHZCUQKVaC4Y34EYJ=@enter.hornet.zone:14626",
-  "EkSLZ4uvSTED1x6KaGzqxoGxjbytt2rPVfbJk1LRLCGL=@enter.manapotion.io:18626",
-  "2GHfjJhTqRaKCGBJJvS5RWty61XhjX7FtbVDhg7s8J1x=@entrynode.tanglebay.org:14626"
+  "46CstniGgfWMdAySiWuS7bVfugwuHZCUQKVaC4Y34EYJ@enter.hornet.zone:14626",
+  "EkSLZ4uvSTED1x6KaGzqxoGxjbytt2rPVfbJk1LRLCGL@enter.manapotion.io:18626",
+  "2GHfjJhTqRaKCGBJJvS5RWty61XhjX7FtbVDhg7s8J1x@entrynode.tanglebay.org:14626"
],
```

`config_comnet.json`

```diff
"entryNodes": [
-  "67it5aiegGwyLPSewfc2Bv42BvdRAdNjaGjf3VMhoG2u=@enter.comnet.thetangle.org:14641",
-  "7Y1GSTTwJLMPCffNJhWggZPtwVce5hsgAVcHanNa6HXh=@entrynode.comnet.tanglebay.org:14636",
-  "FPE6kHwZhvw8g163faJwTaPzYePbYtaXhwpWxFKuJfEY=@enter.comnet.hornet.zone:14627"
+  "67it5aiegGwyLPSewfc2Bv42BvdRAdNjaGjf3VMhoG2u@enter.comnet.thetangle.org:14641",
+  "7Y1GSTTwJLMPCffNJhWggZPtwVce5hsgAVcHanNa6HXh@entrynode.comnet.tanglebay.org:14636",
+  "FPE6kHwZhvw8g163faJwTaPzYePbYtaXhwpWxFKuJfEY@enter.comnet.hornet.zone:14627"
],
```

## [0.4.1-rc2] - 18.06.2020

### Added

    - Config setting for warpsync advancement range
    - Snapshot index and Pruning index to Prometheus
    - Flag to force load a global snapshot if db exists

### Changed

    - Coo plugin milestone validation
    - Only check for pre-release updates if a pre-release version is running
    - Autopeering seed encoding to base58

### Fixed

    - Missing snapshot dir
    - Node does not sync after restart
    - Check to prevent pruning of genesis tx
    - Below max depth for SolidEntryPoints
    - Gossip bind address for IPv6
    - Synchronization with IRI nodes

### Config file changes

`config.json`

```diff
+"warpsync": {
+  "advancementRange": 200
+},

"entryNodes": [
-  "LehlDBPJ6kfcfLOK6kAU4nD7B/BdR7SJhai7yFCbCCM=@enter.hornet.zone:14626",
-  "zEiNuQMDfZ6F8QDisa1ndX32ykBTyYCxbtkO0vkaWd0=@enter.manapotion.io:18626",
-  "EsY+zvaselQYA33AVNzrYIGLplboIh4r8oO+vLKQAVM=@entrynode.tanglebay.org:14626"
+  "46CstniGgfWMdAySiWuS7bVfugwuHZCUQKVaC4Y34EYJ=@enter.hornet.zone:14626",
+  "EkSLZ4uvSTED1x6KaGzqxoGxjbytt2rPVfbJk1LRLCGL=@enter.manapotion.io:18626",
+  "2GHfjJhTqRaKCGBJJvS5RWty61XhjX7FtbVDhg7s8J1x=@entrynode.tanglebay.org:14626"
],
```

`config_comnet.json`

```diff
+"warpsync": {
+  "advancementRange": 50
+},

"entryNodes": [
-  "TANGLEleGqaMFFSTiyAV/vvdING/xuJNTDW16oCXZbo=@enter.comnet.thetangle.org:14641",
-  "YRdteHJeawDw6UMw22yePwiQYlc1CsrmWhVljzfc6uw=@entrynode.comnet.tanglebay.org:14636",
-  "1bU0uI+apA7YRna530e3SYfTDtUsobrLObt58pe5c5E=@enter.comnet.hornet.zone:14627"
+  "67it5aiegGwyLPSewfc2Bv42BvdRAdNjaGjf3VMhoG2u=@enter.comnet.thetangle.org:14641",
+  "7Y1GSTTwJLMPCffNJhWggZPtwVce5hsgAVcHanNa6HXh=@entrynode.comnet.tanglebay.org:14636",
+  "FPE6kHwZhvw8g163faJwTaPzYePbYtaXhwpWxFKuJfEY=@enter.comnet.hornet.zone:14627"
],
```

## [0.4.1-rc1] - 12.06.2020

### Added

    - Config opts modifiable via CLI and env variables
    - Snapshots dir
    - Dockerfile to build a local dev image
    - Ability to let the Prometheus plugin create a 'file service discovery' file

### Changed

    - Comnet coo address
    - Make database revalidation abortable
    - Replace ComputeIfAbsent with Store to reduce IO pressure
    - Updated mqtt lib
    - Updated hive.go
    - Wait until all txs of coo bundles are processed in the storage layer
    - Use new merkle package from iota.go incl. "Shake" key derivation
    - Updated rpm package
    - Detach events
    - README
    - Bump to Go 1.14.4

### Fixed

    - Race condition in tryConstructBundle
    - Remove unused modules (Dashboard)
    - Missing tryte conversion
    - Ignored autopeering max peers
    - Dashboard issues
    - IsStaticallyPeered check
    - Missing ca-certificates in Docker files

### Config file changes

`config.json`

```diff
-      "path": "export.bin",
+      "path": "snapshots/mainnet/export.bin",
```

`config_comnet.json`

```diff
-      "path": "export_comnet.bin",
+      "path": "snapshots/comnet/export.bin",

"coordinator": {
-    "address": "BODHQPXSMDNHBWVZHVATBAHQGZSKWQLXYZNOXMKNUCOZCPTWHHNFBBHFOEGPTWGGUVDJPZAYZIMXIIGVD",
+    "address": "YBWDHGHUEB9KSOPONTLTOSSKITIBE9MXPASCLREDNV9HEABYBPTHRQGWNJWQFSYAYZRDXXIOZHWBC9DWC",
}
```

## [0.4.0] - 05.06.2020

### Added

    - Autopeering
    - Object storage (speed and memory improvement)
    - Warp synchronization (high speed syncing)
    - Coordinator plugin
    - Database re-validation after a crash
    - Add API IP whitelist
    - Additional neighbors stats
    - Dashboard:
      - `bundle not found` alert
      - `unknown Tx` alert
      - GitHub mark linking to github
      - Dark theme
      - Explorer JSON view
      - Explorer text view
      - `Tag` search
      - Show approvers in tx explorer
      - Copy transaction hash
      - Copy transaction raw trytes
      - CTPS graph
      - Tooltip for copy buttons
      - Responsive design
      - Visualizer (ported from GoShimmer)
      - Spam transactions graph
      - Show IOTA units
      - Value-tx only filter
      - Average metrics to confirmed milestones
      - Spam metrics
    - API:
      - `pruneDatabase` call
      - `getLedgerState` call
      - `getFundsOnSpentAddresses` call
      - Health check API route (`/healthz`)
    - Dockerfiles for arm64
    - Neighbor alias
    - Node alias (Dashboard and `getNodeInfo`)
    - Profiles configuration file
    - Check for missing snapshot info
    - Balance check on snapshot import
    - Toolset (Autopeering seed generator, Password SHA256 sum, Coo plugin tool)
    - Snapshot file download when no local snapshot is found
    - Set coordinator address in database
    - Default comnet settings
    - New zmq and mqtt topics (`lm` & `lsm`)
    - Flag to overwrite coo address at startup
    - Show download speed
    - Prometheus exporter plugin
    - Value spam mode (spammer plugin)

### Removed

    - `in-flight` neighbor pool
    - Socket.io in favor of hive.go websockethub
    - Auto snapshot download from nfpm service file
    - Wrong `omitempty` from json tags
    - `getSnapshot` API call
    - armhf support
    - Unnecessary trinary <--> binary conversions (speed improvement)

### Changed

    - Database layout
    - Ignore example neighbor
    - Improved RPM and DEB packages
    - Make config files optional
    - Refactored configuration options
    - Reintroduce spent addresses DB
    - Snapshot format
    - `tx_trytes` ZMQ and MQTT topic changed to `trytes`
    - Debian package structure
    - Do not broadcast known tx
    - Use new object storage interface
    - Refactors networking packages and plugins
    - Send integer values as integers in MQTT topics
    - Renamed packages to pkg
    - Improve solidifier
    - Local snapshots are always enabled now
    - Simplify node sync check
    - Do not start HORNET automatically during an initial installation with the DEB package
    - Milestone logic
    - Pruning logic
    - Database pressure reduced
    - Renamed `ZeroMQ` plugin to `ZMQ`
    - Graph explorer link is now configurable
    - Improved spammer plugin
    - Local snapshot doesn't write to database if triggered externally
    - API:
      - Handle `minWeightMagnitude` as an optional parameter
      - Renamed `createSnapshot` to `createSnapshotFile`
      - Improved error handling in `createSnapshotFile`
    - Set latest known milestone at startup
    - Abort ongoing PoW in spammer on shutdown
    - Reasonable values for config defaults
    - Increase tipselect maxDepth to 5

### Fixed

    - Allow all orders of txs in attachToTangle
    - API getNodeInfo features is `null`
    - Graph plugin
    - Monitor plugin
    - Missing comma in MQTT TX event
    - Missing folder in `.deb` package
    - Updated profiles for better RAM usage
    - ZMQ panics on greeting
    - Scheme for jquery url in monitor plugin
    - HTTP API basic auth
    - High memory usage
    - URL scheme in monitor and graph plugin
    - Local peer string character encoding
    - snapshot.csv reading
    - Heartbeats
    - ZMQ `address` topic
    - Security fixes

### Config file changes

Please use the new config.json and transfer values from your current config.json over to the new one, as a lot of keys have changed (instead of mutating your current one).

## [0.4.0-rc13] - 01.06.2020

### Changed

    - Removed unnecessary trinary <--> binary conversions (speed improvement)

### Fixed

    - File ownership (APT install)

## [0.4.0-rc12] - 29.05.2020

### Added

    - Prometheus exporter plugin
    - fsync calls at CloseDatabases
    - Dashboard:
      - Average metrics to confirmed milestones
      - Spam metrics
    - Value spam to spammer plugin

### Changed

    - Comnet coordinator address
    - Set latest known milestone at startup
    - Abort ongoing PoW in spammer on shutdown

### Fixed

    - Coordinator plugin milestone interval
    - Possible deadlock in pruning
    - Spammer:
      - Shutdown lock
      - High cpu usage if no limits given
      - High cpu usage if not synced but cpu below cpuMaxUsage
    - Pointer bug in coordinator and spammer
    - Wrong snapshot info EntryPointIndex

### Config file changes

Added options:

`config.json` and `config_comnet.json`

```diff
 "spammer": {
+   "bundleSize": 1,
+   "valueSpam": false
 }
+"prometheus": {
+   "bindAddress": "localhost:9311",
+   "goMetrics": false,
+   "processMetrics": false,
+   "promhttpMetrics": false
  }
```

Changed options:

`config_comnet.json`

```diff
 "coordinator": {
-  "address": "ZNCCPOTBCDZXCBQYBWUYYFO9PLRHNAROWOS9KGMYWNVIXWGYGUSJBZUTUQBNQRADHPUEONZZTYGVMSRZD",
+  "address": "BODHQPXSMDNHBWVZHVATBAHQGZSKWQLXYZNOXMKNUCOZCPTWHHNFBBHFOEGPTWGGUVDJPZAYZIMXIIGVD",
 }
```

## [0.4.0-rc11] - 26.05.2020

### Fixed

    - Pruning leading the node to crash due to a nil pointer dereference
    - Panic in revalidation
    - Fast DB size increase with enabled pruning
    - Legacy gossip
    - Incorrect update notification

## [0.4.0-rc10] - 21.05.2020

### Added

    - Show download speed

### Changed

    - Only print download progress every second
    - Use NoSync option to speed up boltdb
    - Confirm txs in visualizer by walking the past cone of milestone tail

### Fixed

    - Pruning of unconfirmed tx not verifying the milestoneIndex
    - Responsive Dashboard design
    - Do not block on visualizer websocket messages
    - Speed up revalidation and pruning
    - Abort snapshot download on daemon shutdown
    - Limit the search for transactions of a given address
    - Search for bundles was not possible in the dashboard

## [0.4.0-rc9] - 19.05.2020

**Breaking change:**<br>
Database implementation changed (moved from Badger to Bolt)<br><br>
_Update note:_ Please remove your database and restart HORNET.

### Added

    - Coordinator plugin
    - Dashboard:
      - Responsive design
      - Visualizer (ported from GoShimmer)
      - Spam transactions graph
      - Show IOTA units
      - Value-tx only filter
    - API:
      - `pruneDatabase` call
      - `getLedgerState` call
      - `getFundsOnSpentAddresses` call
    - Flag to overwrite coo address at startup

### Removed

    - `getSnapshot` API call

### Changed

    - Moved from Badger to Bolt (reduced RAM usage)
    - Milestone logic
    - Pruning logic
    - Database pressure reduced
    - Renamed `ZeroMQ` plugin to `ZMQ`
    - Dashboard graph colors
    - Graph explorer link is now configurable
    - Improved spammer plugin
    - Local snapshot doesn't write to database if triggered externally
    - API:
      - Handle `minWeightMagnitude` as an optional parameter
      - Renamed `createSnapshot` to `createSnapshotFile`
      - Improved error handling in `createSnapshotFile`

### Fixed

    - Database revalidation
    - Websocket messages
    - ZMQ `address` topic

### Config file changes

Added option:

`config.json`

```diff
+"coordinator": {
+  "address": "EQSAUZXULTTYZCLNJNTXQTQHOMOFZERHTCGTXOLTVAHKSA9OGAZDEKECURBRIXIJWNPFCQIOVFVVXJVD9",
+  "securityLevel": 2,
+  "merkleTreeDepth": 23,
+  "mwm": 14,
+  "stateFilePath": "coordinator.state",
+  "merkleTreeFilePath": "coordinator.tree",
+  "intervalSeconds": 60,
+  "checkpointTransactions": 5
+},
"spammer": {
+  "cpuMaxUsage": 0.5,
},
"graph": {
+  "explorerTxLink": "http://localhost:8081/explorer/tx/",
+  "explorerBundleLink": "http://localhost:8081/explorer/bundle/"
},
```

`config_comnet.json`

```diff
+"coordinator": {
+  "address": "ZNCCPOTBCDZXCBQYBWUYYFO9PLRHNAROWOS9KGMYWNVIXWGYGUSJBZUTUQBNQRADHPUEONZZTYGVMSRZD",
+  "securityLevel": 2,
+  "merkleTreeDepth": 23,
+  "mwm": 10,
+  "stateFilePath": "coordinator.state",
+  "merkleTreeFilePath": "coordinator.tree",
+  "intervalSeconds": 60,
+  "checkpointTransactions": 5
+},
"spammer": {
+  "cpuMaxUsage": 0.5,
},
"graph": {
+  "explorerTxLink": "http://localhost:8081/explorer/tx/",
+  "explorerBundleLink": "http://localhost:8081/explorer/bundle/"
},
```

Removed option:

`config.json` + `config_comnet.json`

```diff
-"milestones": {
-  "coordinator": "ZNCCPOTBCDZXCBQYBWUYYFO9PLRHNAROWOS9KGMYWNVIXWGYGUSJBZUTUQBNQRADHPUEONZZTYGVMSRZD",
-  "coordinatorSecurityLevel": 2,
-  "numberOfKeysInAMilestone": 23
-}
-"compass": {
-  "loadLSMIAsLMI": false
-},
-"protocol": {
-  "mwm": 14
-},
```

`config.json` + `config_comnet.json`

```diff
"spammer": {
-  "tpsRateLimit": 0.1,
+  "tpsRateLimit": 0.0,
-  "workers": 1
+  "workers": 0
}
"monitor": {
-  "initialTransactionsCount": 15000,
+  "initialTransactions": 15000,
}
```

## [0.4.0-rc8] - 06.04.2020

### Fixed

    - Warp sync not completing
    - Dashboard frontend dependencies

## [0.4.0-rc7] - 05.04.2020

### Added

    - Autopeering entry node health API (`/healthz`)
    - Debug webapi command `triggerSolidifier`

### Changed

    - Manually trigger solidifer from warp sync start if range already contains milestones
    - Do not start HORNET automatically during an initial installation with the DEB package
    - Badger (database) settings

## [0.4.0-rc6] - 03.04.2020

**Breaking change:**
Database version changed

### Added

    - Warp synchronization (high speed syncing)
    - Tooltip for copy buttons (dashboard)
    - Debug call `searchEntryPoints`

### Changed

    - Improve solidifier
    - Local snapshots are always enabled now
    - Database revalidation now reverts back to the last local snapshot (newer transactions are deleted)
    - Simplify node sync check
    - Use JSON view dark theme (dashboard)

### Fixed

    - Confirmation rate spikes in dashboard
    - Leak in replyToAllRequests
    - Update check panic
    - Heartbeats
    - Dashboard bugs
    - Disconnected peers are not deleted in some cases

### Config file changes

Added option:

`config_comnet.json`

```diff
"httpAPI": {
+  "excludeHealthCheckFromAuth": false
}
```

Removed option:

`config.json`

```diff
"snapshots": {
  "loadType": "local",
  "local": {
-   "enabled": true,
    "depth": 50,
    "intervalSynced": 50,
    "intervalUnsynced": 1000,
    "path": "export.bin",
    "downloadURL": "https://ls.manapotion.io/export.bin"
  },
```

`config_comnet.json`

```diff
"snapshots": {
  "loadType": "local",
  "local": {
-   "enabled": true,
    "depth": 50,
    "intervalSynced": 50,
    "intervalUnsynced": 1000,
    "path": "export.bin",
    "downloadURL": "https://ls.manapotion.io/export.bin"
  },
```

## [0.4.0-rc5] - 28.03.2020

### Changed

    - Send integer values as integers in MQTT topics
    - Renamed packages to pkg

### Fixed

    - Panics at concurrent write/iterations over the connected peers map
    - Atomic uint64 panics on ARM 32bit
    - Code inspection warnings
    - Wrong handling of IPv6 addresses

## [0.4.0-rc4] - 27.03.2020

### Added

    - Show approvers in tx explorer (dashboard)
    - Copy transaction hash (dashboard)
    - Copy transaction raw trytes (dashboard)
    - CTPS graph (dashboard)
    - Health check API route (`/healthz`)
    - New topics to zmq and mqtt (`lm` & `lsm`)

### Changed

    - Do not broadcast known tx
    - Use new object storage interface
    - Update to latest hive.go
    - Refactors networking packages and plugins
    - Changes default theme to dark (dashboard)

### Fixed

    - Database flush deadlock
    - Local snapshots
    - Panics at pruning if bundle was not complete

### Config file changes

New options:

`config.json`

```diff
 "httpAPI": {
+    "excludeHealthCheckFromAuth": false,
   },
```

Renamed config:<br>

`neighbors.json` --> `peering.json`

## [0.4.0-rc3] - 24.03.2020

### Added

    - Balance check on snapshot import
    - Toolset (Autopeering seed generator & Password SHA256 sum)
    - Snapshot file download when no local snapshot is found
    - Debug api call searchConfirmedApprovers
    - Set coordinator address in database
    - Default comnet settings
    - Snapshot download URLs for mainnet and comnet
    - Tanglebay autopeering entry nodes for mainnet and comnet
    - ARMv7 pre-build binary

### Removed

    - Auto snapshot download from nfpm service file
    - Wrong `omitempty` from json tags

### Changed

    - Debian package structure

### Fixed

    - Object storage deadlock
    - High memory usage
    - Revalidation OOM
    - URL scheme in monitor and graph plugin
    - Local peer string character encoding
    - snapshot.csv reading

### Config file changes

New options:

`config.json`

```diff
"snapshots": {
  "loadType": "local",
  "local": {
  "enabled": true,
  "depth": 50,
  "intervalSynced": 50,
  "intervalUnsynced": 1000,
  "path": "export.bin",
+ "downloadURL": "https://ls.manapotion.io/export.bin"
},
```

New config file:<br>
`config_comnet.json`

## [0.4.0-rc2] - 21.03.2020

### Added

    - Node alias (Dashboard and `getNodeInfo`)
    - Check for missing snapshot info

### Fixed

    - Deadlock between confirmation and snapshots
    - Snapshot limits
    - Scheme for jquery url in monitor plugin
    - Solidification trigger signal
    - HTTP API basic auth

### Config file changes

New options:

`config.json`

```diff
"node": {
+   "alias": "",
+   "showAliasInGetNodeInfo": false,
    "disablePlugins": [],
    "enablePlugins": []
  },
```

## [0.4.0-rc1] - 20.03.2020

### Added

    - Autopeering
    - Object storage (speed and memory improvement)
    - Database re-validation after a crash
    - Add API IP whitelist
    - Additional neighbors stats
    - Dashboard add `bundle not found` alert
    - Dashboard add `unknown Tx` alert
    - Dashboard add GitHub mark linking to github
    - Dashboard dark theme
    - Dashboard explorer JSON view
    - Dashboard explorer text view
    - Dashboard `Tag` search
    - Dockerfiles for armhf and arm64
    - Neighbor alias
    - Profiles configuration file

### Removed

    - `in-flight` neighbor pool
    - Socket.io in favor of hive.go websockethub

### Changed

    - Database layout
    - Ignore example neighbor
    - Improved RPM and DEB packages
    - Make config files optional
    - Refactored configuration options
    - Reintroduce spent addresses DB
    - Snapshot format
    - `tx_trytes` ZMQ and MQTT topic changed to `trytes`
    - Updated to Go 1.14.1
    - Updated to packr 2.8.0

### Fixed

    - Allow all orders of txs in attachToTangle
    - API getNodeInfo features is `null`
    - Graph plugin
    - Monitor plugin
    - Missing comma in MQTT TX event
    - Missing folder in `.deb` package
    - Updated profiles for better RAM usage
    - ZMQ panics on greeting

### Config file changes

Please use the new config.json and transfer values from your current config.json over to the new one, as a lot of keys have changed (instead of mutating your current one).

## [0.3.0] - 13.01.2020

### Added

    - Local Snapshots + database pruning
    - RPM and DEB packages
    - Spammer log messages
    - `neighbors.json` hot reload during runtime
      - Changes in the file are recognized and updated
      - Changes via webapi are stored to the file

### Removed

    - macOS binary

### Changed

    - Disable transactions load up during bundle eviction
    - Update to latest hive.go
    - Use Cuckoo filter instead of the spent addresses database
    - Statically link ARMv7 and ARM64 binaries
    - Removed "spent addresses" from database (breaking change)

### Fixed

    - Omit neighbor connection errors on shutdown
    - Broken `tx_trytes` MQTT JSON
    - getNeighbors address field always displays FQDN
    - Wrong inbound duplicate neighbor handling
    - Slow synching due to stalled requests in request queue

### Config file changes

New options:

`config.json`

```diff
+  "pruning": {
+    "enabled": true,
+    "delay": 40000
+  },
   "localsnapshots": {
+    "enabled": true,
+    "depth": 50,
+    "intervalsynced": 50,
+    "intervalunsynced": 1000,
     "path": "latest-export.gz.bin"
   },
+  "globalsnapshot": {
+    "load": false,
+    "path": "snapshotMainnet.txt",
+    "spentaddressespaths": ["previousEpochsSpentAddresses1.txt", "previousEpochsSpentAddresses2.txt", "previousEpochsSpentAddresses3.txt"],
+    "index": 1050000
+  },
+  "privatetangle": {
+    "ledgerstatepath": "balances.txt"
+  },
+  "logger": {
+    "level": "info",
+    "disableCaller": true,
+    "encoding": "console",
+    "outputPaths": [
+      "stdout"
+    ]
+  },
```

Removed options:

`config.json`

```diff
  "node": {
    "disableplugins": [],
    "enableplugins": [],
-   "loglevel": 127
  },
```

## [0.2.12] - 04.01.2020

### Fixed

    - Fixes broken ARM7 build with enabled CGO

## [0.2.11] - 04.01.2020

### Added

    - Seperate config file for neighbor settings
    - MQTT broker plugin
    - IOTA Tangle Visualiser plugin
    - Print HORNET version at startup
    - getLedgerDiffExt webapi call for debug purposes

### Removed

    - Almost all command line flags were removed (use the config file instead)
    - Removed "default" profile (use "auto" instead)

### Changed

    - Switched to hive.go packages to reduce codebase
    - Several speed improvements (binary/trinary conversion) due to latest iota.go version

### Fixed

    - Fixes possible panic with reattached milestones
    - Issue were milestoneSolidifierWorkerPool could block processing of tx
    - Fixes concurrent writes to the host blacklist
    - Fixes wrong order of bundles checks in solidifier

### Config file changes

New options:

`config.json`

```json
  "graph": {
    "webrootPath": "IOTAtangle/webroot",
    "socketiopath": "socket.io-client/dist/socket.io.js",
    "domain": "",
    "host": "127.0.0.1",
    "port": 8083,
    "networkName": "meets HORNET"
  },
  "mqtt": {
    "config": "mqtt_config.json"
  },
```

Now there is a seperate file for the neighbor settings:

`neighbors.json`

```json
{
  "autotetheringenabled": false,
  "maxneighbors": 5,
  "neighbors": [
    {
      "identity": "example1.neighbor.com:15600",
      "alias": "Example Neighbor 1",
      "prefer_ipv6": false
    },
    {
      "identity": "example2.neighbor.com:15600",
      "alias": "Example Neighbor 2",
      "prefer_ipv6": false
    },
    {
      "identity": "example3.neighbor.com:15600",
      "alias": "Example Neighbor 3",
      "prefer_ipv6": false
    }
  ]
}
```

Removed options:

`config.json`

```diff
  "network": {
    "address": "0.0.0.0",
-    "autotetheringenabled": false,
    "prefer_ipv6": false,
-    "maxneighbors": 5,
-    "neighbors": [
-      {
-        "identity": "example1.neighbor.com:15600",
-        "prefer_ipv6": false
-      },
-      {
-        "identity": "example2.neighbor.com:15600",
-        "prefer_ipv6": false
-      },
-      {
-        "identity": "example3.neighbor.com:15600",
-        "prefer_ipv6": false
-      }
-    ],
    "port": 15600,
    "reconnectattemptintervalseconds": 60
  },
```

## [0.2.10] - 27.12.2019

### Added

    - arm64 and armhv support to the Dockerfile

## [0.2.9] - 20.12.2019

### Fixed

    - `addNeighbors` deadlock
    - Message logger caused fatal panic

## [0.2.8] - 19.12.2019

### Added

    - Rate limiting for WebSocket sends
    - Show address balance even if no txs are available (Dashboard - Explorer)
    - Show spent state (Dashboard - Explorer)
    - Port configuration for Monitor plugin
    - Config to prefer IPv6 (addNeighbors)
    - Alternative addNeighbors command

### Changed

    - Release archives now contain a dir which wraps all files
    - API errors
    - TPS chart for better visibility of input and output (Dashboard)

### Fixed

    - Check wasSyncedBefore in ZMQ and Monitor
    - Wrong ZeroMQ `tx_trytes` response order
    - Deadlock if node is shut down during startup phase
    - Different TX order than IRI (attachToTangle)
    - Log level was ignored

### Config file changes

New options:

```json

  "network": {
    "prefer_ipv6": false,
  }

  "monitor": {
    "domain": "",
    "host": "127.0.0.1",
    "port": 4434,
    "apiPort": 4433
  }
```

**Changed option (you have to edit it in your config):**

```json
  "node": {
    "loglevel": 127
  }
```

## [0.2.7] - 17.12.2019

### Added

    - Version printout `--version`

### Changed

    - WorkerPools don't get flushed at shutdown by default
    - Import spent addresses in smaller batches
    - Faster syncing

### Fixed

    - RequestQueue never got empty if the cache overflowed
    - Several shutdown problems
    - Issue were only tail tx of a bundle got confirmed
    - Status report was still active during shutdown
    - Future cone solidifier got stuck, causing the node to become unsync

## [0.2.6] - 16.12.2019

### Changed

    - Faster initial spent addresses import

## [0.2.5] - 15.12.2019

### Added

    - More badger options in the profiles
    - "auto" profile chooses best setting based on available system memory

### Changed

    - "compactLevel0OnClose" is now disabled per default
    - Faster shutdown of the node

### Config file changes

New option:

```json
  "useProfile": "auto",
```

## [0.2.4] - 15.12.2019

This release fixes a CRITICAL bug! You have to delete your database folder.

### Fixed

    - Spent addresses were not imported from snapshot file.

## [0.2.3] - 15.12.2019

### Fixed

    - Close on closed channel in "ordered daemon" on shutdown

## [0.2.2] - 15.12.2019

### Added

    - TangleMonitor Plugin
    - Spammer Plugin
    - More detailed log messages at shutdown

### Fixed

    - Do not expose passwords from config file at startup
    - Duplicated neighbors

### Config file changes

New settings:

```json
  "monitor": {
    "tanglemonitorpath": "tanglemonitor/frontend",
    "domain": "",
    "host": "127.0.0.1"
  },
  "spammer": {
    "address": "HORNET99INTEGRATED99SPAMMER999999999999999999999999999999999999999999999999999999",
    "depth": 3,
    "message": "Spamming with HORNET tipselect",
    "tag": "HORNET99INTEGRATED99SPAMMER",
    "tpsratelimit": 0.1,
    "workers": 1
  },
  "zmq": {
    "host": "127.0.0.1",
  }
```

## [0.2.1] - 13.12.2019

### Added

    - Cache Metrics in SPA
    - Profiles to adjust cache sizes and DB opts

### Fixed

    - Remote PoW for Trinity

## [0.2.0] - 12.12.2019

### Added

    - DB version number
    - Configurable zmq host
    - Solidification timestamp of transactions
    - Docker files

### Changed

    - Database layout (breaking change)

### Fixed

    - Trinity compatibility
    - WebAPI CORS headers

## [0.1.0] - 11.12.2019

### Added

    - First beta release

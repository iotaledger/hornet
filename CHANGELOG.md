# HORNET Changelog

All notable changes to this project will be documented in this file.

## [2.0.0] - 03.10.2023

_**Warning**: This is a mandatory release for all nodes on IOTA mainnet._

### Release notes
- First "Stardust" version targeted for the IOTA mainnet.

### Chore
    - Update go version to 1.21 (#1912)

_**Note**: Removed binary and deb releases. Please switch to a Docker setup using [node-docker-setup](https://github.com/iotaledger/node-docker-setup)._

## [2.0.0-rc.8] - 11.08.2023

# Fixed
    - Fix missing treasury balance in db-hash tool (#1907)
    - Fix error while opening pebble databases "L0StopWritesThreshold"


## [2.0.0-rc.7] - 04.08.2023

# Added
    - Add path to RegisterAPIRoute (#1854) 

# Fixed
    - Port iota.go fixes from v2.0.0-rc.5 (#1853) 
    - Port iota.go fixes from v2.0.0-rc.6
    - Fixed windows build (#1856) 
    - Fix race condition in whiteflag API call (REST+INX) (#1894) 
    - Update modules to fix potential listener leak (#1868) 
    
### Chore
    - Update modules (#1843, #1848, #1895, #1902)
    - Adapt to app module changes in hive.go (#1867) 
    - Move core and plugins to components folder (#1869) 
    - Update inx-app


## [2.0.0-rc.6] - 22.05.2023

_**Warning**: This is a mandatory release for all nodes._

### Fixed
    - Updated iota.go with latest fixes


## [2.0.0-rc.5] - 07.03.2023

_**Warning**: This is a mandatory release for all nodes._

_**Note**: Removed binary and deb releases. Please switch to a Docker setup using [node-docker-setup](https://github.com/iotaledger/node-docker-setup)._

### Fixed
    - Updated iota.go with latest fixes
    - Check block before doing PoW in attacher (#1821)
    - Fix wrong key type in p2pidentity-gen tool (#1831)

### Added
    - Add resync phase to improve future cone solidification (#1832)
   

## [2.0.0-rc.4] - 09.01.2023

### Fixed
    - Fix syncing issue by preventing requests race condition (#1812)

### Added
    - Add /metadata endpoint to /included-block (#1807)

### Chore
    - Update modules (#1813)


## [2.0.0-rc.3] - 21.12.2022

### Fixed
    - Fixed JWT handling when using deeper regexes for INX plugins exposing APIs (#1802)

### Chore
    - Update docker setup documentation (#1775)
    - Remove docs version info (#1776)
    - Move database package to hive.go (#1785)
    - Move StoreHealthTracker to hive.go (#1787)
    - Move p2p identity related stuff to hive.go (#1792)
    - Enhance documentation sidebar with home (#1795)
    - Update using_docker.md (#1799) 
    - Updated dependencies (#1803)


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

# Private Tangle

This folder contains a Docker-based setup to run your own development private Tangle. The steps to run a private tangle
are:

## Requirements
1. A recent release of Docker enterprise or community edition. You can find installation instructions in the [official Docker documentation](https://docs.docker.com/engine/install/).
2. [Docker Compose CLI plugin](https://docs.docker.com/compose/install/compose-plugin/).

## Steps

1. `./bootstrap.sh` this will bootstrap your own private tangle by creating the genesis snapshot and required files.
   - _**Note:** If you are running this from inside the repository, you should run `./bootstrap.sh build` to re-build the docker images after any updates to the HORNET codebase (e.g. changing files or pulling git changes)_ 
2. Run:
   - `./run.sh` to run COO + 1 additional node.
   - `./run.sh 3` to run COO + 2 additional nodes.
   - `./run.sh 4` to run COO + 3 additional nodes.

3. `./cleanup.sh` to clean up all generated files and start over. 

The nodes will then be reachable under these ports:

- inx-faucet:
    - Faucet: http://localhost:8091
    - pprof: http://localhost:6073/debug/pprof
- Hornet-COO:
    - API: http://localhost:14265
    - External Peering: 15600/tcp
    - Dashboard: http://localhost:8081 (username: admin, password: admin)
    - Prometheus: http://localhost:9311/metrics
    - pprof: http://localhost:6060/debug/pprof
- Hornet-2:
    - API: http://localhost:14266
    - External Peering: 15601/tcp
    - Dashboard: http://localhost:8082 (username: admin, password: admin)
    - Prometheus: http://localhost:9312/metrics
    - pprof: http://localhost:6061/debug/pprof
- Hornet-3:
    - API: http://localhost:14267
    - External Peering: 15602/tcp
    - Dashboard: http://localhost:8083 (username: admin, password: admin)
    - Prometheus: http://localhost:9313/metrics
    - pprof: http://localhost:6062/debug/pprof
- Hornet-4:
    - API: http://localhost:14268
    - External Peering: 15603/tcp
    - Dashboard: http://localhost:8084 (username: admin, password: admin)
    - Prometheus: http://localhost:9314/metrics
    - pprof: http://localhost:6063/debug/pprof
- inx-coordinator:
    - pprof: http://localhost:6070/debug/pprof
- inx-indexer:
    - pprof: http://localhost:6071/debug/pprof
    - Prometheus: http://localhost:9321/metrics
- inx-mqtt:
    - pprof: http://localhost:6072/debug/pprof
    - Prometheus: http://localhost:9322/metrics
- inx-participation:
    - pprof: http://localhost:6074/debug/pprof
- inx-spammer:
    - pprof: http://localhost:6075/debug/pprof
    - Prometheus: http://localhost:9325/metrics
- inx-poi:
    - pprof: http://localhost:6076/debug/pprof
- inx-dashboard-1:
    - pprof: http://localhost:6080/debug/pprof
    - Prometheus: http://localhost:9330/metrics
- inx-dashboard-2:
    - pprof: http://localhost:6081/debug/pprof
    - Prometheus: http://localhost:9331/metrics
- inx-dashboard-3:
    - pprof: http://localhost:6082/debug/pprof
    - Prometheus: http://localhost:9332/metrics
- inx-dashboard-4:
    - pprof: http://localhost:6083/debug/pprof
    - Prometheus: http://localhost:9333/metrics

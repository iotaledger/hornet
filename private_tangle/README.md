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
   - `./run.sh` to run 2 nodes + COO.
   - `./run.sh 3` to run 3 nodes + COO.
   - `./run.sh 4` to run 4 nodes + COO.

3. `./cleanup.sh` to clean up all generated files and start over. 

The nodes will then be reachable under these ports:

- inx-faucet:
    - Faucet: http://localhost:8091
    - pprof: http://localhost:6024/debug/pprof
- Hornet-1:
    - API: http://localhost:14265
    - External Peering: 15611/tcp
    - Dashboard: http://localhost:8011 (username: admin, password: admin)
    - Prometheus: http://localhost:9311/metrics
    - pprof: http://localhost:6011/debug/pprof
    - inx: localhost:9011
- Hornet-2:
    - API: http://localhost:14266
    - External Peering: 15612/tcp
    - Dashboard: http://localhost:8012 (username: admin, password: admin)
    - Prometheus: http://localhost:9312/metrics
    - pprof: http://localhost:6012/debug/pprof
    - inx: localhost:9012
- Hornet-3:
    - API: http://localhost:14267
    - External Peering: 15613/tcp
    - Dashboard: http://localhost:8013 (username: admin, password: admin)
    - Prometheus: http://localhost:9313/metrics
    - pprof: http://localhost:6013/debug/pprof
    - inx: localhost:9013
- Hornet-4:
    - API: http://localhost:14268
    - External Peering: 15614/tcp
    - Dashboard: http://localhost:8014 (username: admin, password: admin)
    - Prometheus: http://localhost:9314/metrics
    - pprof: http://localhost:6014/debug/pprof
    - inx: localhost:9014
- inx-coordinator:
    - pprof: http://localhost:6021/debug/pprof
- inx-indexer:
    - pprof: http://localhost:6022/debug/pprof
    - Prometheus: http://localhost:9322/metrics
- inx-mqtt:
    - pprof: http://localhost:6023/debug/pprof
    - Prometheus: http://localhost:9323/metrics
- inx-participation:
    - pprof: http://localhost:6025/debug/pprof
- inx-spammer:
    - pprof: http://localhost:6026/debug/pprof
    - Prometheus: http://localhost:9326/metrics
- inx-poi:
    - pprof: http://localhost:6027/debug/pprof
- inx-dashboard-1:
    - pprof: http://localhost:6031/debug/pprof
    - Prometheus: http://localhost:9331/metrics
- inx-dashboard-2:
    - pprof: http://localhost:6032/debug/pprof
    - Prometheus: http://localhost:9332/metrics
- inx-dashboard-3:
    - pprof: http://localhost:6033/debug/pprof
    - Prometheus: http://localhost:9333/metrics
- inx-dashboard-4:
    - pprof: http://localhost:6034/debug/pprof
    - Prometheus: http://localhost:9334/metrics

## Start the coordinator in case of failure

The `inx-coordinator` container always starts together with the other containers if you execute the `./run.sh` command.
It may happen that the node startup takes longer than expected due to bigger databases or slow host machines. In that case the `inx-coordinator` container shuts down and won't be restarted automatically for security reasons.

If you want to restart the `inx-coordinator` separately, run the following command:
```sh
docker compose start inx-coordinator
```

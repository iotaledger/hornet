# HORNET in Docker

## Table of contents

- [Requirements](#requirements)
- [Quick Start](#quick-start)
  - [Clone Repository](#clone-repository)
  - [Prepare](#prepare)
  - [Docker Compose](#docker-compose)
  - [Build Image](#build-image)
  - [Run](#run)

## Requirements

1. A recent release of Docker enterprise or community edition.

2. git and curl

3. At least 1GB available RAM

## Quick Start

### Clone Repository

Clone the repository

```sh
git clone https://github.com/gohornet/hornet && cd hornet
```

The rest of the document assumes you are executing commands from the root directory of the repository.

### Prepare

1. Edit the `config.json` for alternative ports if needed.

2. Edit `peering.json` to your neighbors addresses.

3. The Docker image runs under user with user id 65532 and group id 65532. To make sure there are no permission issues, create the directory for the database, e.g.:

   ```sh
   sudo mkdir mainnetdb && sudo chown 65532:65532 mainnetdb
   ```

4. The Docker image runs under user with user id 65532 and group id 65532. To make sure there are no permission issues, create the directory for the snapshots, e.g.:

   ```sh
   sudo mkdir -p snapshots/mainnet && sudo chown 65532:65532 snapshots -R
   ```

### Docker Compose

Note: Follow this step only if you want to run HORNET via docker-compose.

If you are using an architecture other than amd64/x86_64 edit the `docker-compose.yml` file and set the correct architecture where noted.

The following command will build the image and run HORNET:

```sh
docker-compose up
```

CTRL-c to stop.

Add `-d` to run detached, and to stop:

```sh
docker-compose down -t 1200
```

### Build Image

If not running via docker-compose, build the image manually:

```sh
docker build -f docker/Dockerfile -t hornet:latest .
```

Or pull it from Docker hub (only available for amd64/x86_64):

```sh
docker pull gohornet/hornet:latest && docker tag gohornet/hornet:latest hornet:latest
```

### Run

Best is to run on host network for better performance (otherwise you are going to have to publish ports, that is done via iptables NAT and is slower)

```sh
docker run --rm \
  -v $(pwd)/config.json:/app/config.json:ro \
  -v $(pwd)/peering.json:/app/peering.json \
  -v $(pwd)/profiles.json:/app/profiles.json \
  -v $(pwd)/mainnetdb:/app/mainnetdb \
  -v $(pwd)/snapshots/mainnet:/app/snapshots/mainnet \
  --name hornet\
  --net=host \
  hornet:latest
```

Use CTRL-c to gracefully end the process.

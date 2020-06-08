# Hornet in Docker

_Table of contents_

<!--ts-->

- [Requirements](#requirements)
- [Quick Start](#quick-start)
  - [Clone Repository](#clone-repository)
  - [Prepare](#prepare)
  - [Docker Compose](#docker-compose)
  - [Build Image](#build-image)
  - [Run](#run)
- [Build Specific Version](#build-specific-version)
<!--te-->

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

i. Edit the `config.json` for alternative ports if needed.

ii. Edit `peering.json` to your neighbors addresses.

iii. The docker image runs under user with uid 39999. To make sure no permission issues, create the directory for the database, e.g.:

```sh
mkdir mainnetdb && chown 39999:39999 mainnetdb
```
iv. The docker image runs under user with uid 39999. To make sure no permission issues, create the directory for the snapshots, e.g.:
```sh
mkdir snapshots/mainnet && chown 39999:39999 snapshots -R
```

### Docker Compose

Note: Follow this step only if you want to run Hornet via docker-compose.

If you are using an architecture other than amd64/x86_64 edit the `docker-compose.yml` file and set the correct architecture where noted.

The following command will build the image and run Hornet:

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

Or pull it from dockerhub (only available for amd64/x86_64):

```sh
docker pull gohornet/hornet:latest && docker tag gohornet/hornet:latest hornet:latest
```

Note: for architectures other than amd64/x86_64 pass the corresponding Dockerfile, e.g.:

```sh
docker build -f docker/Dockerfile.arm64 -t hornet:latest .
```

### Run

Best is to run on host network for better performance (otherwise you are going to have to publish ports, that is done via iptables NAT and is slower)

```sh
docker run --rm -v $(pwd)/config.json:/app/config.json:ro -v $(pwd)/snapshots/mainnet:/app/snapshots/mainnet -v $(pwd)/mainnetdb:/app/mainnetdb --name hornet --net=host hornet:latest
```

Use CTRL-c to gracefully end the process.

## Build Specific Version

By default the Dockerfile builds the image using Hornet's latest version. To build an image with a specific version you can pass it via the build argument `TAG`, e.g.:

```sh
docker build -f docker/Dockerfile -t hornet:v0.3.0 --build-arg TAG=v0.3.0 .
```

# Hornet in Docker

*Table of contents*

<!--ts-->
   * [Requirements](#requirements)
   * [Quick Start](#quick-start)
     * [Clone Repository](#clone-repository)
     * [Prepare](#prepare)
     * [Docker Compose](#docker-compose)
     * [Build Image](#build-image)
     * [Run](#run)
   * [Local Snapshots](#local-snapshots)
   * [Build Specific Version](#build-specific-version)
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

### Prepare

i. Download the DB file
```sh
curl -LO https://dbfiles.iota.org/mainnet/hornet/latest-export.gz.bin
```

ii. Edit the `config.json` for alternative ports if needed.

iii. Edit `neighbors.json` to your neighbors addresses.

iv. The docker image runs under user with uid 39999. To make sure no permission issues, create the directory for the database, e.g.:
```sh
mkdir mainnetdb && chown 39999:39999 mainnetdb
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
docker run --rm -v $(pwd)/config.json:/app/config.json:ro -v $(pwd)/latest-export.gz.bin:/app/latest-export.gz.bin:ro -v $(pwd)/mainnetdb:/app/mainnetdb --name hornet --net=host hornet:latest
```
Use CTRL-c to gracefully end the process.

## Local Snapshots

Version `0.3.0` of Hornet introduced the local snapshots feature. To make sure it works well in Docker, follow these steps:

Before you begin, make sure hornet is stopped.

i. Create and download the latest-export to a separate directory:
```sh
mkdir -p snapshot\
  && curl -L -o snapshot/latest-export.gz.bin https://dbfiles.iota.org/mainnet/hornet/latest-export.gz.bin\
  && chown 39999:39999 snapshot -R
```

ii. Edit `config.json` and set the correct path. This can be done using `sed`:
```sh
sed -i 's#"path": "latest-export.gz.bin"#"path": "snapshot/latest-export.gz.bin"#' config.json
```

iii. To run docker you should mount the new `snapshot` directory with read-write mode. This allows Hornet to update the `latest-export.gz.bin` file:
```sh
docker run --rm -v $(pwd)/config.json:/app/config.json:ro -v $(pwd)/snapshot:/app/snapshot:rw -v $(pwd)/mainnetdb:/app/mainnetdb --name hornet --net=host hornet:latest
```

## Build Specific Version
By default the Dockerfile builds the image using Hornet's latest version. To build an image with a specific version you can pass it via the build argument `TAG`, e.g.:
```sh
docker build -f docker/Dockerfile -t hornet:v0.3.0 --build-arg TAG=v0.3.0 .
```

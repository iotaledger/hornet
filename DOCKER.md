# Quickstart

## Clone
To quickly build, first clone the repository

```sh
git clone https://github.com/gohornet/hornet && cd hornet
```

## Prepare

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
## Docker compose

If you are using an architecture different than amd64 edit the docker-compose.yml and set the correct architecture where noted.

For docker compose: this will build the image and run the process.
```sh
docker-compose up
```
CTRL-c to stop.

Add `-d` to run detached, and to stop:

```sh
docker-compose down
```

## Run build
If not running via docker-compose, build manually:

```sh
docker build -t hornet:latest .
```

Note: for aarch64/arm64 architecture pass the build argument:
```sh
docker build --build-arg ARCH=arm64 -t hornet:latest .
```
For 32 (armhf) pass `--build-arg ARCH=armhf`.


## Run

Best is to run on host network for better performance (otherwise you are going to have to publish ports, that is done via iptables NAT and is slower)
```sh
docker run --rm -v $(pwd)/config.json:/app/config.json:ro -v $(pwd)/latest-export.gz.bin:/app/latest-export.gz.bin:ro -v $(pwd)/mainnetdb:/app/mainnetdb --name hornet --net=host hornet:latest
```
Use CTRL-c to gracefully end the process.

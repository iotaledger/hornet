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

ii. Edit the `config.json` for neighbors and alternative ports if needed.

iii. The docker image runs under user with uid 39999. To make sure no permission issues, create the directory for the database, e.g.:
```sh
mkdir mainnetdb && chown 39999:39999 mainnetdb
```
## Docker compose

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

## Run

Best is to run on host network for better performance (otherwise you are going to have to publish ports, that is done via iptables NAT and is slower)
```sh
docker run --rm -v $(pwd)/config.json:/app/config.json:ro -v $(pwd)/latest-export.gz.bin:/app/latest-export.gz.bin:ro -v /tmp/db:/app/mainnetdb --name hornet --net=host hornet:latest
```
Use CTRL-c to gracefully end the process.

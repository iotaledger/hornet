# Quickstart

## Clone
To quickly build, first clone the repository

```sh
git clone https://github.com/gohornet/hornet && cd hornet
```

## Run build
```sh
docker build -t hornet:latest .
```

## Prepare

i. Download the DB file
```sh
curl -LO https://dbfiles.iota.org/mainnet/hornet/latest-export.gz.bin
```

ii. Create a directory for the database. You can choose any path (perferably on a partition with enough disk space):
```sh
mkdir /tmp/db
```
(Note: In the example above we're using /tmp that will get wiped after a reboot)

iii. Edit the `config.json` for neighbors and alternative ports if needed.


## Run

Best run on host network for better performance (otherwise you are going to have to publish ports, that is done via iptables NAT and is slower)
```sh
docker run --rm -v $(pwd)/config.json:/app/config.json:ro -v $(pwd)/latest-export.gz.bin:/app/latest-export.gz.bin:ro -v /tmp/db:/app/mainnetdb --name hornet --net=host hornet:latest
```
Use CTRL-c to gracefully end the process.

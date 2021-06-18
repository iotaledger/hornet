# Using Docker Compose

Make sure that you have read [Using Docker](using_docker.md) before continue reading. This chapter is for more advanced users. The following link provides more information about [Docker Compose]( https://docs.docker.com/compose/).

## Using Your Own Docker Compose File For Running Hornet

Docker Compose is a tool on top of the Docker engine that enables you to define Docker parameters in a structured way via `docker-compose.yaml` file. You can create and start the container with a single `docker-compose` command based on your configuration.

Create `docker-compose.yml` file among the other files created above:

```plaintext
.
├── config.json
├── peering.json
├── profiles.json
├── docker-compose.yml      <NEWLY ADDED FILE>
├── mainnetdb
└── snapshots
    └── mainnet
```

```plaintext
version: '3'
services:
  hornet:
    container_name: hornet
    image: gohornet/hornet:latest
    network_mode: host
    restart: always
    cap_drop:
      - ALL
    volumes:
      - ./config.json:/app/config.json:ro
      - ./peering.json:/app/peering.json
      - ./profiles.json:/app/profiles.json
      - ./snapshots/mainnet:/app/snapshots/mainnet
      - ./mainnetdb:/app/mainnetdb
```

Run the following command in the current directory to create and start an new Hornet container in detached mode (as daemon).

`docker-compose up -d`

See more details on how to configure Hornet under the [post installation](../post_installation/post_installation.md) chapter.

## Build Your Own Image Using Docker Compose

Note: Follow this step only if you want to run Hornet via Docker Compose.

If you are using an architecture other than amd64/x86_64 edit the `docker-compose.yml` file and set the correct architecture where noted.

The following command will build the image and run Hornet:

```sh
docker-compose up
```

CTRL-c to stop.

Add `-d` to run detached, and to stop:

```sh
docker-compose down -t 200
```

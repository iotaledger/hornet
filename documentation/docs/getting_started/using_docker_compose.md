---
description: Learn how to install and run a Hornet node using Docker Compose.
image: /img/logo/HornetLogo.png
keywords:
- IOTA Node
- Hornet Node
- Linux
- Install
- Docker Compose
- Build
- how to
---

# Using Docker Compose

Make sure that you have read [Using Docker](using_docker.md) before you continue reading as this article is for advanced users. You can find more information on Docker Compose in the [official Docker Compose documentation](https://docs.docker.com/compose/).

## Using Your Own Docker Compose File For Running Hornet

Docker Compose is a tool on top of the Docker engine that help you to define Docker parameters in a structured way using a `docker-compose.yaml` file. You can create and start the container with a single `docker-compose` command based on your configuration.

To do so, you will need to create `docker-compose.yml` in the same directory as described in the [Using Docker](https://wiki.iota.org/hornet/getting_started/using_docker) article:

```plaintext{5}
.
├── config.json
├── peering.json
├── profiles.json
├── docker-compose.yml      <NEWLY ADDED FILE>
├── mainnetdb
├── p2pstore
└── snapshots
    └── mainnet
```

The docker-compose.yml file should have the following content:

```plaintext
version: '3'
services:
  hornet:
    container_name: hornet
    image: gohornet/hornet:latest
    network_mode: host
    restart: always
    ulimits:
      nofile:
        soft: 8192
        hard: 8192
    stop_grace_period: 5m
    cap_drop:
      - ALL
    volumes:
      - ./config.json:/app/config.json:ro
      - ./peering.json:/app/peering.json
      - ./profiles.json:/app/profiles.json:ro
      - ./mainnetdb:/app/mainnetdb
      - ./p2pstore:/app/p2pstore
      - ./snapshots/mainnet:/app/snapshots/mainnet
```

You can run the following command in the current directory to create and start a new Hornet container in detached mode (as daemon):

`docker-compose up -d`

You can find more details on how to configure Hornet in the [post installation](https://wiki.iota.org/hornet/post_installation) section.

## Build Your Own Image Using Docker Compose

:::note

Follow this step only if you want to run Hornet via Docker Compose.

:::

If you are using any architecture other than `amd64/x86_64`, you should edit the `docker-compose.yml` file and set the correct architecture where noted.

You can run the following command to build the image and run Hornet:

```sh
docker-compose up
```

You can use `CTRL+C` to stop the container.

You can add `-d` to run detached.

To gracefully stop the container, you can run the following command:

```sh
docker-compose down
```

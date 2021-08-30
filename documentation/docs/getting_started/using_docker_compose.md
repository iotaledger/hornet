---
keywords:
- IOTA Node 
- Hornet Node
- Linux
- Install
- Docker Compose
- Build
description: Install and run a Hornet node using Docker Compose.
image: /img/logo/HornetLogo.png
---

# Using Docker Compose

Make sure that you have read [Using Docker](using_docker.md) before you continue reading as this chapter is for advanced users.  You can find more information on Docker Compose in the [official Docker Compose documentation](https://docs.docker.com/compose/).

## Using Your Own Docker Compose File For Running Hornet

Docker Compose is a tool on top of the Docker engine that enables you to define Docker parameters in a structured way using a `docker-compose.yaml` file. You can create and start the container with a single `docker-compose` command based on your configuration.

To do so, you will need to create `docker-compose.yml` in the same directory as described in the [Using Docker](using_docker.md)  section:

```plaintext{5}
.
├── config.json
├── peering.json
├── profiles.json
├── docker-compose.yml      <NEWLY ADDED FILE>
├── mainnetdb
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
    cap_drop:
      - ALL
    volumes:
      - ./config.json:/app/config.json:ro
      - ./peering.json:/app/peering.json
      - ./profiles.json:/app/profiles.json
      - ./snapshots/mainnet:/app/snapshots/mainnet
      - ./mainnetdb:/app/mainnetdb
```

You can run the following command in the current directory to create and start a new Hornet container in detached mode (as daemon):

`docker-compose up -d`

You can find more details on how to configure Hornet in the [post installation](../post_installation/post_installation.md) section.

## Build Your Own Image Using Docker Compose

:::info
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
docker-compose down -t 200
```

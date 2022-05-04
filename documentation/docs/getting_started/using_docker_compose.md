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

You can find more information on Docker Compose in the [official Docker Compose documentation](https://docs.docker.com/compose/).

## Requirements

1. A recent release of Docker compose or community edition. You can find installation instructions in the [official Docker documentation](https://docs.docker.com/compose/install/).
2. [GIT](https://git-scm.com/).
3. [CURL](https://curl.se/).
4. At least 1GB available RAM.

## Clone the Repository

Once you have completed all the installation [requirements](#requirements), you can clone the repository by running:

```sh
git clone https://github.com/gohornet/hornet && cd hornet && git checkout mainnet
```

:::note

The next portion of the guide assumes you are executing commands from the root directory of the repository.

:::

## Prepare

1. If you want to use alternative ports, edit the `config.json` file.

2. Add your neighbor's addresses to the `peering.json` file.

The Docker image runs under user with user id 65532 and group id 65532. To make sure there are no permission issues, you will need to:

1. Create the directory for the database by running the following command:

   ```sh
   sudo mkdir mainnetdb && sudo chown 65532:65532 mainnetdb
   ```

2. Create the directory for the peer database by running the following command:

   ```sh
   sudo mkdir p2pstore && sudo chown 65532:65532 p2pstore
   ```

3. Create the directory for the snapshots by running the following command:

   ```sh
   sudo mkdir -p snapshots/mainnet && sudo chown -R 65532:65532 snapshots
   ```

## Run

You can pull the latest image from `gohornet/hornet` public Docker hub registry by running:

```bash
docker pull gohornet/hornet:latest && docker tag gohornet/hornet:latest hornet:latest
```

We recommend that you run on host network to improve performance. Otherwise, you will have to publish ports using iptables NAT which is slower.

```sh
docker run \
  -v $(pwd)/config.json:/app/config.json:ro \
  -v $(pwd)/peering.json:/app/peering.json \
  -v $(pwd)/profiles.json:/app/profiles.json:ro \
  -v $(pwd)/mainnetdb:/app/mainnetdb \
  -v $(pwd)/p2pstore:/app/p2pstore \
  -v $(pwd)/snapshots/mainnet:/app/snapshots/mainnet \
  --restart always \
  --name hornet\
  --net=host \
  --ulimit nofile=8192:8192 \
  -d \
  hornet:latest
```

* `$(pwd)` Stands for the present working directory. All mentioned directories are mapped to the container, so the Hornet in the container persists the data directly to those directories.
* `-v $(pwd)/config.json:/app/config.json:ro` Maps the local `config.json` file into the container in `readonly` mode.
* `-v $(pwd)/peering.json:/app/peering.json` Maps the local `peering.json` file into the container.
* `-v $(pwd)/profiles.json:/app/profiles.json:ro` Maps the local `profiles.json` file into the container in `readonly` mode.
* `-v $(pwd)/mainnetdb:/app/mainnetdb` Maps the local `mainnetdb` directory into the container.
* `-v $(pwd)/p2pstore:/app/p2pstore` Maps the local `p2pstore` directory into the container.
* `-v $(pwd)/snapshots/mainnet:/app/snapshots/mainnet` Maps the local `snapshots` directory into the container.
* `--restart always` Instructs Docker to restart the container after Docker reboots.
* `--name hornet` Name of the running container instance. You can refer to the given container by this name.
* `--net=host` Instructs Docker to use the host's network, so the network is not isolated. We recommend that you run on host network for better performance. This way, the container will also open any ports it needs on the host network, so you will not need to specify any ports.
* `--ulimit nofile=8192:8192` increases the ulimits inside the container. This is important when running with large databases.
* `-d` Instructs Docker to run the container instance in a detached mode (daemon).


You can run `docker stop -t 300 hornet` to gracefully end the process.

## Create Username and Password for the Hornet Dashboard

If you use the Hornet dashboard, you need to create a secure password. You can start your Hornet container and execute the following command when the container is running:

```sh
docker exec -it hornet /app/hornet tool pwd-hash

```

Expected output:

```plaintext
Enter a password:
Re-enter your password:
Success!
Your hash: [YOUR_HASH_HERE]
Your salt: [YOUR_SALT_HERE]
```

You can edit `config.json` and customize the _dashboard_ section to your needs.

```sh
  "dashboard": {
    "bindAddress": "0.0.0.0:8081",
    "auth": {
      "sessionTimeout": "72h",
      "username": "admin",
      "passwordHash": "[YOUR_HASH_HERE]",
      "passwordSalt": "[YOUR_SALT_HERE]"
    }
  },
```

## Using Your Own Docker Compose File For Running Hornet

Docker Compose is a tool on top of the Docker engine that helps you to define Docker parameters in a structured way using a `docker-compose.yaml` file. You can create and start the container with a single `docker-compose` command based on your configuration.

To do so, you will need to create `docker-compose.yml` in the same directory as described in the [Prepare](#prepare) section:

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

## Managing a Node

:::note

Hornet uses an in-memory cache so to save all data to the underlying persistent storage, a grace period of at least 200 seconds for a shutdown is necessary.

:::

### Starting an Existing Hornet

You can start an existing Hornet container by running:

```bash
docker-compose start hornet
```

### Restarting Hornet

You can restart an existing Hornet container by running:

```bash
docker-compose restart -t 300 hornet
```

* `-t 300` Instructs Docker compose to wait for a grace period before shutting down.

### Stopping Hornet

You can stop an existing Hornet container by running:

```bash
docker-compose stop -t 300 hornet
```

* `-t 300` Instructs Docker compose to wait for a grace period before shutting down.

### Displaying Log Output

You can display existing Hornet container logs by running:

```bash
docker-compose logs -f hornet
```

* `-f`
Instructs Docker compose to continue displaying the log to `stdout` until CTRL+C is pressed.

## Removing a Container

You can remove an existing Hornet container by running:

```bash
docker-compose container rm hornet
```

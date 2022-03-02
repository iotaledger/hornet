---
description: Learn how to install and run a Hornet node using Docker. It is recommended for macOS and Windows.
image: /img/logo/HornetLogo.png
keywords:
- IOTA Node
- Hornet Node
- Docker
- Install
- Run
- macOS
- Windows
- how to

---

# Using Docker

Hornet Docker images (amd64/x86_64 architecture) are available at the [gohornet/hornet](https://hub.docker.com/r/gohornet/hornet) Docker hub.

## Requirements

1. A recent release of Docker enterprise or community edition. You can find installation instructions in the [official Docker documentation](https://docs.docker.com/engine/install/).
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

2. Add your neighbors addresses to the `peering.json` file.

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

## Build Your Own Hornet Image

You can build your own Docker image by running the following command:

```sh
docker build -f docker/Dockerfile -t hornet:latest .
```

Or pull it from Docker hub (only available for amd64/x86_64):

```sh
docker pull gohornet/hornet:latest && docker tag gohornet/hornet:latest hornet:latest
```

## Managing a Node

:::note

Hornet uses an in-memory cache so to save all data to the underlying persistent storage, a grace period of at least 200 seconds for a shutdown is necessary.

:::

### Starting an Existing Hornet

You can start an existing Hornet container by running:

```bash
docker start hornet
```

### Restarting Hornet

You can restart an existing Hornet container by running:

```bash
docker restart -t 300 hornet
```

* `-t 300` Instructs Docker to wait for a grace period before shutting down.

### Stopping Hornet

You can stop an existing Hornet container by running:

```bash
docker stop -t 300 hornet
```

* `-t 300` Instructs Docker to wait for a grace period before shutting down.

### Displaying Log Output

You can display existing Hornet container logs by running:

```bash
docker logs -f hornet
```

* `-f`
Instructs Docker to continue displaying the log to `stdout` until CTRL+C is pressed.

## Removing a Container

You can remove an existing Hornet container by running:

```bash
docker container rm hornet
```

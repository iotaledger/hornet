# Using Docker

Hornet Docker images (amd64/x86_64 architecture) are available at [gohornet/hornet](https://hub.docker.com/r/gohornet/hornet) Docker hub.

## Requirements

1. A recent release of Docker enterprise or community edition. (Follow this [link](https://docs.docker.com/engine/install/) for install instructions).
2. git and curl
3. At least 1GB available RAM

## Clone Repository

Clone the repository

```sh
git clone https://github.com/gohornet/hornet && cd hornet
```

The rest of the document assumes you are executing commands from the root directory of the repository.

## Prepare

1. Edit the `config.json` for alternative ports if needed.

2. Edit `peering.json` to your neighbors addresses.

3. The Docker image runs under user with user id 65532 and group id 65532. To make sure there are no permission issues, create the directory for the database, e.g.:

   ```sh
   sudo mkdir mainnetdb && sudo chown 65532:65532 mainnetdb
   ```

4. The Docker image runs under user with user id 65532 and group id 65532. To make sure there are no permission issues, create the directory for the snapshots, e.g.:

   ```sh
   sudo mkdir -p snapshots/mainnet && sudo chown 65532:65532 snapshots -R
   ```

## Run

Pull the latest image from `gohornet/hornet` public Docker hub registry:

```bash
docker pull gohornet/hornet:latest
```

Best is to run on host network for better performance (otherwise you are going to have to publish ports, that is done via iptables NAT and is slower)

```sh
docker run \
  -v $(pwd)/config.json:/app/config.json:ro \
  -v $(pwd)/peering.json:/app/peering.json \
  -v $(pwd)/profiles.json:/app/profiles.json \
  -v $(pwd)/mainnetdb:/app/mainnetdb \
  -v $(pwd)/snapshots/mainnet:/app/snapshots/mainnet \
  --restart always \
  --name hornet\
  --net=host \
  -d \
  hornet:latest
```

Use `docker stop -t 200 hornet` to gracefully end the process.

* `$(pwd)` \
Is for the current directory. All mentioned directories are mapped to container and so the Hornet in container persists the data directly to those directories.
* `-v $(pwd)/config.json:/app/config.json:ro` \
Maps the local `config.json` file into the container in `readonly` mode.
* `-v $(pwd)/peering.json:/app/peering.json` \
Maps the local `peering.json` file into the container.
* `-v $(pwd)/snapshots/mainnet:/app/snapshots/mainnet` \
Maps the local `snapshots` directory into the container.
* `-v $(pwd)/mainnetdb:/app/mainnetdb` \
Maps the local `mainnetdb` directory into the container.
* `--restart always` \
Instructs Docker the given container is restarted after Docker reboot
* `--name hornet` \
Name of the running container instance. You can refer to the given container by this name.
* `--net=host` \
Instructs Docker to use directly network on host (so the network is not isolated). The best is to run on host network for better performance. It also means it is not necessary to specify any ports. Ports that are opened by container are opened directly on the host.
* `-d` \
Instructs Docker to run the container instance in a detached mode (daemon).

## Create Username and Password for the Hornet Dashboard

If you use the Hornet dashboard you need to create a secure password. Start your Hornet container and run the following command when the container is running:

```sh
docker exec -it hornet /app/hornet tool pwdhash

Re-enter your password:
Success!
Your hash: [YOUR_HASH_HERE]
Your salt: [YOUR_SALT_HERE]
```

Edit `config.json` and customize the "dashboard" section to your needs.

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

:::info
Hornet uses an in-memory cache and so it is necessary to provide a grace period while shutting it down (at least 200 seconds) in order to save all data to the underlying persistent storage.
:::

### Starting an Existing Hornet

```bash
docker start hornet
```

### Restarting Hornet

```bash
docker restart -t 200 hornet
```

* `-t 200`: instructs Docker to wait for a grace period before shutting down

### Stopping Hornet

```bash
docker stop -t 200 hornet
```

* `-t 200`: instructs Docker to wait for a grace period before shutting down

### Displaying Log Output

```bash
docker logs -f hornet
```

* `-f` \
Instructs Docker to continue displaying the log to `stdout` until CTRL+C is pressed

## Removing a Container

```bash
docker container rm hornet
```

---
description: Learn how to install and run a HORNET node using Docker. It is recommended for macOS and Windows.
image: /img/Banner/banner_hornet_using_docker.png
keywords:
- IOTA Node
- HORNET Node
- Docker
- Install
- Run
- macOS
- Windows
- how to

---

# Install HORNET using Docker

![HORNET Node using Docker](/img/Banner/banner_hornet_using_docker.png)

This guide represents the recommended setup to run a HORNET node.
It includes everything required to setup a public node accessible by wallets and applications:
- [HORNET](https://github.com/iotaledger/hornet)
- [Traefik](https://traefik.io) - Reverse proxy using SSL certificates to secure access to the node API and dashboard.
- [Prometheus](https://prometheus.io) - Metrics scraper configured to collect all metrics from HORNET and INX extensions.
- [Grafana](https://grafana.com) - Data visualizer that can be used to display the metrics collected by Prometheus.
- [inx-dashboard](https://github.com/iotaledger/inx-dashboard) - Node dashboard.
- [inx-indexer](https://github.com/iotaledger/inx-dashboard) - Indexer extension for wallets and applications.
- [inx-mqtt](https://github.com/iotaledger/inx-mqtt) - MQTT extension providing the Event API.
- [inx-participation](https://github.com/iotaledger/inx-participation) - Participation extension providing on-tangle-voting.
- [inx-poi](https://github.com/iotaledger/inx-poi) - Extention to generate proofs of inclusion.
- [inx-spammer](https://github.com/iotaledger/inx-spammer) - Network spammer.
- [wasp](https://github.com/iotaledger/wasp) - L2 Node for IOTA Smart Contracts.

We only recommend running a node on hosted servers and not on personal computers.
Please take into consideration the points explained in the [Security 101](https://wiki.iota.org/nodes/explanations/security_101#securing-your-device).

HORNET Docker images (amd64/x86_64 and arm64 architecture) are available at the [iotaledger/hornet](https://hub.docker.com/r/iotaledger/hornet) Docker hub.

## Requirements
1. A recent release of Docker enterprise or community edition. You can find installation instructions in the [official Docker documentation](https://docs.docker.com/engine/install/).
2. [Docker Compose CLI plugin](https://docs.docker.com/compose/install/linux/).
3. A registered domain name pointing to the public IP address of your server. _(optional if not using HTTPS)_
4. Opening up the following ports in your servers firewall:
  - `15600 TCP` - Used for HORNET gossip.
  - `14626 UDP` - Used for HORNET autopeering.
  - `80 TCP` - Used for HTTP. _(can be changed, see below)_
  - `443 TCP` - Used for HTTPS. _(optional if not using HTTPS)_
  - `4000 UDP` - Used for Wasp gossip. _(optional if not using Wasp)_
  - `5550 TCP` - Used for Wasp nanomsg events. _(optional if not using Wasp)_
5. [curl](https://curl.se/).

## Download the latest release

> **NOTE**: The commands assume you are using Linux.

Once you have completed all the installation [requirements](#requirements), you can download the latest release by running:

```sh
mkdir hornet
cd hornet
curl -L -O "https://github.com/iotaledger/node-docker-setup/releases/download/v1.0.0-rc.1/node-docker-setup-v1.0.0-rc.1.tar.gz"
tar -zxf node-docker-setup-v1.0.0-rc.1.tar.gz
```

## Prepare

### 1. Setup Environment

You can configure your node to either use HTTP or HTTPS. For publicly exposed nodes we heavily recommend using HTTPS.

#### 1.1 HTTPS

Create a file named `.env` add the following to the file:

```
COMPOSE_FILE=docker-compose.yml:docker-compose-https.yml

ACME_EMAIL=your-email@example.com

NODE_HOST=node.your-domain.com
```

* Replace `your-email@example.com` with the e-mail used for issuing a [Let's Encrypt](https://letsencrypt.org) SSL certificate.
* Replace `node.your-domain.com` with the domain pointing to your public IP address as described in the [requirements](#requirements).

#### 1.2 HTTP

By default this setup will expose the Traefik reverse proxy on the default HTTP port `80`.
If you want to change the port to a different value you can create a file named  `.env` and add the following to e.g. expose it over port `9000`:

```
HTTP_PORT=9000
```

### 2. Setup neighbors

Add your HORNET neighbor addresses to the `peering.json` file.

:::note
This step is recommended, but optional if you are using autopeering.
See [peering](../references/peering.md) for more information.
:::


### 3. Create the `data` folder

All files used by HORNET, the INX extensions, Wasp, Traefik & co will be stored in a directory called `data`.
Docker image runs under user with user id 65532 and group id 65532, so this directory needs to have the correct permissions to be accessed by the containers.
To create this directory with correct permissions run the contained script:

```sh
./prepare_docker.sh
```

### 4. Set dashboard credentials

To access your HORNET dashboard, a set of credentials need to be configured.
Run the following command to generate a password hash and salt for the dashboard:

```
docker compose run hornet tool pwd-hash
```

Create a file named `.env` if you did not create it already and add the following lines:

```
DASHBOARD_PASSWORD=0000000000000000000000000000000000000000000000000000000000000000
DASHBOARD_SALT=0000000000000000000000000000000000000000000000000000000000000000
```

* Update the `DASHBOARD_PASSWORD` and `DASHBOARD_SALT` values in the `.env` file with the result of the previous command.

If you want to change the default `admin` username, you can add this line to your `.env` file:

```
DASHBOARD_USERNAME=someotherusername
```

### 5. Enable additional monitoring

To enable additional monitoring (cAdvisor, Prometheus, Grafana), the docker compose profile needs to be configured.
Create a file named `.env` if you did not create it already and add the following line:

```
COMPOSE_PROFILES=monitoring
```

### 6. Enable Wasp node

To also run a Wasp node, the docker compose profile needs to be configured.
Create a file named `.env` if you did not create it already and add the following line:

```
COMPOSE_PROFILES=wasp
```

If you already enabled the `monitoring` profile, modify the profiles:
```
COMPOSE_PROFILES=monitoring,wasp
```


## Run

### Starting the node

You can start the HORNET node and INX extensions by running:

```sh
docker compose up -d
```

* `-d` Instructs Docker to start the containers in the background.

#### HTTPS

After starting the node you will be able to access your services at the following endpoints:
- API: `https://node.your-domain.com/api/routes`
- HORNET Dashboard: `https://node.your-domain.com/dashboard`
- Grafana: `https://node.your-domain.com/grafana`  _(optional if using "monitoring" profile)_
- Wasp API: `https://node.your-domain.com/wasp/api`  _(optional if using "wasp" profile)_
- Wasp Dashboard: `https://node.your-domain.com/wasp/dashboard`  _(optional if using "wasp" profile)_

:::warning
   After starting your node for the first time, please change the default grafana credentials<br />
   User: `admin`<br />
   Password: `admin`
:::

You can configure your wallet software to use `https://node.your-domain.com`

#### HTTP

After starting the node you will be able to access your services at the following endpoints:
- API: `http://localhost/api/routes`
- HORNET Dashboard: `http://localhost/dashboard`
- Grafana: `http://localhost/grafana`  _(optional if using "monitoring" profile)_
- Wasp API: `http://localhost/wasp/api`  _(optional if using "wasp" profile)_
- Wasp Dashboard: `http://localhost/wasp/dashboard`  _(optional if using "wasp" profile)_

:::note
   If you changed the default `HTTP_PORT` value, you will need to add the port to the urls.
:::

You can configure your wallet software to use `http://localhost`

### Displaying Log Output

You can display the HORNET logs by running:
```sh
docker compose logs -f hornet
```

* `-f`
Instructs Docker to continue displaying the log to `stdout` until CTRL+C is pressed.

### Stopping the node

You can stop the HORNET node and INX extensions by running:
```sh
docker compose down
```

### Tools

To access the tools provided inside HORNET you can use:
```sh
docker compose run hornet tool <tool-name>
```

To see the list of tools included run:
```sh
docker compose run hornet tool -h
```

## JWT Auth

To generate a JWT token to be used to access protected routes you can run:
```sh
docker compose run hornet tool jwt-api --databasePath data/p2pstore
```

* If you changed the `restAPI.jwtAuth.salt` value in the `config.json`, then you need to pass that value as a parameter as `--salt <restAPI.jwtAuth.salt value from your config.json>`

## INX

This setup includes the INX extensions listed at the beginning of this guide.
If you want to disable certain extensions you can comment out the different services in the `docker-compose.yml` file and restart the node.

# More Information

For more information look at the [Github repository](https://github.com/iotaledger/node-docker-setup)

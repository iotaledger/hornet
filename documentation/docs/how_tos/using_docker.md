---
description: Learn how to install and run a Hornet node using Docker. It is recommended for macOS and Windows.
image: /img/Banner/banner_hornet_using_docker.png
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

# Install Hornet using Docker

![Hornet Node using Docker](/img/Banner/banner_hornet_using_docker.png)

This guide represents the recommended setup to run a Hornet node.
It includes everything required to setup a public node accessible by wallets and applications:
- [Hornet](https://github.com/iotaledger/hornet)
- [Traefik](https://traefik.io) - Reverse proxy using SSL certificates to secure access to the node API and dashboard.
- [Prometheus](https://prometheus.io) - Metrics scraper configured to collect all metrics from Hornet and INX extensions.
- [Grafana](https://grafana.com) - Data visualizer that can be used to display the metrics collected by Prometheus.

We only recommend running a node on hosted servers and not on personal computers.
Please take into consideration the points explained in the [Security 101](https://wiki.iota.org/develop/nodes/explanations/security_101#securing-your-device).

Hornet Docker images (amd64/x86_64 and arm64 architecture) are available at the [iotaledger/hornet](https://hub.docker.com/r/iotaledger/hornet) Docker hub.

## Requirements
1. A recent release of Docker enterprise or community edition. You can find installation instructions in the [official Docker documentation](https://docs.docker.com/engine/install/).
2. [Docker Compose CLI plugin](https://docs.docker.com/compose/install/linux/).
3. A registered domain name pointing to the public IP address of your server. _(optional if not using HTTPS)_
4. Opening up the following ports in your servers firewall:
  - `15600 TCP` - Used for Hornet gossip.
  - `14626 UDP` - Used for Hornet autopeering.
  - `80 TCP` - Used for HTTP. _(can be changed, see below)_
  - `443 TCP` - Used for HTTPS. _(optional if not using HTTPS)_
5. [curl](https://curl.se/).

## Download the latest release

> **NOTE**: The commands assume you are using Linux.

Once you have completed all the installation [requirements](#requirements), you can download the latest release by running:

```sh
mkdir hornet
cd hornet
curl -L -O "https://github.com/iotaledger/node-docker-setup/releases/download/v1.0.0-rc.5/node-docker-setup_chrysalis-v1.0.0-rc.5.tar.gz"
tar -zxf node-docker-setup_chrysalis-v1.0.0-rc.5.tar.gz
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

Add your Hornet neighbor addresses to the `peering.json` file.

:::note
This step is recommended, but optional if you are using autopeering.
:::


### 3. Create the `data` folder

All files used by Hornet, Traefik & co will be stored in a directory called `data`.
Docker image runs under user with user id 65532 and group id 65532, so this directory needs to have the correct permissions to be accessed by the containers.
To create this directory with correct permissions run the contained script:

```sh
./prepare_docker.sh
```

### 4. Set dashboard credentials

To access your Hornet dashboard, a set of credentials need to be configured.
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

## Run

### Starting the node

You can start the Hornet node by running:

```sh
docker compose up -d
```

* `-d` Instructs Docker to start the containers in the background.

#### HTTPS

After starting the node you will be able to access your services at the following endpoints:
- API: `https://node.your-domain.com/api/routes`
- Hornet Dashboard: `https://node.your-domain.com/dashboard`
- Grafana: `https://node.your-domain.com/grafana`  _(optional if using "monitoring" profile)_

:::warning
   After starting your node for the first time, please change the default grafana credentials<br />
   User: `admin`<br />
   Password: `admin`
:::

You can configure your wallet software to use `https://node.your-domain.com`

#### HTTP

After starting the node you will be able to access your services at the following endpoints:
- API: `http://localhost/api/routes`
- Hornet Dashboard: `http://localhost/dashboard`
- Grafana: `http://localhost/grafana`  _(optional if using "monitoring" profile)_

:::note
   If you changed the default `HTTP_PORT` value, you will need to add the port to the urls.
:::

You can configure your wallet software to use `http://localhost`

### Displaying Log Output

You can display the Hornet logs by running:
```sh
docker compose logs -f hornet
```

* `-f`
Instructs Docker to continue displaying the log to `stdout` until CTRL+C is pressed.

### Stopping the node

You can stop the Hornet node by running:
```sh
docker compose down
```

### Tools

To access the tools provided inside Hornet you can use:
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

# More Information

For more information look at the [Github repository](https://github.com/iotaledger/node-docker-setup)

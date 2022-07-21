# Hornet Docker Setup
This setup lets you run an [IOTA Hornet node](https://wiki.iota.org/hornet/welcome) using Docker to manage your services and Traefik as [a reverse proxy](https://en.wikipedia.org/wiki/Reverse_proxy) to enable TLS using [Let's Encrypt](https://letsencrypt.org/), control access to your node and route requests to the correct endpoints.

## Requirements
1. A recent release of Docker enterprise or community edition. You can find installation instructions in the [official Docker documentation](https://docs.docker.com/engine/install/).
2. [Docker Compose CLI plugin](https://docs.docker.com/compose/install/compose-plugin/).
3. A registered domain name pointing to the public IP address of your server.
4. Opening up the following ports in your servers firewall:
  - `15600 TCP` - Used for gossip.
  - `14626 UDP` - Used for autopeering.
  - `80 TCP` - Used for HTTP.
  - `443 TCP` - Used for HTTPS.

## Prepare

> **NOTE**: The commands assume you are using Linux.

### 1. Setup Environment

Create a file named `.env` add the following to the file:

```
ACME_EMAIL=your-email@example.com

HORNET_HOST=node.your-domain.com

DASHBOARD_USERNAME=admin
DASHBOARD_PASSWORD=0000000000000000000000000000000000000000000000000000000000000000
DASHBOARD_SALT=0000000000000000000000000000000000000000000000000000000000000000
```

* Replace `your-email@example.com` with the e-mail used for issuing a [Let's Encrypt](https://letsencrypt.org) SSL certificate.
* Replace `node.your-domain.com` with the domain pointing to your public IP address as described in the [requirements](#requirements).

### 2. Setup neighbors

Add your neighbors addresses to the `peering.json` file.

> **NOTE**:
> This step is recommended, but optional if you are using autopeering.
> See [peering](../references/peering.md) for more information.


### 3. Create the `data` folder

All files used by HORNET, the INX extensions, Traefik & co will be stored in a directory called `data`.
Docker image runs under user with user id 65532 and group id 65532, so this directory needs to have the correct permissions to be accessed by HORNET.
To create this directory with correct permissions run the contained script:

```sh
./prepare_docker.sh
```

### 4. Set dashboard credentials

To access your nodes dashboard, a set of credentials need to be configured.
Run the following command to generate a password hash and salt for the dashboard:

```
docker compose run hornet tool pwd-hash
```

* Update the `DASHBOARD_PASSWORD` and `DASHBOARD_SALT` values in the `.env` file with the result of the previous command.

## Run

### Starting HORNET

You can start a HORNET by running:

```sh
docker compose up -d
```

* `-d` Instructs Docker to start HORNET in the background.

After starting HORNET you will be able to access your node at the following endpoints:
- API: `https://node.your-domain.com/api/routes`
- Dashboard: `https://node.your-domain.com/dashboard`
- Grafana: `https://node.your-domain.com/grafana`

> **_Warning:_**
> After starting your node for the first time, please change the default grafana credentials<br />
> User: `admin`<br />
> assword: `admin`

You can configure your wallet software to use `https://node.your-domain.com`

### Displaying Log Output

You can display the HORNET logs by running:
```sh
docker compose logs -f hornet
```

* `-f`
  Instructs Docker to continue displaying the log to `stdout` until CTRL+C is pressed.

### Stopping HORNET

You can stop HORNET container by running:
```sh
docker compose down
```

### Tools

To access the tools provided inside HORNET you can use
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
If you want to disable certain extensions you can comment out the different services in the `docker-compose.yml` file and restart HORNET.


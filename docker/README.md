# Hornet Docker Setup
This setup lets you run an [IOTA Hornet node](https://wiki.iota.org/hornet/welcome) that connects to the `alphanet` test network, using Docker to manage your services and Traefik as [a reverse proxy](https://en.wikipedia.org/wiki/Reverse_proxy) to enable TLS using [Let's Encrypt](https://letsencrypt.org/), control access to your node and route requests to the correct endpoints.

## Prerequisites
- [Docker engine](https://docs.docker.com/engine/install/).
- A registered domain name.

## Usage
*NOTE: The commands assume you are using Linux.*

### Configure your node
Create a file named `.env`, add the following to the file and change the email and domain to your own:

```
ACME_EMAIL=your-email@example.com

HORNET_HOST=node.your-domain.com

DASHBOARD_USERNAME=admin
DASHBOARD_PASSWORD=0000000000000000000000000000000000000000000000000000000000000000
DASHBOARD_SALT=0000000000000000000000000000000000000000000000000000000000000000
```

Create the `data` folder with correct permissions by running the contained script:

```
./prepare_docker.sh
```

Generate a password hash and salt for the password you want to use to access your dashboard:

```
docker compose run hornet tool pwd-hash
```

And update the `DASHBOARD_PASSWORD` and `DASHBOARD_SALT` values in the `.env` file with the result of the previous command.

### Configure routing and access
Be sure to open the following ports for your firewall:

- `15600/tcp` for gossip.
- `14626/udp` for autopeering.

To get access to your node API, for some endpoints you need an authorization token (Hornet uses [JWT](https://jwt.io/)). To generate a token, run the `jwt-api` tool and use the correct p2pstore path:

```
docker compose run hornet tool jwt-api --databasePath data/p2pstore
```

*NOTE: Depending on your Docker installation you might need to run `docker-compose` instead.*


### Start your node
Start your node as a daemon process:

```
docker compose up -d
```

**WARNING: The initial Grafana credentials are admin/admin, so be sure to log in once to change them.**

## Further steps
Now that you have your node running, you can try any of the following:

- View logs of your node using `docker compose logs`.
- Stop your node using `docker compose down`.
- Access your node endpoints, for example:
    - `/api/core/v2/info` to get information about your node.
    - `/dashboard` to view your dashboard.
    - `/grafana` to view your Grafana dashboard, a powerful dashboard to display metrics about HORNET and your system.
- Run `docker compose run hornet tools` to find out about other available tools and run them with `docker compose run hornet tool [TOOL]`.

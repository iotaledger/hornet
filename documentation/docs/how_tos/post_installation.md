---
description: Introducing the Hornet nodes configuration files and their settings.
image: /img/logo/HornetLogo.png
keywords:
- IOTA Node 
- Hornet Node
- Configuration
- REST API
- Dashboard
- how to
---


# Post-installation

Once you have deployed Hornet, you can set all the parameters using configuration files.

## Configuration Files

The most important configuration files are:

* `config.json` - Includes all configuration flags and their values.
* `peering.json` - Includes all connection details to your static peers (neighbors).

:::note

Hornet version 0.5.x targets the legacy IOTA 1.0 network. Hornet version 1.x.x targets the IOTA 1.5 network, also known as [Chrysalis](https://wiki.iota.org/introduction/explanations/update/what_is_chrysalis), which is the focus of this documentation.

:::

Depending on the installation path you selected, default configuration files may also be part of the installation process. So, you may see the following configuration files in your deployment directory:

```bash
config.json
config_devnet.json
peering.json
profiles.json
```

## Default Configuration

By default, Hornet searches for configuration files in the working directory and expects default names, such as `config.json` and `peering.json`.

You can change this behavior by running Hornet with some altering arguments.

Please see the [config.json](../references/configuration.md) article for more information regarding the respective configuration file.

Once you have executed Hornet, it will output all loaded configuration parameters to `stdout` to show what configuration Hornet actually loaded (omitting sensitive values for things like passwords).

You can see a list of all the other altering command line parameters by running:

```bash
hornet --help
```

If you want a more detailed output you can run:

```bash
hornet --help --full
```

## Dashboard

By default, an admin dashboard (web interface) plugin is available on port 8081. This provides useful information regarding the node's health, peering/neighbors, overall network health, and consumed system resources.

The dashboard plugin only listens on localhost:8081 by default. If you want to make it accessible from the Internet, you will need to change the default configuration. It can be changed using the following `config.json` file section:

```json{2}
"dashboard": {
  "bindAddress": "localhost:8081",
  "auth": {
    "sessionTimeout": "72h",
    "username": "admin",
    "passwordHash": "0000000000000000000000000000000000000000000000000000000000000000",
    "passwordSalt": "0000000000000000000000000000000000000000000000000000000000000000"
  }
}
```

Change `dashboard.bindAddress` to either `0.0.0.0:8081` to listen on all available interfaces, or the specific interface address accordingly.

Even if it is accessible from the Internet, any visitor will still need a valid username and password combination to access the management section of the dashboard.

The password, hash, and salt can be generated using the integrated `pwd-hash` CLI tool:

```bash
./hornet tools pwd-hash
```

Output example:

```plaintext
Enter a password:
Re-enter your password:
Success!
Your hash: 24c832e35dc542901b90888321dbfc4b1d9617332cbc124709204e6edf7e49f9
Your salt: 6c71f4753f6fb52d7a4bb5471281400c8fef760533f0589026a0e646bc03acd4
```

:::note

The `pwd-hash` tool outputs the `passwordHash` and `passwordSalt` based on your input password.

:::

Copy both values to their corresponding configuration options: `dashboard.auth.passwordHash` and
`dashboard.auth.passwordSalt` respectively.

For the new password to take effect, you must restart Hornet.

## Configuring HTTP REST API

One of the tasks the the node is responsible for is exposing [API](../references/api_reference.md) to clients that would like to interact with the IOTA network, such as crypto wallets, exchanges, IoT devices, etc.

By default, Hornet will expose the [REST API](../references/api_reference.md) on port `14265`.
If you use the [recommended setup](using_docker.md) the API will be exposed on the default HTTPS port (`443`) and secured using an SSL certificate.

Since offering the HTTP REST API to the public can consume your node's resources, there are options to restrict which routes can be called and other request limitations:

### Routes

* `restAPI.publicRoutes` defines which routes can be called without JWT authorization. 
* `restAPI.protectedRoutes` defines which routes require JWT authorization.
* All other routes will not be exposed.

### JWT Auth

To generate a JWT-token to be used with the protected routes you can run:

```sh
./hornet tool jwt-api --databasePath <path to your p2pstore> --salt <restAPI.jwtAuth.salt value from your config.json>
```

If you are running our [recommended setup](using_docker.md) then see [here](using_docker.md#tools).

### Proof-of-Work

If you are concerned with resource consumption, consider turning off `restAPI.pow.enabled`. 
This way, the clients must perform proof of work locally before submitting a block for broadcast.
If you would like to offer proof of work to clients, consider increasing the `restAPI.pow.workerCount` to provide a faster block submission experience.

### Reverse Proxy
We recommend that you provide your HTTP REST API behind a reverse proxy, such as [HAProxy](http://www.haproxy.org/), [Traefik](https://traefik.io/), [Nginx](https://www.nginx.com/), or [Apache](https://www.apache.org/) configured with TLS.
When using our [recommended setup](using_docker.md) this is done for you automatically.

You can explore more details regarding different API calls at the [IOTA client library documentation](https://wiki.iota.org/iota.rs/welcome).

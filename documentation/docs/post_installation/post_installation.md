---
keywords:
- IOTA Node 
- Hornet Node
- Configuration
- REST API
- Dashboard
description: Introducing the Hornet nodes configuration files and their settings.
image: /img/logo/HornetLogo.png
---


# Post-installation

Once you have deployed Hornet, you can set all the parameters using configuration files.

## Configuration Files

The most important configuration files are:

* `config.json`: includes all configuration flags and their values.
* `peering.json`: includes all connection details to your static peers (neighbors).

:::info
Hornet version 0.5.x targets legacy IOTA 1.0 network. Hornet version 1.x.x targets IOTA 1.5 network, also known as [Chrysalis](https://chrysalis.docs.iota.org/), which is the focus of this documentation.
:::

Depending on the installation path you selected, default configuration files may also be part of the installation process. Therefore, you may see the following configuration files in your deployment directory:

```bash
config.json
config_chrysalis_testnet.json
peering.json
profiles.json
```

## Default Configuration

By default, Hornet searches for configuration files in the working directory and expects default names, such as `config.json` and `peering.json`.

You can change this behavior by running Hornet with some altering arguments.

Please see the [config.json](configuration.md) and [peering.json](peering.md) sections for more information regarding their respective configuration files.

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

By default, an admin dashboard (web interface) plugin is available on port 8081. It provides some useful information regarding the node's health, peering/neighbors, overall network health and consumed system resources.

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

Even if accessible from the Internet, any visitor will still need a valid username and password combination to access the management section of the dashboard.

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

:::info
The `pwd-hash` tool outputs the `passwordHash` and `passwordSalt` based on your input password.
:::

Copy both values to their corresponding configuration options: `dashboard.auth.passwordHash` and
`dashboard.auth.passwordSalt` respectively.

In order for the new password to take effect, you must restart Hornet.

## Configuring HTTP REST API

The node is responsible, amongst other tasks, of exposing the [HTTP REST ](../getting_started/nodes_101.md#http-rest-api) to clients that would like to interact with the IOTA network, such as crypto wallets, exchanges, IoT devices, etc.

By default, Hornet will publicly expose the [HTTP REST ](../getting_started/nodes_101.md#http-rest-api) on port 14265, ready to accept incoming connections from the Internet.

Since offering the HTTP REST API to the public can consume your nodes resources, there are options to restrict which routes can be called and other request limitations.

You can find the HTTP REST API related options in the  `restAPI` section within the `config.json` file:

```json
  "restAPI": {
    "jwtAuth": {
      "enabled": false,
      "salt": "HORNET"
    },
    "excludeHealthCheckFromAuth": false,
    "permittedRoutes": [
      "/health",
      "/mqtt",
      "/api/v1/info",
      "/api/v1/tips",
      "/api/v1/messages/:messageID",
      "/api/v1/messages/:messageID/metadata",
      "/api/v1/messages/:messageID/raw",
      "/api/v1/messages/:messageID/children",
      "/api/v1/messages",
      "/api/v1/transactions/:transactionID/included-message",
      "/api/v1/milestones/:milestoneIndex",
      "/api/v1/milestones/:milestoneIndex/utxo-changes",
      "/api/v1/outputs/:outputID",
      "/api/v1/addresses/:address",
      "/api/v1/addresses/:address/outputs",
      "/api/v1/addresses/ed25519/:address",
      "/api/v1/addresses/ed25519/:address/outputs",
      "/api/v1/treasury"
    ],
    "whitelistedAddresses": [
      "127.0.0.1",
      "::1"
    ],
    "bindAddress": "0.0.0.0:14265",
    "powEnabled": true,
    "powWorkerCount": 1,
    "limits": {
      "bodyLength": "1M",
      "maxResults": 1000
    }
  }
```

If you want to make the HTTP REST API only accessible from localhost, you change the `restAPI.bindAddress` config option accordingly.

`restAPI.permittedRoutes` defines which routes can be called from foreign addresses which are not defined under `restAPI.whitelistedAddresses`.

If you are concerned with resource consumption, consider turning off `restAPI.powEnabled`.  This way, the clients must perform proof of work locally, before submitting a message for broadcast. In case you'd like to offer proof of work to clients, consider increasing the `restAPI.powWorkerCount` to provide a faster message submission experience.

We suggest that you provide your HTTP REST API behind a reverse proxy, such as [HAProxy](http://www.haproxy.org/), [Traefik](https://traefik.io/), [Nginx](https://www.nginx.com/), [Apache](https://www.apache.org/) configured with TLS.

Please see some of our additional security recommendations in the [Security 101 section](../getting_started/security_101.md).

Feel free to explore more details regarding different API calls at the [IOTA client library documentation](https://chrysalis.docs.iota.org/libraries/client).

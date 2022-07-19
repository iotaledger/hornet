---
description: Introducing the HORNET nodes configuration files and their settings.
image: /img/logo/HornetLogo.png
keywords:
- IOTA Node 
- HORNET Node
- Configuration
- REST API
- Dashboard
- how to
---


# Post-installation

Once you have deployed HORNET, you can set all the parameters using configuration files.

## Configuration Files

The most important configuration files are:

* `config.json` - Includes all configuration flags and their values.
* `peering.json` - Includes all connection details to your static peers (neighbors).

## Default Configuration

By default, HORNET searches for configuration files in the working directory and expects default names, such as `config.json` and `peering.json`.

There are default configuration files available that you can use:

* `config_testnet.json` - Includes the default values required to join the Shimmer Testnet.
* `config_defaults.json` - Includes all default parameters used by HORNET. You can use this file as a reference when customizing your `config.json`

You can pick one of these files and use it as your `config.json` to join the configured network.

Please see the [`config.json`](../references/configuration.md) and [`peering.json`](../references/peering.md) articles for more information about the contents of the configuration files.

## Configuring HTTP REST API

One of the tasks the the node is responsible for is exposing [API](../references/api_reference.md) to clients that would like to interact with the IOTA network, such as crypto wallets, exchanges, IoT devices, etc.

By default, HORNET will expose the [Core REST API v2](../references/api_reference.md) on port `14265`. If you use the [recommended setup](using_docker.md) the API will be exposed on the default HTTPS port (`443`) and secured using an SSL certificate.

Since offering the HTTP REST API to the public can consume your node's resources, there are options to restrict which routes can be called and other request limitations.

You can find the HTTP REST API related options in the [`config.json` reference](../references/configuration.md#restapi)
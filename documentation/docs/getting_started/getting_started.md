---
description: Getting started with the recommended requirements and installation links.
image: /img/Banner/banner_hornet_getting_started.png
keywords:
- IOTA Node
- HORNET Node
- Linux
- macOS
- Windows
- Docker
- reference
- Requirements
---


# Getting Started

![HORNET Node getting started](/img/Banner/banner_hornet_getting_started.png)

Running a node is an efficient way to use IOTA or Shimmer. By doing so, you have direct access to the Tangle instead of having to
connect to and trust someone else's node. Additionally, you help the network to become more distributed and resilient.

The node software is the backbone of the network. For an overview of tasks a node is responsible for, please
see our [Node 101](https://wiki.iota.org/nodes/explanations/nodes_101) section.

To make sure that your device meets the minimum security requirements for running a node, please
see our [Security 101](https://wiki.iota.org/nodes/explanations/security_101) section.

:::note

Make sure you install HORNET v2.0.0+ since it is the minimum version that targets the Stardust upgrade.

:::

## Recommended Requirements

To handle a potential high rate of blocks per second, nodes need enough computational power to run reliably, and
should have the minimum specs:

- 4 cores or 4 vCPU.
- 8 GB RAM.
- SSD storage.
- A public IP address.

The amount of storage you need will depend on whether and how often you plan on pruning old data from your local
database.

HORNET exposes different functionality on different ports:

- `15600 TCP` - Gossip protocol port.
- `14626 UDP` - Autopeering port (optional).
- `14265 TCP` - REST HTTP API port (optional).

These ports are important for flawless node operation. The REST HTTP API port is optional and is only needed if
you want to offer access to your node's API. All ports can be customized inside
the [`config.json`](../how_tos/post_installation.md) file.

## Operating System

HORNET is written in Go and can be deployed on all major platforms.
The [recommended setup](../how_tos/using_docker.md) uses Docker to run HORNET together with some [INX](https://wiki.iota.org/nodes/about-nodes) extensions secured behind a [Traefik](https://traefik.io) SSL reverse proxy.

## Configuration

HORNET uses two JSON configuration files that you can tweak based on your deployment requirements:

- `config.json` - Includes all core configuration parameters.
- `peering.json` - Includes connection details to node neighbors (peers).

You can read more about the configuration in the [post installation](../how_tos/post_installation.md) article.

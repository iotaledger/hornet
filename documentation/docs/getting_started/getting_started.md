---
keywords:
- IOTA Node 
- Hornet Node
- Linux
- macOS
- Windows
- Docker
description: Getting Started.  Recommended requirements and installation links for Linux, macOS, and Windows
image: /img/logo/HornetLogo.png
---


# Getting Started

Running a node is the best way to use IOTA. By doing so, you have direct access to the Tangle instead of having to
connect to and trust someone else's node. Additionally, you help the IOTA network to become more distributed and resilient.

The node software is the backbone of the IOTA network. For an overview of tasks a node is responsible for, please
see [Node 101](nodes_101.md).

To make sure that your device meets the minimum security requirements for running a node, please
see [Security 101](security_101.md).

:::info
Make sure you install Hornet version 0.6.0+ since it is the minimum version that targets IOTA 1.5 (Chrysalis) network.
Versions below 0.6.0 (such as 0.5.x) target the legacy IOTA network which is not the focus of this documentation.
:::

## Recommended Requirements

To handle a potential high rate of messages per second, nodes need enough computational power to run reliably, and
should have following minimum specs:

- 4 cores or 4 vCPU
- 8 GB RAM
- SSD storage
- A public IP address

The amount of storage you need will depend on whether and how often you plan on pruning old data from your local
database.

Hornet exposes different functionality on different ports:

- 15600 TCP - Gossip protocol port
- 14626 UDP - Autopeering port (optional)
- 14265 TCP - REST HTTP API port (optional)
- 8081 TCP - Dashboard (optional)
- 1883 TCP - MQTT (optional)

The mentioned ports are important for flawless node operation. The REST HTTP API port is optional and is only needed if
you want to offer access to your node's API. All ports can be customized inside
the [config.json](../post_installation/configuration.md) file.

The default dashboard only listens on localhost:8081 per default. If you want to make it accessible from
the Internet, you will need to change the default configuration (though we recommend using a reverse proxy).

## Operating System

Hornet is written in Go and can be deployed on all major platforms using several installation methods.

Hornet ships as a single executable binary (`hornet` or `hornet.exe`) and some JSON configuration files; no further dependencies are needed.

### Linux (and Raspberry Pi)

Recommended installation methods:

- [Hornet apt repository](hornet_apt_repository.md#hornet-apt-repository-linux-distro-specific)
- [Docker image](using_docker.md)

Other installation methods:

- [Prebuilt binary files](hornet_apt_repository.md#pre-built-binaries)
- [Build from source](hornet_apt_repository.md#build-from-source)

### MacOS

Recommended installation methods:

- [Docker image](using_docker.md)

Other installation methods:

- [Prebuilt binary files](hornet_apt_repository.md#pre-built-binaries)

### Windows

Recommended installation methods:

- [Docker image](using_docker.md)

Other installation methods:

- [Prebuilt binary files](hornet_apt_repository.md#pre-built-binaries)

## Configuration

Hornet uses several JSON configuration files that can be adjusted based on your deployment and use cases:

- `config.json`: includes all core configuration parameters
- `peering.json`: includes connection details to node neighbors (peers)

See more details regarding the configuration in the [post installation](../post_installation/post_installation.md)
chapter.

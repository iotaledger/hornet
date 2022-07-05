---
description: Getting started with the recommended requirements and installation links for Linux, macOS, and Windows.
image: /img/Banner/banner_hornet_getting_started.png
keywords:
- IOTA Node
- Hornet Node
- Linux
- macOS
- Windows
- Docker
- reference

---


# Getting Started

![Hornet Node getting started](/img/Banner/banner_hornet_getting_started.png)

Running a node is an efficient way to use IOTA. By doing so, you have direct access to the Tangle instead of having to
connect to and trust someone else's node. Additionally, you help the IOTA network to become more distributed and resilient.

The node software is the backbone of the IOTA network. For an overview of tasks a node is responsible for, please
see our [Node 101](../explanations/nodes_101.md) section.

To make sure that your device meets the minimum security requirements for running a node, please
see our [Security 101](../explanations/security_101.md) section.

:::note

Make sure you install Hornet version 0.6.0+ since it is the minimum version that targets IOTA 1.5 (Chrysalis) network.
Versions below 0.6.0 (such as 0.5.x) target the legacy IOTA network which is not the focus of this documentation.

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

Hornet exposes different functionality on different ports:

- 15600 TCP - Gossip protocol port.
- 14626 UDP - Autopeering port (optional).
- 14265 TCP - REST HTTP API port (optional).

These ports are important for flawless node operation. The REST HTTP API port is optional and is only needed if
you want to offer access to your node's API. All ports can be customized inside
the [config.json](../how_tos/post_installation.md) file.

## Operating System

Hornet is written in Go and can be deployed on all major platforms using several installation methods.

Hornet ships as a single executable binary (`hornet` or `hornet.exe`) and some JSON configuration files; no further dependencies are needed.

### Linux (and Raspberry Pi)

You can install Hornet on Linux using:

- [The docker image](../how_tos/using_docker.md).

### MacOS

You can install Hornet on MacOS using:

- [The docker image](../how_tos/using_docker.md).

Alternatively, you can build Hornet by following these instructions:

- [Prebuilt binary files](../how_tos/using_docker.md#starting-an-existing-hornet).

### Windows

You can install Hornet on Windows using:

- [The docker image](../how_tos/using_docker.md).

## Configuration

Hornet uses two JSON configuration files that you can tweak based on your deployment requirements:

- `config.json`: includes all core configuration parameters.
- `peering.json`: includes connection details to node neighbors (peers).

You can read more about the configuration in the [post installation](../how_tos/post_installation.md)
article.

# Getting started
Running a node is the best way to use IOTA. By doing so, you have direct access to the Tangle instead of having to connect to and trust someone else's node, and you help the IOTA network to become more distributed and resilient.

Node software is the backbone of the IOTA network. For an overview of tasks any node is responsible for, please see [Node 101](./nodes_101.md).

To make sure that your device meets the minimum security requirements for running a node, please see [Security 101](./security_101.md).

> Please note: make sure you install Hornet version 0.6.0+ since it is the minimum version that targets IOTA 1.5 (Chrysalis) network. Version below 0.6.0 targets the legacy IOTA network.

## Recommended requirements
To handle a potential high rate of messages per second, nodes need enough computational power to run reliably, including the following requirements:
* 4 cores or 4 vCPU
* 8 GB RAM
* SSD storage
* A public IP address

The amount of storage you need will depend on whether and how often you plan on pruning messages from your local database.

Hornet exposes different functionality on different ports:
* 14626 UDP - Autopeering port (depends whether autopeering is enabled)
* 15600 TCP - Gossip port
* 14265 TCP - REST HTTP API port (optional)
* 8081 TCP - Dashboard/WebInterface (optional)

The mentioned ports are important for flawless node operation. The REST HTTP API port is optional and only needed if you want to offer access to your node's API. All ports can be customized in [config.json](../post_installation/config.md) file.

Please note: the default dashboard/webinterface only listens on localhost:8081 per default. If you want to make it accessible from the Internet, you will need to change the default configuration.


## Operating system
Hornet is written in Go and can be deployed on all major platforms using several different installation methods.

Hornet ships as a single executable binary (`hornet` or `hornet.exe`) and some JSON configuration files, no further dependencies are needed.

### Linux (and Raspberry Pi)
Available installation methods:
* [Hornet apt repository](./installation_steps.md#hornet-apt-repository) (RECOMMENDED)
* [Docker image](./installation_steps.md#docker-image) (RECOMMENDED)
* [Prebuilt binary files](./installation_steps.md#pre-built-binaries)
* [Build from source](./installation_steps.md#build-from-the-source-code)

### MacOS
Available installation methods:
* [Docker image](./installation_steps.md#docker-image) (RECOMMENDED)
* [Prebuilt binary files](./installation_steps.md#pre-built-binaries)

### Windows
Available installation methods:
* [Docker image](./installation_steps.md#docker-image) (RECOMMENDED)
* [Prebuilt binary files](./installation_steps.md#pre-built-binaries)

## Configuration
Hornet uses several JSON configuration files that can be adjusted based on your deployment and use cases:
* `config.json`: includes all core configuration parameters
* `peering.json`: includes connection details to node neighbors (peers)

See more details regarding the configuration in [post installation](../post_installation/config.md) chapter.

# Getting started
Running a node is the best way to use IOTA. By doing so, you have direct access to the Tangle instead of having to connect to and trust someone else node, and you help the IOTA network to become more distributed and resilient.

A node software is a backbone of IOTA network. For a overview of tasks any node is responsible for, please see [Node 101](./nodes_101.md).

To make sure that your device meets the minimum security requirements for running a node, please see [Security 101](./security_101.md).

> Please note: make sure you install Hornet version 0.6.0+ since it is the minimum version that targets IOTA 1.5 (Chrysalis) network. Version below 0.6.0 targets original IOTA network

## Recommended requirements
To handle a potential high rate of transactions per second, nodes need enough computational power to run reliably, including the following requirements:
* 4 cores or 4 vCPU
* 8 GB RAM
* SSD storage
* A public IP address (nodes gossiping with other nodes)

The amount of storage you need will depend on whether and how often do you plan on pruning messages from your local database.

Exposed a minimum set of ports:
* 14626 UDP - Autopeering port (depends whether autopeering is enabled)
* 15600 TCP - Gossip (neighbors) port
* 14265 TCP - Rest API port (optional)

The mentioned ports are important for a minimum flawless node operation. Rest API port is optional one if you want to provide access to your node's API calls from external parties. All ports can be customized in a [config.json](../post_installation/config.md) file.

There may be also additional ports required in order to work with additional optional plugins, such as dashboard, etc.

## Configuration
Hornet uses a set of several `json` configuration files that can be customized based on your use cases:
* `config.json`: a configuration file that includes all core configuration parameters
* `peering.json`: a configuration file that includes connection details to node neighbors (peers)

See more details regarding the configuration in [post installation](../post_installation/config.md) chapter.

## Operating system
Hornet is written in Go and can be deployed on all major platforms using a several installation methods.

### Linux (and Raspberry PI)
Recommended installation methods:
* [hornet apt repository](./installation_steps.md#hornet-apt-repository)
* [docker image](./installation_steps.md#docker-image)
* [prebuilt binary files](./installation_steps.md#pre-built-binaries)
* [build from the source code](./installation_steps.md#build-from-the-source-code)

### MacOS
Recommended installation methods:
* [docker image](./installation_steps.md#docker-image)
* [prebuilt binary files](./installation_steps.md#pre-built-binaries)

### Windows
Recommended installation methods:
* [docker image](./installation_steps.md#docker-image)
* [prebuilt binary files](./installation_steps.md#pre-built-binaries)

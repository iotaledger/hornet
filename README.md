# HORNET - The IOTA community node

![GitHub Workflow Status](https://img.shields.io/github/workflow/status/gohornet/hornet/Build?style=for-the-badge) ![GitHub release (latest by date)](https://img.shields.io/github/v/release/gohornet/hornet?style=for-the-badge) ![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/gohornet/hornet?style=for-the-badge) ![GitHub](https://img.shields.io/github/license/gohornet/hornet?style=for-the-badge)

<p><img src="https://raw.githubusercontent.com/gohornet/logo/master/HORNET_logo.svg?sanitize=true"></p>

HORNET is a powerful, community driven IOTA fullnode software written in Go.
It is easy to install and runs on low-end devices like the Raspberry Pi 4.

---

## Notes

- **Please open a [new issue](https://github.com/gohornet/hornet/issues/new) if you detect an error or crash (or submit a PR if you have already fixed it).**
- **The issue section is not a support section, if you have questions about HORNET please post them in the `#hornet` channel ([official iota discord server](https://discord.iota.org/)).**

---

_Table of contents_

<!--ts-->

- [Documentation](#documentation)
- [Autopeering](#autopeering)
- [Contributing](#contributing)
- [Installation](#installation)
- [Plugins](#plugins)
- [Docker](#docker)
<!--te-->

## Documentation

Please have a look into our [HORNET wiki](https://github.com/gohornet/hornet/wiki)

## Autopeering

**WARNING: The autopeering plugin will disclose your public IP address to possibly all nodes and entry points. Please disable the plugin if you do not want this to happen!**

The autopeering plugin is still in an early state. We recommend to add 1-2 static peers as well.
If you want to disable autopeering, you can do so by adding it to the `disablePlugins` in your `config.json`:

```json
"node": {
    "disablePlugins": ["Autopeering"],
    "enablePlugins": []
  },
```

## Contributing

- See [CONTRIBUTING](/CONTRIBUTING.md)

## Installation

### Binary

- Download the [latest release](https://github.com/gohornet/hornet/releases/latest) for your system (e.g. `HORNET-x.x.x_Linux_ARM64.tar.gz` for the Raspberry Pi 4)
- Extract the files in a folder of your choice
- Add neighbors to the `peering.json` file (optional)
- Run HORNET: `./hornet -c config`

### APT

```
wget -qO - https://ppa.hornet.zone/pubkey.txt | sudo apt-key add -
sudo sh -c 'echo "deb http://ppa.hornet.zone stable main" >> /etc/apt/sources.list.d/hornet.list'
sudo apt update
sudo apt install hornet
```

[Tutorial: Install HORNET with APT](https://github.com/gohornet/hornet/wiki/Tutorials%3A-Linux%3A-Install-HORNET)

---

## Plugins

HORNETs functionality is extended by plugins. Available plugins are listed [here](https://github.com/gohornet/hornet/wiki/Plugins).

---

## Docker

![Docker Pulls](https://img.shields.io/docker/pulls/gohornet/hornet?style=for-the-badge)

Pull HORNET from [Docker Hub](https://hub.docker.com/r/gohornet/hornet)

**Build a Docker image**

- See [Docker](docker/README.md)

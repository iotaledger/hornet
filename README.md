# HORNET - The IOTA community node

![GitHub Workflow Status](https://img.shields.io/github/workflow/status/gohornet/hornet/Build?style=for-the-badge) ![GitHub release (latest by date)](https://img.shields.io/github/v/release/gohornet/hornet?style=for-the-badge) ![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/gohornet/hornet?style=for-the-badge) ![GitHub](https://img.shields.io/github/license/gohornet/hornet?style=for-the-badge)

<p><img src="https://raw.githubusercontent.com/gohornet/logo/master/HORNET_logo.svg?sanitize=true"></p>

HORNET is a powerful, community driven IOTA fullnode software written in Go.
It is easy to install and runs on low-end devices like the Raspberry Pi 4.

---

## Notes

- **Currently HORNET is only released for testing purposes. Don't use it for wallet transfers (except testing with small amounts).**
- **Please open a [new issue](https://github.com/gohornet/hornet/issues/new) if you detect an error or crash (or submit a PR if you have already fixed it).**
- **The issue section is not a support section, if you have questions about HORNET please post them in the `#hornet` channel ([official iota discord server](https://discord.iota.org/)).**

---

_Table of contents_

<!--ts-->

- [Documentation](#documentation)
- [Autopeering](#autopeering)
- [Contributing](#contributing)
- [Installation](#installation)
- [Available Plugins](#available-plugins)
  - [TangleMonitor](#tanglemonitor)
  - [IOTA Tangle Visualiser](#iota-tangle-visualiser)
  - [MQTT Broker](#mqtt-broker)
  - [Spammer](#spammer)
  - [Autopeering](#autopeering)
- [Docker](#docker)
<!--te-->

## Documentation

Please have a look into our [HORNET wiki](https://github.com/gohornet/hornet/wiki)

## Autopeering

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
sudo apt install hornet
```

[Tutorial: Install HORNET with APT](https://github.com/gohornet/hornet/wiki/Tutorials%3A-Linux%3A-Install-HORNET)

---

## Available Plugins

### TangleMonitor

- Download the latest TangleMonitor source code

```bash
git clone https://github.com/unioproject/tanglemonitor.git
```

- Modify the `config.json` to fit your needs
  - `"tanglemonitorpath"` has to point to the frontend folder of the TangleMonitor source code
  - Add `"Monitor"` to `"enableplugins"`
  - Change `"webBindAddress"` to `"0.0.0.0:4434"` and `"apiBindAddress"` to `"0.0.0.0:4433"` if you want to access TangleMonitor from anywhere

```json
  "monitor": {
    "tangleMonitorPath": "tanglemonitor/frontend",
    "domain": "",
    "initialTransactions": 15000,
    "remoteApiPort": 4433,
    "webBindAddress": "localhost:4434",
    "apiBindAddress": "localhost:4433",
    "websocket": {
      "uri": ""
    }
  },
  "node": {
    "disableplugins": [],
    "enableplugins": ["Monitor"],
    "loglevel": 127
  },
```

### IOTA Tangle Visualiser

- Download the latest IOTA Tangle Visualiser and socket.io source code

```bash
git clone https://github.com/glumb/IOTAtangle.git
```

- Modify the `config.json` to fit your needs
  - `"webrootPath"` has to point to the frontend folder of the IOTA Tangle Visualiser source code
  - Add `"Graph"` to `"enableplugins"`
  - Change `"bindAddress"` to `"0.0.0.0:8083"` if you want to access IOTA Tangle Visualiser from anywhere

```json
  "graph": {
    "webRootPath": "IOTAtangle/webroot",
    "domain": "",
    "webSocket": {
      "uri": ""
    },
    "bindAddress": "localhost:8083",
    "networkName": "meets HORNET"
  },
  "node": {
    "disableplugins": [],
    "enableplugins": ["Graph"],
    "loglevel": 127
  },
```

### MQTT Broker

- Modify the `mqtt_config.json` to fit your needs
  - Change `"host"` to `"0.0.0.0"` if you want to access MQTT from anywhere
  - Change `"port"` to `""` and `"tlsPort"` to a port number if you want to use TLS (you also need certificate files)

```json
{
  ...
  "port": "1883",
  "host": "127.0.0.1",
  ...
  "tlsPort": "",
  "tlsHost": "",
  "tlsInfo": {
    "verify": false,
    "caFile": "tls/ca/cacert.pem",
    "certFile": "tls/server/cert.pem",
    "keyFile": "tls/server/key.pem"
  },
  "plugins": {}
}
```

- Modify the `config.json`
  - Add `"MQTT"` to `"enableplugins"`

```json
  "node": {
    "disableplugins": [],
    "enableplugins": ["MQTT"],
    "loglevel": 127
  },
```

### Spammer

- Modify the `config.json` to fit your needs
  - Change `"address"`, `"message"` and `"tag"`
  - `"tpsratelimit"` defines how many transactions (TX) the spammer should try to send (e.g. 0.1 stands for 0.1 TX per second --> 1 TX every 10 seconds. NOTE: the maximum `"tpsratelimit"` is limited by your used hardware.
  - Add `"Spammer"` to `"enableplugins"`

```json
  "spammer": {
    "address": "HORNET99INTEGRATED99SPAMMER999999999999999999999999999999999999999999999999999999",
    "depth": 3,
    "message": "Spamming with HORNET tipselect",
    "tag": "HORNET99INTEGRATED99SPAMMER",
    "tpsratelimit": 0.1,
    "workers": 1
  },
  "node": {
    "disableplugins": [],
    "enableplugins": ["Spammer"],
    "loglevel": 127
  },
```

---

## Docker

![Docker Pulls](https://img.shields.io/docker/pulls/gohornet/hornet?style=for-the-badge)

Pull HORNET from [Docker Hub](https://hub.docker.com/r/gohornet/hornet)

**Build a Docker image**

- See [Docker](docker/README.md)

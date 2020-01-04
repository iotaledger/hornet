# HORNET - The IOTA community node

![GitHub Workflow Status](https://img.shields.io/github/workflow/status/gohornet/hornet/Build?style=for-the-badge) ![GitHub release (latest by date)](https://img.shields.io/github/v/release/gohornet/hornet?style=for-the-badge) ![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/gohornet/hornet?style=for-the-badge) ![GitHub](https://img.shields.io/github/license/gohornet/hornet?style=for-the-badge)

<p><img src="https://raw.githubusercontent.com/gohornet/logo/master/HORNET_logo.svg?sanitize=true"></p>

HORNET is a lightweight alternative to IOTA's fullnode software “[IRI](https://github.com/iotaledger/iri)”.
The main advantage is that it compiles to native code and does not need a Java Virtual Machine, which considerably decreases the amount of needed resources while significantly increasing the performance.
This way, HORNET is easier to install and runs on low-end devices.

---

### Notes

- **Currently HORNET is only released for testing purposes. Don't use it for wallet transfers (except testing with small amounts).**
- **Please open a [new issue](https://github.com/gohornet/hornet/issues/new) if you detect an error or crash (or submit a PR if you have already fixed it).**
- **The issue section is not a support section, if you have questions about HORNET please post them in the `#hornet` channel ([official iota discord server](https://discord.iota.org/)).**

---

### Run HORNET

- Download the [latest release](https://github.com/gohornet/hornet/releases/latest) for your system (e.g. `HORNET-x.x.x_Linux_ARM.tar.gz` for the Raspberry Pi 3B)
- Extract the files in a folder of your choice
- Add neighbors to the `neighbors.json` file
- Download the latest HORNET snapshot from [dbfiles.iota.org](https://dbfiles.iota.org/mainnet/hornet/latest-export.gz.bin)
- Run HORNET: `./hornet -c config`

---

### Available plugins

#### TangleMonitor

- Download the latest TangleMonitor source code

```bash
git clone https://github.com/unioproject/tanglemonitor.git
```

- Modify the `config.json` to fit your needs
  - `"tanglemonitorpath"` has to point to the frontend folder of the TangleMonitor source code
  - Add `"Monitor"` to `"enableplugins"`
  - Change `"host"` to `"0.0.0.0"` if you want to access TangleMonitor from anywhere
  - Only change the `"port"` and the `"apiPort"` if you redirect them back to the default ports, because they are hardcoded in the frontend
```json
  "monitor": {
    "tanglemonitorpath": "tanglemonitor/frontend",
    "domain": "",
    "host": "127.0.0.1",
    "port": 4434,
    "apiPort": 4433
  },
  "node": {
    "disableplugins": [],
    "enableplugins": ["Monitor"],
    "loglevel": 127
  },
```

#### IOTA Tangle Visualiser

- Download the latest IOTA Tangle Visualiser and socket.io source code
```bash
git clone https://github.com/glumb/IOTAtangle.git
git clone https://github.com/socketio/socket.io-client.git
```
- Modify the `config.json` to fit your needs
    - `"webrootPath"` has to point to the frontend folder of the IOTA Tangle Visualiser source code
    - Add `"Graph"` to `"enableplugins"`
    - Change `"host"` to `"0.0.0.0"` if you want to access IOTA Tangle Visualiser from anywhere
```json
  "graph": {
    "webrootPath": "IOTAtangle/webroot",
    "socketiopath": "socket.io-client/dist/socket.io.js",
    "domain": "",
    "host": "127.0.0.1",
    "port": 8083,
    "networkName": "meets HORNET"
  },
  "node": {
    "disableplugins": [],
    "enableplugins": ["Graph"],
    "loglevel": 127
  },
```

#### MQTT Broker

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

#### Spammer

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

### Docker

![Docker Pulls](https://img.shields.io/docker/pulls/gohornet/hornet?style=for-the-badge)

Pull HORNET from [Docker Hub](https://hub.docker.com/r/gohornet/hornet)

**Build a Docker image**

- See [Docker](DOCKER.md)

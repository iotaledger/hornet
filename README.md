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
- Add neighbors to the config.json file
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

```json
  "monitor": {
    "tanglemonitorpath": "tanglemonitor/frontend",
    "domain": "",
    "host": "127.0.0.1"
  },
  "node": {
    "disableplugins": [],
    "enableplugins": ["Monitor"],
    "loglevel": 3
  },
```

#### Spammer

- Modify the `config.json` to fit your needs
  - Change `"address"`, `"message"`, `"tag"` and `"tpsratelimit"`
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
    "loglevel": 3
  },
```

---

### Docker

![Docker Pulls](https://img.shields.io/docker/pulls/gohornet/hornet?style=for-the-badge)

Pull HORNET from [Docker Hub](https://hub.docker.com/r/gohornet/hornet)

**Build a Docker image**

- See [Docker](DOCKER.md)

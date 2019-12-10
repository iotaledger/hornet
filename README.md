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
- **Feature requests will be deleted, because we cannot handle them at the moment.**
- **The issue section is not a support section, if you have questions about HORNET please post them in the `#hornet` channel ([official iota discord server](https://discord.iota.org/)).**

---

### Run HORNET

- Download the [latest release](https://github.com/gohornet/hornet/releases/latest) for your system (e.g. `HORNET-x.x.x_Linux_ARM.tar.gz` for the Raspberry Pi 3B)
- Extract the files in a folder of your choice
- Add neighbors to the config.json file
- Download the latest HORNET snapshot from [dbfiles.iota.org](https://dbfiles.iota.org/mainnet/hornet/latest-export.gz.bin)
- Run HORNET: `./hornet -c config`
---
description: How to run your own private Chrysalis Tangle
image: /img/Banner/banner_hornet_private_tangle.png
keywords:
- IOTA Node 
- HORNET Node
- Private Tangle
- Chrysalis
- Coordinator
- Wallet
- how to

---

# Run a Private Tangle

![HORNET Node Private Tangle](/img/Banner/banner_hornet_private_tangle.png)

This guide explains how you can run your own private tangle.
Private tangles are targetted at developers wanting to test their solutions in a private network environment. 

## Requirements

1. A recent release of Docker enterprise or community edition. You can find installation instructions in the [official Docker documentation](https://docs.docker.com/engine/install/).
2. [Docker Compose CLI plugin](https://docs.docker.com/compose/install/compose-plugin/).

## Download the latest release

Once you have completed all the installation [requirements](#requirements), you can download the latest release by running:

```sh
mkdir private_tangle
cd private_tangle
curl -L -O "https://github.com/iotaledger/hornet/releases/download/v2.0.0-rc.1/HORNET-2.0.0-rc.1-private_tangle.tar.gz"
tar -zxf HORNET-2.0.0-rc.1-private_tangle.tar.gz
```

## Bootstrap your network

To bootstrap the network you should run:
```sh
./bootstrap.sh
```

## Run your network

To run the private tangle you should run:
```sh
./run.sh
```

This will start the private tangle using a coordinator node and second node.
You can use `./run.sh 3` or `./run.sh 4` to start the private tangle with additional nodes instead.

## Start the coordinator in case of failure

The `inx-coordinator` container always starts together with the other containers if you execute the `./run.sh` command.
It may happen that the node startup takes longer than expected due to bigger databases or slow host machines. In that case the `inx-coordinator` container shuts down and won't be restarted automatically for security reasons.

If you want to restart the `inx-coordinator` separately, run the following command:
```sh
docker compose start inx-coordinator
```

## Access your network

All the information required to access the private tangle is contained inside the `README.md`.
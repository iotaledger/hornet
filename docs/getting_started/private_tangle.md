# Private Tangle

This is a small tutorial on how to run your own private [Chrysalis](https://chrysalis.iota.org/) tangle.

You will need to set up [a Coordinator](https://legacy.docs.iota.org/docs/getting-started/1.1/the-tangle/the-coordinator) and at least one additional [node](./getting_started.md) and distribute some tokens to an address that you can manage using a wallet. You will do this by running some scripts from the _private_tangle_ folder that you can find in the HORNET repository.

## Preparations

1. [Build HORNET](./installation_steps.md).
2. Install a wallet built for Chrysalis, for example [the cli-wallet](https://github.com/iotaledger/cli-wallet).

## Configuration

The scripts in the _private_tangle_ folder of the HORNET repository, which you will use to run the Coordinator and the nodes, are preconfigured. By default, they will distribute the tokens to the following address:

```
atoi1qpszqzadsym6wpppd6z037dvlejmjuke7s24hm95s9fg9vpua7vluehe53e
```

If you want to use an existing address, search for your address in [the explorer](https://explorer.iota.org/mainnet) and look for the corresponding ED25519 address. You can find the _create_snapshot_private_tangle_ script in the _private_tangle_ directory.  You will need to update the following line to use your own ED25519 address:

```bash
...
go run "..\main.go" tool snapgen private_tangle1 [ADDRESS] 1000000000 "snapshots\private_tangle1\full_snapshot.bin"
...
```

## Start the Coordinator

In the HORNET repository, change to the _private_tangle_ directory and run the `run_coo_bootstrap` script. This will create all the necessary files to run the network, distribute the tokens to the address you configured, and start the Coordinator.

In subsequent starts you can use the `run_coo` script, to skip the bootstrap step.

## Start Additional Nodes

To start additional nodes you can use the _run_ scripts provided in the +private_tangle_ folder. You can run them on the same device, as they are preconfigured to listen on different ports.

Congratulations, you are now running a private network! You can find the dashboard at [http://localhost:8081](http://localhost:8081), and use it to monitor your Coordinator. You can log in to the Dashboard using `admin` as username and password.

## Use a Wallet to Manage the Tokens

To easily access the tokens on the network, you need to take one more step. If you used the default configuration, you can use the following mnemonic to set up a wallet:

```
giant dynamic museum toddler six deny defense ostrich bomb access mercy blood explain muscle shoot shallow glad autumn author calm heavy hawk abuse rally
```

You can now connect your wallet to one of the nodes that are running. You should be able to find the tokens distributed to your wallet.

If you are using the [cli-wallet](https://github.com/iotaledger/cli-wallet), you should run the following command:

```bash
> wallet mnemonic "giant dynamic museum toddler six deny defense ostrich bomb access mercy blood explain muscle shoot shallow glad autumn author calm heavy hawk abuse rally"
> wallet new --node "http://localhost:14266" --alias EXAMPLE
```

You are now ready start using your own private tangle!

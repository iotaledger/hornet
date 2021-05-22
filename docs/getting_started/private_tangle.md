This is a small tutorial on how to run your own private [Chrysalis](https://chrysalis.iota.org/) tangle.

We will need to set up [a coordinator](https://legacy.docs.iota.org/docs/getting-started/1.1/the-tangle/the-coordinator) and at least one additional [node](./getting_started.md) and distribute some tokens to an address that we can manage using a wallet. We will do this by running some scripts from the *alphanet* folder that you will find in the HORNET respository.

### Preparations

- [Build HORNET](./installation_steps.md)
- Get a wallet built for Chrysalis (for example [the cli-wallet](https://github.com/iotaledger/cli-wallet))

### Configuration

The scripts in the *private_tangle* folder of the HORNET repository, which we will use to run the coordinator and the nodes, are preconfigured. By default they will distribute the tokens to the following address:

```
atoi1qpszqzadsym6wpppd6z037dvlejmjuke7s24hm95s9fg9vpua7vluehe53e
```

If you want to use an existing address, search for your address in [the explorer](https://explorer.iota.org/mainnet) and look for the corresponding ED25519 address. Find the *create_snapshot_private_tangle* script in the *private_tangle* directory and change the following line to use your ED25519 address:

```bash
...
go run "..\main.go" tool snapgen private_tangle1 [ADDRESS] 1000000000 "snapshots\private_tangle1\full_snapshot.bin"
...
```

### Start the coordinator

In the HORNET repository, change to the *private_tangle* directory and run the *run_coo_bootstrap* script. This will create all files needed to run the network and distribute the tokens to the address we configured. It will also start the coordinator.

In subsequent starts we can use the *run_coo* script, to skip the bootstrap step.

### Start additional nodes

To start additional nodes we simply use the additional *run* scripts provided in the *private_tangle* folder. We can run them on the same device, as they are preconfigured to listen on different ports.

Congratulations, you are now running a private network! To monitor your coordinator, you can find the dashboard at [http://localhost:8081](http://localhost:8081). Login to the Dashboard with `admin` as username and password.

### Use a wallet to manage the tokens

To easily access the tokens on the network, we need to take one more step. If you used the default configuration, use the following mnemonic to set up a wallet:

```
giant dynamic museum toddler six deny defense ostrich bomb access mercy blood explain muscle shoot shallow glad autumn author calm heavy hawk abuse rally
```

Now connect your wallet to one of the nodes that are running and you should be able to find the tokens distributed to your wallet.

Using [the cli-wallet](https://github.com/iotaledger/cli-wallet), the commands would be:

```bash
> wallet mnemonic "giant dynamic museum toddler six deny defense ostrich bomb access mercy blood explain muscle shoot shallow glad autumn author calm heavy hawk abuse rally"
> wallet new --node "http://localhost:14266" --alias EXAMPLE
```

You are now all set to start using your own private tangle!
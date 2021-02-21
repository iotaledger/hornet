This is a small tutorial on how to run your own private Tangle.

**Note: This tutorial assumes that you are using Ubuntu. The setup process may differ on other OS.**

### Preparations

- Set up your server and install HORNET ([APT install](./setup.md)) - **Do not start it yet**
- Generate a secure Seed. Even if it's a private Tangle without real value you should use a random Seed.

### Configuration

Adapt the configuration to your needs<br>
<br>

**Enable the coordinator plugin:**

```json
"node":{
    "alias": "Coordinator",
    "showAliasInGetNodeInfo": false,
    "disablePlugins": [],
    "enablePlugins": ["Coordinator"]
},
```

**Coordinator settings:**
<br>
[Coordinator configuration docu](https://github.com/gohornet/hornet/wiki/Configuration#Coordinator)
<br><br>
Some hints:

- Set a low MWM. If you are the only one using this Tangle, you need no (high) spam protection
- The higher the merkle tree depth, the longer it will take to compute the merkle tree. Calculate how many milestones you really need:<br>Number of possible milestones: 2<sup>merkleTreeDepth</sup>

<br>
Example:

```json
  "coordinator": {
    "address": "",
    "securityLevel": 2,
    "merkleTreeDepth": 18,
    "mwm": 5,
    "stateFilePath": "coordinator.state",
    "merkleTreeFilePath": "coordinator.tree",
    "intervalSeconds": 60,
    "checkpoints": {
      "maxTrackedTails": 10000
    }
  },
```

**Global snapshot settings:**
<br>
[Snapshot configuration docu](https://github.com/gohornet/hornet/wiki/Configuration#Snapshots)
<br><br>

```json
  "snapshots": {
    "loadType": "global",
    "global": {
      "path": "snapshot.csv",
      "spentAddressesPaths": [],
      "index": 0
    }
  },
```

### Generate the merkle tree

**Note: This may take some time (depending on the merkle tree depth and your hardware)**

1.  ```bash
    cd /var/lib/hornet
    sudo -u hornet COO_SEED="YOUR9COO9SEED9HERE..." hornet tool merkle
    ```

    The output will look like this:

    ```
    calculating 1024 addresses...
    calculated 1024/1024 (100.00%) addresses (took 1s).
    calculating nodes for layer 9
    calculating nodes for layer 8
    calculating nodes for layer 7
    calculating nodes for layer 6
    calculating nodes for layer 5
    calculating nodes for layer 4
    calculating nodes for layer 3
    calculating nodes for layer 2
    calculating nodes for layer 1
    calculating nodes for layer 0
    merkle tree root: BHKJSBMRSZLFMXJFYE9NHYTZCRAZQHLZTIBTKVNZLVWAXKESPOANYARWQYOYYHONDJYEAMMSOQEGGEPKB
    successfully created merkle tree (took 1s).
    ```

2.  Add the `merkle tree root` address to your config<br><br>
    Example:
```json
  "coordinator": {
    "address": "BHKJSBMRSZLFMXJFYE9NHYTZCRAZQHLZTIBTKVNZLVWAXKESPOANYARWQYOYYHONDJYEAMMSOQEGGEPKB",
    "securityLevel": 2,
    "merkleTreeDepth": 18,
    "mwm": 5,
    "stateFilePath": "coordinator.state",
    "merkleTreeFilePath": "coordinator.tree",
    "intervalSeconds": 60,
    "checkpoints": {
      "maxTrackedTails": 10000
    }
  },
```

### Initial IOTA Distribution

If you set up a new tangle you initially need to distribute the IOTA to at least one address.

1.  Generate a secure (random) Seed and generate an address with this Seed
2.  Backup this Seed
3.  Create a new file called `snapshot.csv` and add this contend:

    ```
    YOUR9GENERATED9ADDRESS9FROM9YOUR9SEED;2779530283277761
    ```

4.  Save this file to the HORNET dir (`/var/lib/hornet`)

### Bootstrap the Coordinator

1.  Change to the HORNET dir:
    ```
    cd /var/lib/hornet
    ```
2.  Run HORNET the first time:
    ```
    sudo -u hornet COO_SEED="YOUR9COO9SEED9HERE..." hornet --cooBootstrap
    ```
3.  Once the bootstrap process is done you can stop HORNET (CTRL+C) and let it run as a service:
    ```
    sudo sh -c 'echo "COO_SEED=YOUR9COO9SEED9HERE..." >> /etc/default/hornet'
    sudo systemctl enable --now hornet.service
    ```
4.  Congrats, your HORNET Coordinator is up and running!

### Add additional HORNET nodes to your private tangle

1.  Install an additional HORNET
2.  Copy over the config.json from your HORNET Coordinator (**make sure you removed `Coordinator` from the `"enablePlugins"`**)
3.  Copy over the `snapshot.csv` from your HORNET Coordinator
4.  Start HORNET

Note: Once your HORNET Coordinator node created the first local snapshot, you can use this local snapshot to add new HORNET nodes to your private tangle.

### Use a new merkle tree

In case you run out of new milestones, you can replace the old merkle tree with a new one.

1.  Stop the HORNET Coordinator
2.  Generate a new merkle tree
3.  Replace the `coordinator.address` in your `config.json` with the new `merkle tree root` address
4.  Migrate your HORNET Coordinator to this merkle tree:
    ```
    cd /var/lib/hornet
    sudo rm coordinator.state
    sudo -u hornet hornet --overwriteCooAddress --cooBootstrap --cooStartIndex xxxx
    ```
    Note: Replace `xxxx` with the start index (e.g. last issued milestone index + 1)
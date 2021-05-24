# Peering configuration

The easiest way to add peers in Hornet is via the Dashboard. Simply go to `Peers` and click on `Add Peer`.

But for the sake of completeness this document describes the structure of the `peering.json` file.

The default config file is named `peering.json`. You can change the path or name of the config file by using the `-n` or `--peeringConfig` argument while executing `hornet` executable.

The `peering.json` file contains a list of peers. Peers have the following attributes:

| Name         | Description                                 | Type   |
| :----------- | :------------------------------------------ | :----- |
| alias        | alias of the peer                          | string |
| multiAddress | multiAddress of the peer                    | string |

Example:

```json
{
  "peers": [
    {
      "alias": "Node1",
      "multiAddress": "/ip4/192.0.2.0/tcp/15600/p2p/12D3KooWCKWcTWevORKa2KEBputEGASvEBuDfRDSbe8t1DWugUmL"
    },
    {
      "alias": "Node2",
      "multiAddress": "/ip6/2001:db8:3333:4444:5555:6666:7777:8888/tcp/16600/p2p/12D3KooWJDqHjhd8us8XdbKy1Adp5nV6XoI7XhjZbPWAfbAbkLbH"
    },
    {
      "alias": "Node3",
      "multiAddress": "/dns/example.com/tcp/15600/p2p/12D3KooWN7F4eRAYbavnasME8WGXwkrpzWWoZSXfNSEpudmWi9YP"
    }
  ]
}
```

## Make your PeerID static

It's a good practice to have a static PeerID, so don't need to resend it to your peers when docker is restarted. Before start container you can generate a p2pidentity with the folowing command:

```sh
hornet tools p2pidentity
Your p2p private key:  [YOUR_PK_HERE]
Your p2p public key:  [YOUR_PUBKEY_HERE]
Your p2p PeerID:  [YOUR_PEERID_HERE]
```
If you already started hornet and have PeerID, you can extract you private key. You need to specify the location of p2pstore folder in the following command:
```sh
hornet tools p2pidentityextract p2pstore
Your p2p private key:  [YOUR_PK_HERE]
Your p2p public key:  [YOUR_PUBKEY_HERE]
Your p2p PeerID:  [YOUR_PEERID_HERE]
```

Edit `config.json` and add your private key.

```sh
"p2p": {
    "bindMultiAddresses": [
      "/ip4/0.0.0.0/tcp/15600"
    ],
    "connectionManager": {
      "highWatermark": 10,
      "lowWatermark": 5
    },
    "gossipUnknownPeersLimit": 4,
    "identityPrivateKey": "[YOUR_PK_HERE]",
    "peerStore": {
      "path": "./p2pstore"
    },
    "reconnectInterval": "30s"
  },
```


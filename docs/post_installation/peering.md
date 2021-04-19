# Peering configuration

The easiest way to add peers in Hornet is via dashboard. Simply go to `Peers` and click on `Add Peer`.

But for the sake of completeness this document describes the structure of the `peering.json` file.

The default config file is `peering.json`. You can change the path or name of the config file by using the `-n` or `--peeringConfig` argument while executing `hornet` executable.

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

## Autopeering

The autopeering plugin is still in an early state. We still recommend to add 1-2 static peers as well. If you want to disable autopeering, you can do so by adding it to the disablePlugins in your `config.json`:
```json
"node": {
    "disablePlugins": ["Autopeering"],
    "enablePlugins": []
  }
```

> Please note: `autopeering` plugin will disclose your public IP address to possibly all nodes and entry points. Please disable the plugin if you do not want this to happen.
# peering.json

The easiest way to add peers in Hornet is via dashboard. Simply go to `Peers` and click on `Add Peer`.  
But for the sake of completeness this document describes the structure of the `peering.json` file.

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
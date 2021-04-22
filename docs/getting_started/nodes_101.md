# Nodes 101

The IOTA network is a distributed type of network called Tangle, and the network is distributed among plenty of servers
called nodes. Nodes are the backbone of an IOTA network. This section covers what nodes do in an IOTA network.

Nodes are responsible for the following:

- Providing an API to interact with the Tangle/IOTA network.
- Validating [messages]((https://chrysalis.docs.iota.org/guides/dev_guide.html#messages-payloads-and-transactions)) and ledger mutations for consistency.
- Provide data for other nodes to synchronize to the latest state of the network.

## Attaching new messages to the Tangle

A `message` is a data structure that is actually being broadcasted in the IOTA network and represents a vertex in the
Tangle graph. When nodes receive a new message, they attach it to the Tangle by adding it to their local database.

As a result, at any point in time, all nodes may have different messages in their local databases. These messages make
up a node's view of the Tangle.

To distribute the messages across the rest of the network, nodes synchronize their local databases with their neighbors.

## Synchronizing with the rest of the network

Like any distributed system, nodes in an IOTA network synchronize their databases with others called neighbors to form a
single source of truth.

When one node, no matter where it is in the world, receives a message, it will try to gossip it to all its neighbors.
This way, all nodes eventually see all messages and store them in their local databases.

To synchronize, nodes in IOTA networks use milestones.

If the node has the history of messages that a milestone references, that milestone is solid. Therefore, nodes know if
they are synchronized if the index of their latest solid milestone is the same as the index of the latest milestone that
it has received.

When a node is synchronized, it then has enough information to decide which transactions it considers confirmed.

## Deciding which messages are confirmed

All messages remain in a pending state until the node is sure of its validity. For a definition of a message,
see [Messages, payloads, and transactions](https://chrysalis.docs.iota.org/guides/dev_guide.html#messages-payloads-and-transactions)
.

However, even when a message is valid, nodes may not be able to make a decision like in the case of a double spend.

When nodes detect double spends they must decide which message to consider confirmed and which one to ignore. Nodes do
this by using consensus rules that are built into their node software using a network protocol.

## Keeping a record of the balances on addresses via `UTXO`

All nodes keep a record of
the [Unspent Transaction Outputs (UTXO)](https://chrysalis.docs.iota.org/guides/dev_guide.html#unspent-transaction-output-utxo)
so they can do the following:

* Check that a transaction is not transferring more IOTA tokens than are available on the address
* Respond to clients' requests for their balance
* Only when a transaction is confirmed, can nodes update their record of balances

## Exposing APIs for clients

Nodes come with two set of low-level APIs:

* HTTP(rest) API
* Event API

> Developers are not needed to communicate with nodes using mentioned low-level API. Developers can successfully leverage [iota client libraries](https://chrysalis.docs.iota.org/libraries/overview.html) that provide a high-level abstraction to all features IOTA nodes provide, either on HTTP API level or Event API level

### HTTP Rest API

The HTTP API allows clients to interact with the Tangle and ask nodes to do the following:

* Get tip messages
* Attach new messages to the Tangle
* Do proof of work
* Get messages from the Tangle

### Event API

The Events API allows clients to poll nodes for new messages and other events that happen on nodes. This type of API is
useful for building applications such as custodial wallets that need to monitor the Tangle for updates to the balances
of certain addresses.

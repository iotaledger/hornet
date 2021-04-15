# Prerequisites
Running a node is the best way to use IOTA. By doing so, you have direct access to the Tangle instead of having to connect to and trust someone else's node, and you help the IOTA network to become more distributed and resilient. This topic provides a checklist of steps for running a reliable and secure node.

## Setting up a reliable architecture
To handle a high rate of transactions per second, nodes need enough computational power to run reliably, including the following minimum requirements:
* A dual-core CPU
* 2 GB RAM
* SSD storage

The amount of storage you need will depend on whether you plan on pruning messages from your local database.

## Securing your device
The security of the device that's running your node is important to stop attackers from gaining access to it.

You should consider doing the following before running a node on your device:
* Securing SSH logins
* Blocking unnecessary ports

### Securing SSH logins
If you log into your device through SSH, you should use measures to protect it from unauthorized access. Many guides have been written about this subject. For more information, see [10 Steps to Secure Open SSH](https://blog.devolutions.net/2017/4/10-steps-to-secure-open-ssh). In addition to that, one can also leverage tools such as Fail2ban to harden security even more.

### Blocking unnecessary ports
Attackers can abuse any open ports on your device.

To secure your device against attacks on unused open ports, you should close all ports except those that are in use.

To do so, you can use a firewall. All operating systems include firewall options. By having a firewall in place you can completely block unused and unnecessary ports.

## Deciding whether to enable remote proof of work
When you're configuring your node, you may have the option to allow it to do proof of work. When this feature is enabled, clients can ask your node to do remote proof of work.

Proof of work takes time and uses your node's computational power. So, you should consider it according to your infrastructure.

## Load balancing
If you run more than one node, it's a best practice to make sure that API requests are distributed among all of them.

To evenly distribute the API requests among all your nodes, you can run a reverse proxy server that will act as a load balancer (HAproxy, Traefik, Nginx, Apache, etc.). This way, you can have one domain name for your reverse proxy server that all nodes will send their API calls to. But, on the backend, the nodes with the most spare computational power will process the request and return the response.

Broadcasted messages are atomic and nodes uses restful api to communicate, so sticky sessions or similar tech is not needed.

---
keywords:
description: This section provides a checklist of steps for running a reliable and secure node.
image: /img/logo/HornetLogo.png

- IOTA Node 
- Hornet Node
- Hornet
- Security
- reference

---


# Security 101

You can follow the checklists below to run your node securely.

:::note

Servers that are reachable from the Internet are a constant target from security challengers. Please follow the security essentials summarized in this article.

:::

## Securing Your Device

The security of the device running your node is of the utmost importance to stop attackers from gaining access to the node.

You should consider [securing SSH logins](#securing-ssh-logins) and [blocking unnecessary ports](#blocking-unnecessary-ports) before running a node on your device.



### Securing SSH logins

You can take measures to protect your device from unauthorized access when logging in through SSH from several readily avilable sources. The [10 Steps to Secure Open SSH](https://blog.devolutions.net/2017/04/10-steps-to-secure-open-ssh) guide and the tools found on [Fail2ban](https://www.fail2ban.org/wiki/index.php/Main_Page) should help strengthen your node's security.

### Blocking Unnecessary Ports

Attackers can abuse any open ports on your device. To secure your device against attacks on unused open ports, you can close all ports except the ones that you are using.

You can use a firewall for port security since all operating systems include firewall options. Using a firewall lets you completely block unused and unnecessary ports.

On cloud platforms such as AWS, Azure, or GCP, you can block ports on VPS networking settings.

## Deciding Whether to Enable Remote Proof of Work

When you are configuring your node, you have the option to allow it to do proof of work (PoW). If you enable this feature, clients can ask your node to remotely do PoW.

PoW takes time and uses your node's computational power. So, consider enabling it according to your infrastructure.

## Load Balancing

If you run more than one node, it is a good practice to make sure that you distribute the API requests among all of them.

To evenly distribute the API requests among all your nodes, you can run a reverse proxy server that will act as a load balancer ([HAProxy](http://www.haproxy.org/), [Traefik](https://traefik.io/), [Nginx](https://www.nginx.com/), [Apache](https://www.apache.org/), etc.). This way, you can have one domain name for your reverse proxy server that all nodes will send their API calls to. On the backend, the nodes with the most spare computational power will process the request and return the response.

Since broadcasted messages are atomic and nodes provide RESTful API to communicate, you will not need sticky sessions or similar technologies.

## Reverse Proxy

We recommend that you use a reverse proxy in front of a node is even if you are deploying a single node. Using a reverse proxy adds an additional security layer that can handle tasks such as:

- IP address filtering. 
- Abuse rate limiting. 
- SSL encrypting.
- Additional authorization layer.

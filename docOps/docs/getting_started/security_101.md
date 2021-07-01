# Security 101

This section provides a checklist of steps for running a reliable and secure node.

:::info
Please note that servers that are reachable from the Internet are a constant target from security challengers.  Please, make sure you the follow minimum security essentials summarized in this article.
:::

## Securing Your Device
The security of the device that is running your node is of the utmost importance to stop attackers from gaining access to the node.

You should consider doing the following before running a node on your device:
* [Securing SSH logins](#securing-ssh-logins).
* [Blocking unnecessary ports](#blocking-unnecessary-ports).

### Securing SSH logins
If you log into your device through SSH, you should take measures to protect it from unauthorized access. Many guides have been written about this subject. For more information, see [10 Steps to Secure Open SSH](https://blog.devolutions.net/2017/4/10-steps-to-secure-open-ssh). In addition to that, you can also leverage tools such as [Fail2ban](https://www.fail2ban.org/wiki/index.php/Main_Page) to further tighten your nodes security.

### Blocking Unnecessary Ports
Attackers can abuse any open ports on your device. To secure your device against attacks on unused open ports, you should close all ports except those that are in use.

You can use a firewall to accomplish this. All operating systems include firewall options. By having a firewall in place, you can completely block unused and unnecessary ports.

On a cloud platforms such as AWS, Azure or GCP, one can block ports on VPS networking settings.

## Deciding Whether to Enable Remote Proof of Work
When you're configuring your node, you have the option to allow it to do proof of work. If you enable this feature, clients can ask your node to do remote proof of work.

Proof of work takes time and uses your node's computational power. So, you should consider enabling it according to your infrastructure.

## Load Balancing
If you run more than one node, it's a good practice to make sure that you distribute the API requests among all of them.

To evenly distribute the API requests among all your nodes, you can run a reverse proxy server that will act as a load balancer ([HAProxy](http://www.haproxy.org/), [Traefik](https://traefik.io/), [Nginx](https://www.nginx.com/), [Apache](https://www.apache.org/), etc.). This way, you can have one domain name for your reverse proxy server that all nodes will send their API calls to. On the backend, the nodes with the most spare computational power will process the request and return the response.

Since broadcasted messages are atomic and nodes provides restful API to communicate, you will not need sticky sessions or similar technologies.

## Reverse Proxy
We recommend that you use a reverse proxy in front of a node is even if you are deploying a single node. Using a reverse proxy adds an additional security layer that can handle tasks such as:

- IP address filtering. 
- Abuse rate limiting. 
- SSL encrypting.
- Additional authorization layer.

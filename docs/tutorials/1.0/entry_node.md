This is a tutorial on how to host an autopeering entry node.

**Note: This tutorial assumes that you are using Ubuntu and our [APT install](./setup.md). The setup process may differ on other OS or install methods.**

### Preparations

- Setup and secure your server (Firewall, SSH-Key,...)
- Install HORNET (don't start it yet!)
- It is recommended to use a domain name for your entry node (you'll need a proxy setup, e.g. nginx). However an IP address will work too.

### Configuration

- Randomly generate a Base58 encoded 256-bit string (seed):
  - Option 1: Generate your seed with a true random number generator (recommended for production usage).
  - Option 2: Use the build in HORNET seed generator: `hornet tool seedgen`.
- Switch on the autopeering mode in your `config.json`:<br>
  `/var/lib/hornet/config.json`
  ```json
  ...
  "network": {
      ...
      "autopeering": {
          "bindAddress": "0.0.0.0:14626",
          "runAsEntryNode": true,
          "seed": "7xFNTP5Fc3wnD78LarNTrvRoKiLESA9qecn3eR5HSVBj"
      }
  }
  ...
  ```
- Start HORNET with `sudo service start hornet && sudo journalctl -n hornet -f`. HORNET will start in an autopeering entry node only mode. Other functions are disabled in this mode.
- In the HORNET logs, you will see something like this:
  ```
  INFO    Autopeering     Autopeering started: ID=a0ba6bf62d6fe911 Address=0.0.0.0:14626/udp PublicKey=9CT42uZC6GetwoT2Jz7Lc5t6LGvXphp9gLvEsdoXEizV
  ```
  Note down your PublicKey.<br>
  **Your entry node address is `<PublicKey>@<your-domain.tld>:<autopeering-port>`**
- Add your entry node address to the `network.autopeering.entryNodes` in your nodes `config.json`
- Done, your nodes should now autopeer with the help of your entry node.

### Monitoring

You can monitor your entry node with the `healthz` API route.<br>
`<your-domain.tld>:<api-port>/healthz`<br>
It will return an http status code 200 if everything is OK.

### Hosting a public entry node

If you want to host an entry node for a public network such as the mainnet, there are a few things to consider:

- You should be experienced in working with servers and server security.
- A domain name is a must have.
- Your domain name should have both, an A (IPv4) and AAAA (IPv6) record.
- Your entry node gets monitored (e.g. with the `healthz` API and an uptime monitor).
- You are able to update your entry node shortly after a new HORNET version gets released.

If these points are not a problem for you, feel free to contact us (e.g. in the [official IOTA Discord `#hornet` channel](https://discord.iota.org/)) to have your entry node added to the official entry nodes!
# Setup a HORNET Node

This is a small tutorial on how to install HORNET using our apt repository or using the RPM package.

### Preparations

- Setup an e.g. Ubuntu (>= 16.04) or CentOS (>= 7) server with at least 1GB RAM
- Optional but recommended: Add a ssh key, activate a firewall ...
- Update your system (Ubuntu) with:<br>
  ```bash
  sudo apt update
  sudo apt upgrade
  ```
- Allow basic ports in your firewall settings (and your router if you run HORNET behind one).<br>
  The following ports are important for a flawless node operation.<br>
  Please note that these ports can be customized in your `config.json`:<br>
  ```
  14626 UDP - Autopeering port
  15600 TCP - Gossip (neighbors) port
  14265 TCP - API port (optional if you don't want to access your node's API from external)
  ```
  Please also have a look at the [configuration documentation](https://github.com/gohornet/hornet/wiki/Configuration)

### Installation (Ubuntu)

- Import the key that is used to sign the release
  ```bash
  wget -qO - https://ppa.hornet.zone/pubkey.txt | sudo apt-key add -
  ```
- Add the HORNET repository to your APT sources<br>
  **Stable:**
  ```bash
  sudo sh -c 'echo "deb http://ppa.hornet.zone stable main" >> /etc/apt/sources.list.d/hornet.list'
  ```
  **Testing (pre-releases):**<br>
  :warning: **Do not use in production, bugs are likely** :warning:
  ```bash
  sudo sh -c 'echo "deb http://ppa.hornet.zone testing main" >> /etc/apt/sources.list.d/hornet.list'
  ```
    **Alpha (pre-releases):**<br>
  :warning: **Only for Comnet, do not use in production, bugs are likely** :warning:
  
  Follow the updates in the #comnet channel if you want to run this version!
  ```bash
  sudo sh -c 'echo "deb http://ppa.hornet.zone alpha main" >> /etc/apt/sources.list.d/hornet.list'
  ```
- Install HORNET:
  ```bash
  sudo apt update
  sudo apt install hornet
  ```

### Installation (CentOS)

- Open [HORNET latest release page](https://github.com/gohornet/hornet/releases/latest)

- Copy the link to the RPM package (e.g. `hornet-0.4.0-x86_64.rpm`)

- Consider validating the RPM package checksum against the corresponding one from the `checksums.txt`

- Install the HORNET RPM package from the releases page, e.g. version v0.4.0:
  ```bash
  sudo yum install https://github.com/gohornet/hornet/releases/download/v0.4.0/hornet-0.4.0-x86_64.rpm
  ```
  or from a file:
  ```bash
  sudo yum install hornet-0.4.0-x86_64.rpm
  ```
- Note: Checksum can be verified if the RPM package and checksum file are present in the same directory, should return a value and the file name:
  ```bash
  grep "^$(sha256sum hornet-0.4.0-x86_64.rpm)$" checksums.txt
  ```

### Setup

- Adapt the settings to your needs (e.g. [setup for comnet](#comnet-community-network-setup))
- Enable the HORNET service:
  ```bash
  sudo systemctl enable hornet.service
  ```
- Start HORNET:
  ```bash
  sudo service hornet start
  ```
- Watch the logs with:
  ```bash
  sudo journalctl -u hornet -f
  ```
- Done

### Operation

- Stop HORNET
  - `sudo service hornet stop`
- Start HORNET
  - `sudo service hornet start`
- Restart HORNET
  - `sudo service hornet restart`
- Check HORNET status
  - `sudo service hornet status`
- Watch the logs
  - `sudo journalctl -u hornet -f`
- Remove the mainnetdb (e.g. in case of a failure):
  1. Stop HORNET
  2. `sudo rm -r /var/lib/hornet/mainnetdb`
  3. Start HORNET

### Configuration

You can find HORNET's configuration files in:<br>
`/var/lib/hornet/`<br>

Additional cli arguments can be set in:<br>
`/etc/default/hornet`<br><br>

If you have modified the `config.json`, you have to restart HORNET:<br>
`sudo service hornet stop && sudo service hornet start`<br>
or<br>
`sudo service hornet restart`

### Comnet (community network) setup

1. Edit `/etc/default/hornet`:
   ```bash
   sudo nano /etc/default/hornet
   ```
   `/etc/default/hornet`:
   ```
   # Add cli arguments to hornet, e.g.:
   # (For the full list of cli options run 'hornet -h')
   OPTIONS="--config config_comnet"
   ```
2. Start HORNET
---
description: Learn how to install and run a Hornet node using Hornet's apt repository using this tutorial. It is recommended for Linux and Ubuntu.
image: /img/logo/HornetLogo.png
keywords:
- IOTA Node 
- Hornet Node
- Linux
- Ubuntu
- apt
- install
- how to

---

# Hornet `apt` Repository (Linux-distro specific)
Hornet developers maintain the Hornet `apt` repository. It installs Hornet as a `systemd` service under a user called `hornet`. 

## Ubuntu/Debian

1. Import the public key that is used to sign the software release:
    ```bash
    wget -qO - https://ppa.hornet.zone/pubkey.txt | sudo apt-key add -
    ```

2. Add the Hornet APT repository to your APT sources:
    ```bash
    sudo sh -c 'echo "deb http://ppa.hornet.zone stable main" >> /etc/apt/sources.list.d/hornet.list'
    ```

3. Update `apt` package lists and install Hornet:
    ```bash
    sudo apt update
    sudo apt install hornet
    ```

4. Enable the `systemd` service:
   ```bash
   sudo systemctl enable hornet.service
   ```

You can find the Hornet configuration files under the `/var/lib/hornet` directory. You can also find more details on how to configure Hornet in the [post installation](post_installation.md) article.

Additionally, the Environment file for configuring multiple default parameters can be found under the 
`/etc/default/hornet` directory.

### Start the Node

You can use  the `systemd` service to start running Hornet on the Mainnet by running the following command:
```bash
sudo service hornet start
```

### Managing the Node

#### Displaying log output

You can display the nodes logs by running the following command:

```bash
journalctl -fu hornet
```

* `-f`: instructs `journalctl` to continue displaying the log to stdout until CTRL+C is pressed
* `-u hornet`: filters the log output by user name

#### Restarting Hornet
You can restart `hornet` by running the following command:

```bash
sudo systemctl restart hornet
```

#### Stopping Hornet
You can stop `hornet` by running the following command:

```bash
sudo systemctl stop hornet
```

:::note

Hornet uses an in-memory cache. To save all data to the underlying persistent storage, a grace period of at least 200 seconds for shutting down is required.

::: 

You can find more details on how to configure Hornet in the [post installation](post_installation.md) article.


# Pre-built Binaries
There are several pre-built binaries of Hornet for major platforms available including some default configuration JSON files.

:::note

All installation methods mentioned in this article from this point should be considered advanced for production use as you will have to prepare a system environment to run the executable as a service (in daemon mode), using `systemd` or `supervisord`.

:::

1. Download the latest release compiled for your system from [GitHub release assets](https://github.com/iotaledger/hornet/releases):

   ```bash
   curl -LO https://github.com/iotaledger/hornet/releases/download/v1.0.5/HORNET-1.0.5_Linux_x86_64.tar.gz
   ```

   Please make sure to download the binaries for your system:
   
   * `HORNET-X.Y.Z_Linux_x86_64.tar.gz`: standard 64-bit-linux-based executable, such as Ubuntu, Debian, etc.
   * `HORNET-X.Y.Z_Linux_arm64.tar.gz`: executable for 64bit ARM based systems.
   * `HORNET-X.Y.Z_Windows_x86_64.zip`: executable for Windows 10-64-bit-based systems.
   * `HORNET-X.Y.Z_macOS_x86_64.tar.gz`: executable for macOS.

2. Extract the files in a folder of your choice (for example `/opt` on Linux):

   ```bash
   tar -xf HORNET-1.0.5_Linux_x86_64.tar.gz
   ```

3. Once you have extracted the files, you get a main executable file. You can also find sample [configuration](post_installation.md) JSON files available in the archive (tar or zip).

You can run Hornet using `--help` to get all executable-related arguments by running:
   
```bash
./hornet --help
```

You can double-check that you have version 0.6.0+ deployed by running:
   
```bash
./hornet --version
```

You can run Hornet using default settings by running:

```bash
./hornet
```

## Example of Systemd Unit File

The following is an example of a `systemd` unit file. If you have extracted the Hornet executable to `/opt/hornet` together with configuration files, this file should work as is. If you have extracted the Hornet executable in another location, please review the configuration and update it accordingly.

```plaintext
[Unit]
Description=Hornet
Wants=network-online.target
After=network-online.target

[Service]
StandardOutput=syslog
StandardError=syslog
SyslogIdentifier=hornet
PrivateDevices=yes
PrivateTmp=yes
ProtectSystem=full
ProtectHome=yes

User=hornet
WorkingDirectory=/opt/hornet
TimeoutSec=1200
Restart=always
ExecStart=/opt/hornet/hornet

[Install]
WantedBy=multi-user.target
```

# Build From Source

1. Install Go:

You can find installation instructions in the [official Go documentation](https://golang.org/doc/install).

2. Install dependencies: `Git` and `build-essentials`:
   
   ```bash
   sudo apt update
   sudo apt install git build-essential
   ```

3. Check the golang/git version:

   ```bash
   go version
   git --version
   ```
   Make sure you have the latest version from https://golang.org/dl/.

4. Clone the Hornet source code from GitHub:
   
   ```bash
   git clone https://github.com/iotaledger/hornet.git && cd hornet && git checkout mainnet
   ```

5. Build the Hornet:
   ```bash
   ./scripts/build_hornet_rocksdb_builtin.sh
   ```
   * This command will build Hornet based on the latest commit from the currently chosen branch.
   * This may take a couple of minutes.
   
6. Once it is compiled, then the executable file named `hornet` should be available in the current directory. You can check the version by running:

   ```bash
   ./hornet --version
   ```

   Example of version:
   ```plaintext
   HORNET c37bbe0f
   ```
   For self-compiled binaries, the version is the short commit `sha`, which you can use to check which commit the given version is compiled against.

You can run Hornet using `--help` to get all executable-related arguments by running:
   
```bash
./hornet --help
```

You can double-check that you have version 0.6.0+ deployed by running:
   
```bash
./hornet --version
```

You can run Hornet using default settings by running:

```bash
./hornet
```

Using this method, you have to make sure the executable runs in a daemon mode using for example `systemd`.


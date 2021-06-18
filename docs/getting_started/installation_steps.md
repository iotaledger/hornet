# Hornet `apt` Repository (Linux-distro specific)
Hornet `apt` repository is maintained by the Hornet developers. It installs Hornet as a `systemd` service under a user called `hornet`.

*Ubuntu/Debian*

**Import the public key that is used to sign the software release:**
```bash
wget -qO - https://ppa.hornet.zone/pubkey.txt | sudo apt-key add -
```

**Add the Hornet APT repository to your APT sources:**
```bash
sudo sh -c 'echo "deb http://ppa.hornet.zone stable main" >> /etc/apt/sources.list.d/hornet.list'
```

**Update `apt` package lists and install Hornet:**
```bash
sudo apt update
sudo apt install hornet
```

**Enable the `systemd` service:**
```bash
sudo systemctl enable hornet.service
```

The Hornet configuration files are located under the `/var/lib/hornet` directory. See more details on how to configure Hornet under the [post installation](../post_installation/post_installation.md) chapter.

Environment file to configure multiple default parameters are located under the `/etc/default/hornet` directory.

**Start the node;** use `systemd` service to start running Hornet on the Mainnet:
```bash
sudo service hornet start
```

### Managing the Node
**Displaying log output:**
```bash
journalctl -fu hornet
```
* `-f`: instructs `journalctl` to continue displaying the log to stdout until CTRL+C is pressed
* `-u hornet`: filter log output by user name

**Restarting Hornet:**
```bash
sudo systemctl restart hornet
```

**Stopping Hornet:**
```bash
sudo systemctl stop hornet
```
:::info
Hornet uses an in-memory cache, so it is necessary to provide a grace period while shutting it down (at least 200 seconds) in order to save all data to the underlying persistent storage.
::: 

See more details on how to configure Hornet under the [post installation](../post_installation/post_installation.md) chapter.

-------

# Pre-built Binaries
There are several pre-built binaries of Hornet for major platforms available including some default configuration JSON files.

This method is considered a bit advanced for production use since you have to usually prepare a system environment in order to run the given executable as a service (in a daemon mode) via `systemd` or `supervisord`.

**Download the latest release compiled for your system from [GitHub release assets](https://github.com/gohornet/hornet/releases), for ex:**

```bash
curl -LO https://github.com/gohornet/hornet/releases/download/v0.6.0/HORNET-0.6.0_Linux_x86_64.tar.gz
```
Some navigation hints:
* `HORNET-X.Y.Z_Linux_x86_64.tar.gz`: standard 64-bit-linux-based executable, such as Ubuntu, Debian, etc.
* `HORNET-X.Y.Z_Linux_arm64.tar.gz`: executable for Raspberry Pi 4
* `HORNET-X.Y.Z_Windows_x86_64.zip`: executable for Windows 10-64-bit-based systems
* `HORNET-X.Y.Z_macOS_x86_64.tar.gz`: executable for macOS

**Extract the files in a folder of your choice (for ex. `/opt` on Linux), for ex:**
```bash
tar -xf HORNET-0.6.0_Linux_x86_64.tar.gz
```
* Once extracted, you get a main executable file
* There are also sample [configuration](../post_installation/post_installation.md) JSON files available in the archive (tar or zip)

**Run Hornet using `--help` to get all executable-related arguments:**
```bash
./hornet --help
```

*Also double check that you have version 0.6.0+ deployed:*
```bash
./hornet --version
```

**Run Hornet using default settings:**
```bash
./hornet
```

Using this method, you have to make sure the executable runs in a daemon mode using for example `systemd`.

::: 
Hornet uses an in-memory cache, so it is necessary to provide a grace period while shutting it down (at least 200 seconds) in order to save all data to the underlying persistent storage.
:::

See more details on how to configure Hornet under the [post installation](../post_installation/post_installation.md) chapter.

### Example of Systemd Unit File
Assuming the Hornet executable is extracted to `/opt/hornet` together with configuration files, please find the following example of a `systemd` unit file:

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


----------------



# Build From Source
This method is considered a bit advanced for production use since you usually have to prepare a system environment in order to run the given executable as a service (in a daemon mode) via `systemd` or `supervisord`.

**Install Go:**

Install [Go](https://golang.org/doc/install)

**Install dependencies: Git and build-essentials:**
```bash
sudo apt update
sudo apt install git build-essential
```

**Check the golang/git version:**
```bash
go version
git --version
```
Make sure you have the latest version from https://golang.org/dl/

**Clone the Hornet source code from GitHub:**
```bash
git clone https://github.com/gohornet/hornet.git && cd hornet
```

**Build the Hornet:**
```bash
./build_hornet_rocksdb_builtin.sh
```
* it builds Hornet based on the latest commit from `main` branch
* it takes a couple of minutes

Once it is compiled, then the executable file named `hornet` should be available in the current directory:
```bash
./hornet --version
```

Example of version:
```plaintext
HORNET 0.6.0-31ad46bb
```
* there is also short commit `sha` added to be sure what commit the given version is compiled against

**Run Hornet using `--help` to get all executable-related arguments:**
```bash
./hornet --help
```

**Run Hornet using a default settings:**
```bash
./hornet
```

Using this method, you have to make sure the executable runs in a daemon mode using for example `systemd`.

:::info
Hornet uses an in-memory cache, so it is necessary to provide a grace period while shutting it down (at least 200 seconds) in order to save all data to the underlying persistent storage.
:::

See more details on how to configure Hornet under the [post installation](../post_installation/post_installation.md) chapter.

### Example of Systemd Unit File
Assuming the Hornet executable is extracted to `/opt/hornet` together with configuration files, please find the following example of a `systemd` unit file:

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

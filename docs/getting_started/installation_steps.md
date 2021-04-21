# Hornet `apt` repository (Linux-distro specific)
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

### Managing the node
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

> Please note: Hornet uses an in-memory cache, so it is necessary to provide a grace period while shutting it down (at least 200 seconds) in order to save all data to the underlying persistent storage.

See more details on how to configure Hornet under the [post installation](../post_installation/post_installation.md) chapter.


-------


# Docker image
Prepared Hornet Docker images (amd64/x86_64 architecture) are available at [gohornet/hornet](https://hub.docker.com/r/gohornet/hornet) Docker hub.

Make sure that you've installed Docker on your machine before trying to use the Docker images. (Follow this [link](https://docs.docker.com/engine/install/) for install instructions).

**Hornet uses JSON configuration files which can be downloaded from the repository on GitHub:**
```bash
curl -LO https://raw.githubusercontent.com/gohornet/hornet/master/config.json
curl -LO https://raw.githubusercontent.com/gohornet/hornet/master/peering.json
```
See more details on how to configure Hornet under the [post installation](../post_installation/post_installation.md) chapter.

**Create empty directories for the database, snapshots and set user permission to them:**
```bash
mkdir mainnetdb && sudo chown 39999:39999 mainnetdb
mkdir -p snapshots/mainnet && sudo chown 39999:39999 snapshots -R
```
* The Docker image runs under user with uid `39999`, so it has a full permission to the given directory

**Pull the latest image from `gohornet/hornet` public Docker hub registry:**
```bash
docker pull gohornet/hornet:latest

3 directories, 2 files
```

**Start the node;** using `docker run` command:
```bash
docker run --rm -d --restart always -v $(pwd)/config.json:/app/config.json:ro -v $(pwd)/snapshots/mainnet:/app/snapshots/mainnet -v $(pwd)/mainnetdb:/app/mainnetdb --name hornet --net=host gohornet/hornet:latest

**Removing container:**
```bash
docker container rm hornet
```

See more details on how to configure Hornet under the [post installation](../post_installation/post_installation.md) chapter.


--------


# Pre-built binaries
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
* Once extracted, you get a main executable file, for example `hornet` for linux, or `hornet.exe` for Windows
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

> Please note: Hornet uses an in-memory cache, so it is necessary to provide a grace period while shutting it down (at least 200 seconds) in order to save all data to the underlying persistent storage.

See more details on how to configure Hornet under the [post installation](../post_installation/post_installation.md) chapter.

### Example of systemd unit file
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



# Build from source
This method is considered a bit advanced for production use since you usually have to prepare a system environment in order to run the given executable as a service (in a daemon mode) via `systemd` or `supervisord`.

*Ubuntu/Debian*

*There is usually quite old version of Go language in standard `apt` repositories available, so it is better to add `golang-backports` PPA to get the latest version.*

**Install dependencies: Go, git and build-essentials:**
```bash
sudo add-apt-repository ppa:longsleep/golang-backports
sudo apt update
sudo apt install golang-go git build-essential
```

**Check the golang/git version:**
```bash
go version
git --version
```
You should see Golang version at least `1.15.0`.

**Clone the Hornet source code from GitHub:**
```bash
git clone https://github.com/gohornet/hornet.git && cd hornet
```

**Build the Hornet:**
```bash
./scripts/build_hornet.sh
```
* it builds Hornet based on the latest commit from `master` branch
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

> Please note: Hornet uses an in-memory cache, so it is necessary to provide a grace period while shutting it down (at least 200 seconds) in order to save all data to the underlying persistent storage.

See more details on how to configure Hornet under the [post installation](../post_installation/post_installation.md) chapter.

### Example of systemd unit file
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

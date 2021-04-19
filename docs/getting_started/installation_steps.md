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

In `/var/lib/hornet` directory, there all the configuration files. See more details regarding the configuration in [post installation](../post_installation/config.md) chapter.

In `etc/default/hornet` file, there is environment file that can be used to customize default parameters.

**Start the node;** use `systemd` service to start running Hornet on the Mainnet:
```bash
sudo service hornet start
```

### Managing node
**Displaying log output:**
```bash
journalctl -fu hornet
```
* `-f`: instructs `journalctl` to continue displaying the log to stdout until CTRL+C is pressed
* `-u hornet`: it filters log output by user name

**Restarting Hornet:**
```bash
sudo systemctl restart hornet
```

**Stopping Hornet:**
```bash
sudo systemctl stop hornet
```

> Please note: Hornet uses an in-memory cache and so it is necessary to provide a grace period while shutting it down (at least 200 secs) in order to save all data to an underlying persistent storage

Please continue to [post-installation steps](../post_installation/post_installation.md) to properly configure Hornet.


-------


# Docker image
Prepared Hornet docker images (amd64/x86_64 architecture) are available at [gohornet/hornet](https://hub.docker.com/r/gohornet/hornet) docker hub.

There has to be installed `docker engine` on a host in order to run docker images. Please, follow an [official installation guide](https://docs.docker.com/engine/install/) in order to install `docker engine`.

**Hornet uses `json` configuration files and those can be downloaded from the source code repo:**
```bash
curl -LO https://raw.githubusercontent.com/gohornet/hornet/master/config.json
curl -LO https://raw.githubusercontent.com/gohornet/hornet/master/peering.json
```
See more details regarding the configuration in [post installation](../post_installation/config.md) chapter.

**Create the directory for the database and set user permission to it:**
```bash
mkdir mainnetdb && sudo chown 39999:39999 mainnetdb
```
* the docker image runs under user with uid `39999`, and so it has a full permission to the given directory

**Create the directory for the snapshots and set user permission to it:**
```bash
mkdir -p snapshots/mainnet && sudo chown 39999:39999 snapshots -R
```
* the docker image runs under user with uid `39999`, and so it has a full permission to the given directory

**Pull the latest image from `gohornet/hornet` public docker hub registry:**
```bash
docker pull gohornet/hornet:latest
```

So basically there should be the following files and directories created in the current directory:
```plaintext
.
├── config.json
├── peering.json
├── mainnetdb       <DIRECTORY>
└── snapshots       <DIRECTORY>
    └── mainnet     <DIRECTORY>

3 directories, 2 files
```

**Start the node;** using `docker run` command:
```bash
docker run --rm -d --restart always -v $(pwd)/config.json:/app/config.json:ro -v $(pwd)/snapshots/mainnet:/app/snapshots/mainnet -v $(pwd)/mainnetdb:/app/mainnetdb --name hornet --net=host gohornet/hornet:latest
```
* `$(pwd)`: stands for the current directory
* `-d`: instructs docker to run the container instance in a detached mode (daemon).
* `--restart always`: instructs docker the given container is restarted after docker reboot
* `--name hornet`: a name of the running container instance. You can refer to the given container by this name
* `--net=host`: instructs docker to use directly network on host (so the network is not isolated). The best is to run on host network for better performance. It also means it is not necessary to specify any ports. Ports that are opened by container are opened directly on the host
* `-v $(pwd)/config.json:/app/config.json:ro`: it maps the local `config.json` file into the container in `readonly` mode
* `-v $(pwd)/snapshots/mainnet:/app/snapshots/mainnet`: it maps the local `snapshots` directory into the container
* `-v $(pwd)/mainnetdb:/app/mainnetdb`: it maps the local `mainnetdb` directory into the container
* all mentioned directories are mapped to the given container and so the Hornet in container persists the data directly to those directories

### Managing node
**Displaying log output:**
```bash
docker log -f hornet
```
* `-f`: instructs docker to continue displaying the log to stdout until CTRL+C is pressed

**Restarting Hornet:**
```bash
docker restart hornet
```

**Stopping Hornet:**
```bash
docker stop --time 200 hornet
```
* `--time 200`: instructs docker to wait for a grace period before shutting down

> Please note: Hornet uses an in-memory cache and so it is necessary to provide a grace period while shutting it down (at least 200 secs) in order to save all data to an underlying persistent storage

**Removing container:**
```bash
docker container rm hornet
```

Please continue to [post-installation steps](../post_installation/post_installation.md) to properly configure Hornet.


--------


# Pre-built binaries
Pre-built binaries are a great way how to get the latest single compiled executable in a single file including some default configuration `json` files.

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
* Once extracted, you get a main executable file, for ex `hornet` for linux, or `hornet.exe` for Windows
* There are also sample [configuration](../post_installation/config.md) `json` files available in the archive

**Run Hornet using `--help` to get all executable-related arguments:**
```bash
./hornet --help
```

*Also double check that you have version 0.6.0+ deployed:*
```bash
./hornet --version
```

**Run Hornet using a default settings:**
```bash
./hornet
```

Using this method, you have to make sure the executable runs in a daemon mode using for example `systemd`.

> Please note: Hornet uses an in-memory cache and so it is necessary to provide a grace period while shutting it down (at least 200 secs) in order to save all data to an underlying persistent storage

Please continue to [post-installation steps](../post_installation/post_installation.md) to properly configure Hornet.

### Example of systemd unit file
Assuming Hornet binary is extracted to `/opt/hornet` together with configuration files, `systemd` unit file may look:

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



# Build from the source code
This method is considered a bit advanced for production use since you have to usually prepare a system environment in order to run the given executable as a service (in a daemon mode) via `systemd` or `supervisord`.

*Ubuntu/Debian*

*There is usually quite old version of Go language in a standard `apt` repositories available, and so it is better to add `golang-backports` PPA to get the latest version.*

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
* it builds the Hornet based on the latest commit from `master` branch
* it takes a couple of minutes

Once it is compiled, then the executable file named `hornet` should be available in the current directory:
```bash
./hornet --version
```

Example of version:
```plaintext
HORNET 0.5.6-31ad46bb
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

> Please note: Hornet uses an in-memory cache and so it is necessary to provide a grace period while shutting it down (at least 200 secs) in order to save all data to an underlying persistent storage

Please continue to [post-installation steps](../post_installation/post_installation.md) to properly configure Hornet.

### Example of systemd unit file
Assuming Hornet binary is extracted to `/opt/hornet` together with configuration files, `systemd` unit file may look:

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

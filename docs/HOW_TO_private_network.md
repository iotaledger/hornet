
# **How to set up a private Tangle based only on HORNET nodes (including Coordinator)**

The following procedure explain how to setup a private Tangle based only on HORNET nodes (including Coordinator).
The basic architecture to implement this private tangle is composed by:
1. The Coordinator node (composed by regular Hornet node, working with Compass)
2. One or many other network nodes (regular hornet nodes)

This procedure has been successfully tested / implemented in a Contabo VPS based on LINUX UBUNUTU 18.04 LTS .

All the operation have been performed using ROOT user.

## **1. Install all requirements**

First of all, we need to install all requirements.
Install basic packages:

`sudo apt-get install pkg-config zip g++ zlib1g-dev unzip python python3 apt-transport-https ca-certificates curl`

Install JAVA 8 OpenJDK:

`sudo apt-get install software-properties-common`

`sudo add-apt-repository universe`

`sudo apt-get update`

`sudo apt-get install openjdk-8-jdk`

Install DOCKER: 

`curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -`

`sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu bionic stable"`

`sudo apt update`

`sudo apt install docker-ce`

Install JQ tool to format JSON data:

`sudo apt install jq`

Install BAZEL 0.29.1:
`cd /root`

`wget https://github.com/bazelbuild/bazel/releases/download/0.29.1/bazel-0.29.1-installer-linux-x86_64.sh`

`chmod +x bazel-0.29.1-installer-linux-x86_64.sh`

`./bazel-0.29.1-installer-linux-x86_64.sh --user`

To set up the environment for BAZEL, edit the bashrc file:

`sudo nano ~/.bashrc`

Add the following line:

`export PATH="$PATH:$HOME/bin"`

Reload bashrc:

`source ~/.bashrc`


## **2. Install Hornet node**
Download Hornet release 0.2.9:
`mkdir GoHornet`

`cd GoHornet`

`wget https://github.com/gohornet/hornet/releases/download/v0.2.9/HORNET-0.2.9_Linux_x86_64.tar.gz`

`tar xfv HORNET-0.2.9_Linux_x86_64.tar.gz`

Create a systemd service to START / STOP the Hornet node:

`cd etc/systemd/system`

`touch GoHornet.service`

`sudo nano GoHornet.service`

Add all this lines in the GoHornet.service file:

`[Unit]`

`Description=GoHornet service`

`After=multi-user.target`

`[Service]`

`Type=simple`

`ExecStart=/root/GoHornet/HORNET-0.2.9_Linux_x86_64/hornet -c config`

`WorkingDirectory=/root/GoHornet/HORNET-0.2.9_Linux_x86_64`

`Restart=on-abort`

`[Install]`

`WantedBy=multi-user.target`

Press CTRL + X to save and exit; load the new service:

`systemctl daemon-reload`

`systemctl enable GoHornet`

The next steps of node configuration will be completed later.


## **3. Install Compass code**
`cd /root`

`git clone https://github.com/iotaledger/compass.git`


## **4. Generate a seed for the Coordinator**

`cat /dev/urandom |LC_ALL=C tr -dc 'A-Z9' | fold -w 81 | head -n 1`

In this case, this is the generated seed:

`NYXW9AXDUYTSAJEDKRKGBRFFIOBWHPYSCPCMTG9UMBWSYPPANELSPSGZEDWPTE9RQFKDENTLXHSYZMQQZ`

## **5. Generate the Merkle tree**
We generate the Merkle Tree which will be stored under data/layers folder in the Compass working directory.

First, we need to set up the correct information in the Compass configuration file (a configuration file used to generate the Merkle Tree and to set up the Coordinator):

`cd compass/docs/private_tangle`

Copy the example configuration file and rename it:
`cp config.example.json config.json`

Edit config.json file:
`sudo nano config.json file`

Replce the existing example seed with the coordinator seed you generated :
`"seed": "NYXW9AXDUYTSAJEDKRKGBRFFIOBWHPYSCPCMTG9UMBWSYPPANELSPSGZEDWPTE9RQFKDENTLXHSYZMQQZ"`

Replace thesigMode with KERL:

`"sigMode": "KERL"`

Replace Depth with 16:

`"depth": 16`

Build the Layer Calculator using Bazel (this operation takes some time):

`bazel run //docker:layers_calculator`

Compute the Merkle tree:

`sudo ./01_calculate_layers.sh`

The elaboration will start and, until it's finished, only this message will be displayed (it will take about 10 minutes in a 4 core VPS):

`INFO org.iota.compass.LayersCalculator - Calculating 65536 addresses.`

At the end of the elaboration, you'll find this message:

`Successfully wrote Merkle Tree with root: UAZCCTNPNHWGXRPEWWTTFH9QLLDJSQOGTLCYYOWOWWOJVRBHTPQECFKDYYQDRUFMGKVGYISMUJ9JEEBYA`

This is the coordinator address, which will be used to:
- Assign all prIOTA (Private Iota :-) ) tokens at genesis
- Identify the address which will be generating all the milestones

## **6. Configure the Coordinator node**


``




``
``
``
``
``
``

``
``
``
``
``
``

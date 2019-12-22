
# **How to set up a private Tangle exclusively based on HORNET nodes (including Coordinator)**

The following procedure explains how to setup a private Tangle exclusively based on HORNET nodes (including Coordinator).

The basic architecture required to implement this private tangle is composed by:
1. The Coordinator node (composed by regular Hornet node, working with Compass)
2. One or many other network nodes (regular hornet nodes)

This procedure has been successfully tested / implemented in a Contabo VPS based on LINUX UBUNUTU 18.04 LTS .

All the operations have been performed using ROOT user.

&nbsp;
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


&nbsp;
## **2. Install Hornet node**
Download Hornet release 0.2.9:

`cd /root`

`mkdir GoHornet`

`cd GoHornet`

`wget https://github.com/gohornet/hornet/releases/download/v0.2.9/HORNET-0.2.9_Linux_x86_64.tar.gz`

`tar xfv HORNET-0.2.9_Linux_x86_64.tar.gz`

Create a systemd service to START / STOP the Hornet node:

`cd /etc/systemd/system`

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

**DO NOT ACTIVATE THE NODE, YET.**

The next steps of node configuration will be completed later.


&nbsp;
## **3. Install Compass code**

`cd /root`

`git clone https://github.com/iotaledger/compass.git`


&nbsp;
## **4. Generate a seed for the Coordinator**

`cat /dev/urandom |LC_ALL=C tr -dc 'A-Z9' | fold -w 81 | head -n 1`

In this case, this is the generated seed:

`NYXW9AXDUYTSAJEDKRKGBRFFIOBWHPYSCPCMTG9UMBWSYPPANELSPSGZEDWPTE9RQFKDENTLXHSYZMQQZ`

&nbsp;
## **5. Generate the Merkle tree**

We generate the Merkle Tree which will be stored under data/layers folder in the Compass working directory.

First, we need to set up the correct information in the Compass configuration file (a configuration file used to generate the Merkle Tree and to set up the Coordinator):

`cd /root/compass/docs/private_tangle`

Copy the example configuration file and rename it:

`cp config.example.json config.json`

Edit config.json file:

`sudo nano config.json file`

Replace the example seed with the coordinator seed you generated :

`"seed": "NYXW9AXDUYTSAJEDKRKGBRFFIOBWHPYSCPCMTG9UMBWSYPPANELSPSGZEDWPTE9RQFKDENTLXHSYZMQQZ"`

Set the sigMode to KERL:

`"sigMode": "KERL"`

Set depth parameter to 16:

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
- Identify the address which will be generating all milestones


&nbsp;
## **6. Configure and run the Hornet node**
Open the configuration file of the Hornet node (config.json):

`cd /root/GoHornet/HORNET-0.2.9_Linux_x86_64`

`sudo nano config.json`


Modify the following parameters:

`"compass": { "loadLSMIAsLMI": true }`

`"dashboard": { "host": "0.0.0.0", `

`"db": { "path": "../mainnetdb" }`

`"localsnapshots": { "path": "" }`

And in the milestone parameters section:

`"coordinator": "UAZCCTNPNHWGXRPEWWTTFH9QLLDJSQOGTLCYYOWOWWOJVRBHTPQECFKDYYQDRUFMGKVGYISMUJ9JEEBYA"`

`"coordinatorsecuritylevel": 1`

`"numberofkeysinamilestone": 16`

`"protocol": { "mwm": 9 }`


Remove all neighbors, unless you already identified a neighbors your want to connect.

Press CTRL + X to save and exit.

Generate a snapshot.txt file, in the same folder where the hornet node is:

`touch snapshot.txt`

`sudo nano snapshot.txt`

Add the following line, which simply contains the address of the coordinator and the number of IOTA tokens which will be generated / assigned:

`UAZCCTNPNHWGXRPEWWTTFH9QLLDJSQOGTLCYYOWOWWOJVRBHTPQECFKDYYQDRUFMGKVGYISMUJ9JEEBYA;2779530283277761`

Run the Hornet node:

`systemctl start GoHornet`

Check if the node has been activated on your browser:

`http://YOUR_NODE_IP_ADDRESS:8081`

The node should be active but NOT SYNCED ( LSMI/LMI: 0 / 0 )


&nbsp;
## **7. Build and run the Coordinator (Compass)**
Build the Coordinator using Bazel (this operation could take some time):

`cd /root/compass`

`bazel run //docker:coordinator`

Bootstrap the Coordinator (the -bootstrap parameter is required only at first booststrap; on the following activation of the Coordinator, use the command below with only -broadcast parameter):

`cd /root/compass/docs/private_tangle`

`sudo ./03_run_coordinator.sh -bootstrap -broadcast`

Instantly, the Coordinator send the first milestone to the Hornet node.

On the webpage of the Hornet node, you'll see:

`LSMI/LMI: 1 / 1 - SYNCED`

Your private Tangle Hornet coordinator is now up and running.


&nbsp;
## **8. Run the Coordinator (Compass) as a service, after the bootstrap**

Create a systemd service to START / STOP the Coordinator:

`cd /etc/systemd/system`

`touch Coordinator.service`

`sudo nano Coordinator.service`

Add all this lines in the GoHornet.service file:

`[Unit]`

`Description=Coordinator service`

`After=multi-user.target`

`[Service]`

`Type=simple`

`ExecStart=/root/compass/docs/private_tangle/03_run_coordinator.sh -broadcast`

`WorkingDirectory=/root/compass/docs/private_tangle`

`Restart=on-abort`

`[Install]`

`WantedBy=multi-user.target`

Press CTRL + X to save and exit; load the new service:

`systemctl daemon-reload`

`systemctl enable Coordinator`

Activate the Coordinator:

`systemctl start Coordinator`



&nbsp;
## **9. Install, configure and run all the other Hornet nodes in your private network**
As soon as your Hornet coordinator is up and running, you can bootstrap all the other Hornet nodes in your IOTA private network.

Please follow all the instructions on points N°2 and N°6 above, with only two differences:

1. The compass parameter have to be set to FALSE 

- `"compass": { "loadLSMIAsLMI": false }`

2. Under NEIGHBORS section, add the Coordinator Hornet node as neighbors, in order to allow the synchronization

Then, bootstrap all the nodes of the network and start SPAMMING transactions!

**After every node shutdown, you need to delete the mainnetdb folder, in order to force the synchronization process.**
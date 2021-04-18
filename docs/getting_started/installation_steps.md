# Installation steps

## `hornet` apt repository (Linux-distro specific)
Ubuntu/Debian:



## docker image



## pre-built binaries
Pre-built binaries are a great way how to get the latest single compiled executable in a single file including some default configuration `json` files.

Download the latest release compiled for your system from [GitHub release assets](https://github.com/gohornet/hornet/releases), for ex:

```bash
curl -L --output HORNET-0.5.6_Linux_x86_64.tar.gz https://github.com/gohornet/hornet/releases/download/v0.5.6/HORNET-0.5.6_Linux_x86_64.tar.gz
```
Some navigation hints:
* `HORNET-X.Y.Z_Linux_x86_64.tar.gz`: standard 64-bit-linux-based executable, such as Ubuntu, Debian, etc.
* `HORNET-X.Y.Z_Linux_arm64.tar.gz`: executable for Raspberry Pi 4
* `HORNET-X.Y.Z_Windows_x86_64.zip`: executable for Windows 10-64-bit-based systems
* `HORNET-X.Y.Z_macOS_x86_64.tar.gz`: executable for macOS

Extract the files in a folder of your choice (for ex. `/opt` on Linux), for ex:
```bash
tar -xf HORNET-0.5.6_Linux_x86_64.tar.gz
```
* Once extracted, you get a main executable file, for ex `hornet` for linux, or `hornet.exe` for Windows
* There are also sample [configuration](../post_installation/config.md) `json` files available

Run Hornet using `--help` to get all executable-related arguments:
```bash
./hornet --help
```

Please continue to [post-installation steps](../post_installation/post_installation.md) to properly configure Hornet.


## build from the source code


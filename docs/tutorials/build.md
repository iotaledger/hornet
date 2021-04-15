This is a small tutorial on how to build HORNET.

**Note: This tutorial assumes that you are using Ubuntu. The setup and build process may differ on other OS.**

### Preparations

- Install go1.16: See [golang.org](https://golang.org/doc/install) for more information.
  ```bash
    wget https://golang.org/dl/go1.16.linux-amd64.tar.gz
    tar -C /usr/local -xzf go1.16.linux-amd64.tar.gz
  ```
- Put this line to modify PATH in ~/.profile
  ```bash
    export PATH=$PATH:/usr/local/go/bin
  ```
- Check if Go is installed correctly:
  ```bash
  go version
  ```
  The output should be something like this:
  ```bash
  go version go1.16 linux/amd64
  ```
- Install the git and build-essential packages (if not already done):
  ```bash
  sudo apt install git build-essential
  ```

### Build HORNET

- Clone HORNET with:
  ```bash
  git clone https://github.com/gohornet/hornet.git
  ```
- Change to the cloned `hornet` directory
  ```bash
  cd hornet
  ```
- Checkout the develop branch to build HORNET for Chrysalis (IOTA 1.5) (optional if you want to build the latest HORNET release for the current Network (IOTA 1.0), please use the `master` branch):
  ```bash
  git checkout develop
  ```
- Build HORNET
  ```bash
  go build
  ```

- Done, now you should be able to start HORNET.
  Test, if the build was successful:
  ```bash
  ./hornet --version
  ```
  If HORNET prints out its version, the build was successful.

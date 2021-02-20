This is a small tutorial on how to build HORNET if you don't want to use our [pre-built binaries](https://github.com/gohornet/hornet/releases). 

**Note: This tutorial assumes that you are using Ubuntu. The setup and build process may differ on other OS.**

### Preparations

- Install Go1.14:
  ```bash
     sudo add-apt-repository ppa:longsleep/golang-backports
     sudo apt update
     sudo apt install golang-go
  ```
- Check if Go is installed correctly:
  ```bash
  go version
  ```
  The output should be something like this:
  ```bash
  go version go1.14 linux/amd64
  ```
- Install git (if not already done):
  ```bash
  sudo apt install git
  ```
- Install `build-essential` (optional, needed for optimized PoW)
  ```bash
  sudo apt install build-essential
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
- Checkout the branch you want to build (optional if you want to build the latest HORNET release (master)):
  <br>As an example we want to build HORNET from the `develop` branch
  ```bash
  git checkout develop
  ```
- Build HORNET
  without optimized PoW:
  ```bash
  go build
  ```
  **or** with optimized PoW (**only available for newer x86_64 (amd64) systems**):
  ```bash
  go build -tags=pow_avx
  ```
- Done, now you should be able to start HORNET.
  Test, if the build was successful:
  ```bash
  ./hornet --version
  ```
  If HORNET prints out its version, the build was successful.
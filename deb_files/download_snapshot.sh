#!/bin/bash

set -e
set -u

REMOTE_FILE="https://dbfiles.iota.org/mainnet/hornet/latest-export.gz.bin"

# Download with timestamping (only download if remote file is newer)
wget -N -q ${REMOTE_FILE}

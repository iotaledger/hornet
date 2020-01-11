#!/bin/bash
set -e

REMOTE_FILE="https://dbfiles.iota.org/mainnet/hornet/latest-export.gz.bin"

# Download with timestamping (only download if remote file is newer)
echo "Checking for the latest snapshot file..."
echo "This may take a while"
wget -N -q "${REMOTE_FILE}"

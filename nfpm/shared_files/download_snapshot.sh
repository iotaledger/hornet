#!/usr/bin/env bash

# Set default if none set.
REMOTE_FILE="${1:-https://dbfiles.iota.org/mainnet/hornet/latest-export.gz.bin}"

# Download with timestamping (only download if remote file is newer)
echo "Checking for the latest snapshot file..."
echo "This may take a while"

# Capture only stderr
WGET_STDERR=$( { wget -N "${REMOTE_FILE}"; } 2>&1 >/dev/null )
if [ $? -ne 0 ]
then
    >&2 echo -e "Downloading latest snapshot file failed:\n${WGET_STDERR}\n"
    exit 1
fi

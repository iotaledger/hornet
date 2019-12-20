#!/bin/bash

set -e
set -u

finalurl() { curl --silent --location --head --output /dev/null --write-out '%{url_effective}' -- "$@"; }

REMOTE_FILE=$(finalurl https://dbfiles.iota.org/mainnet/hornet/latest-export.gz.bin)

# Download with timestamping (only download if remote file is newer)
wget -N "${REMOTE_FILE}"

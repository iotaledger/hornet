#!/bin/bash
rm -rf alphanetdb2
go run -tags "pow_avx" main.go -c config_alphanet2.json \
--p2p.bindMultiAddresses="/ip4/127.0.0.1/tcp/15601" \
--p2p.peerStore.path="./p2pstore2" \
--p2p.peers="/ip4/127.0.0.1/tcp/15600/p2p/12D3KooWLeCUVqiNbr68vDmjzK1GbJrWGjwttPJ41QvNf36Vavyd"
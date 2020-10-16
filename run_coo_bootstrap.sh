#!/bin/bash
export COO_PRV_KEY=651941eddb3e68cb1f6ef4ef5b04625dcf5c70de1fdc4b1c9eadb2c219c074e0ed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c
rm -rf alphanetdb
rm coordinator.state
go run -tags "pow_avx" main.go -c config_alphanet \
--cooBootstrap \
--cooStartIndex 0 \
--node.enablePlugins="Spammer,Coordinator" \
--p2p.bindMultiAddresses="/ip4/127.0.0.1/tcp/15600" \
--p2p.peerStore.path="./p2pstore" \
--p2p.peers="/ip4/127.0.0.1/tcp/15601/p2p/12D3KooWSPvkitDJdbFjM9sRW6i23YrVfWHRVyHrErTbazGpARZH"

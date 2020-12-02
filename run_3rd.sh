#!/bin/bash
rm -rf alphanetdb3
go run -tags "pow_avx" main.go -c config_alphanet.json \
--protocol.networkID="alphanet1" \
--restAPI.bindAddress="0.0.0.0:14267" \
--dashboard.bindAddress="localhost:8083" \
--db.path="alphanetdb3" \
--node.disablePlugins="Autopeering" \
--node.enablePlugins="Spammer,MQTT" \
--snapshots.fullPath="snapshots/alphanet3/full_export.bin" \
--snapshots.deltaPath="snapshots/alphanet3/delta_export.bin" \
--p2p.bindMultiAddresses="/ip4/127.0.0.1/tcp/15602" \
--profiling.bindAddress="127.0.0.1:6062" \
--prometheus.bindAddress="localhost:9313" \
--prometheus.fileServiceDiscovery.target="localhost:9313" \
--p2p.peerStore.path="./p2pstore3" \
--p2p.identityPrivateKey="5126767a84e1ced849dbbf2be809fd40f90bcfb81bd0d3309e2e25e34f803bf265500854f1f0e8fd3c389cf7b6b59cfd422b9319f257e2a8d3a772973560acdd" \
--p2p.peers="/ip4/127.0.0.1/tcp/15600/p2p/12D3KooWSagdVaCrS14GeJhM8CbQr41AW2PiYMgptTyAybCbQuEY,/ip4/127.0.0.1/tcp/15601/p2p/12D3KooWCKwcTWevoRKa2kEBputeGASvEBuDfRDSbe8t1DWugUmL,/ip4/127.0.0.1/tcp/15603/p2p/12D3KooWC7uE9w3RN4Vh1FJAZa8SbE8yMWR6wCVBajcWpyWguV73"

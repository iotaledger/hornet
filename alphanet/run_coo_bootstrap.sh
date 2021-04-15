#!/bin/bash
./create_snapshot_alphanet.sh
rm -rf alphanetdb
rm coordinator.state
export COO_PRV_KEYS=651941eddb3e68cb1f6ef4ef5b04625dcf5c70de1fdc4b1c9eadb2c219c074e0ed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c,0e324c6ff069f31890d496e9004636fd73d8e8b5bea08ec58a4178ca85462325f6752f5f46a53364e2ee9c4d662d762a81efd51010282a75cd6bd03f28ef349c
go run ../main.go -c config_alphanet.json \
--cooBootstrap \
--cooStartIndex 0 \
--protocol.networkID="alphanet1" \
--restAPI.bindAddress="0.0.0.0:14265" \
--dashboard.bindAddress="localhost:8081" \
--db.path="alphanetdb" \
--node.disablePlugins="Autopeering" \
--node.enablePlugins="Spammer,Coordinator,MQTT,Debug,Prometheus" \
--snapshots.fullPath="snapshots/alphanet1/full_export.bin" \
--snapshots.deltaPath="snapshots/alphanet1/delta_export.bin" \
--p2p.bindMultiAddresses="/ip4/127.0.0.1/tcp/15600" \
--profiling.bindAddress="127.0.0.1:6060" \
--prometheus.bindAddress="localhost:9311" \
--prometheus.fileServiceDiscovery.target="localhost:9311" \
--p2p.peerStore.path="./p2pstore" \
--p2p.identityPrivateKey="1f46fad4f538a031d4f87f490f6bca4319dfd0307636a5759a22b5e8874bd608f9156ba976a12918c16a481c38c88a7b5351b769adc30390e93b6c0a63b09b79" \
--p2p.peers="/ip4/127.0.0.1/tcp/15601/p2p/12D3KooWCKwcTWevoRKa2kEBputeGASvEBuDfRDSbe8t1DWugUmL,/ip4/127.0.0.1/tcp/15602/p2p/12D3KooWGdr8M5KX8KuKaXSiKfHJstdVnRkadYmupF7tFk2HrRoA,/ip4/127.0.0.1/tcp/15603/p2p/12D3KooWC7uE9w3RN4Vh1FJAZa8SbE8yMWR6wCVBajcWpyWguV73"

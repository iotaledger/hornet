#!/bin/bash
rm -rf privatedb2
go run ../main.go -c config_private_tangle.json \
--protocol.networkID="private_tangle1" \
--restAPI.bindAddress="0.0.0.0:14266" \
--dashboard.bindAddress="localhost:8082" \
--db.path="privatedb2" \
--node.disablePlugins="Autopeering" \
--node.enablePlugins="Spammer,MQTT,Debug,Prometheus" \
--snapshots.fullPath="snapshots/private_tangle2/full_snapshot.bin" \
--snapshots.deltaPath="snapshots/private_tangle2/delta_snapshot.bin" \
--p2p.bindMultiAddresses="/ip4/127.0.0.1/tcp/15601" \
--profiling.bindAddress="127.0.0.1:6061" \
--prometheus.bindAddress="localhost:9312" \
--prometheus.fileServiceDiscovery.target="localhost:9312" \
--p2p.db.path="p2pstore2" \
--p2p.identityPrivateKey="a06b288ce7fc3b6f1e716f6f7d72050b53417aae4b305a68883550a3bb28597f254b082515a79391a7f13009b4133851a0c4d48e0e948809c3b46ff3e2500b4f" \
--p2p.peers="/ip4/127.0.0.1/tcp/15600/p2p/12D3KooWSagdVaCrS14GeJhM8CbQr41AW2PiYMgptTyAybCbQuEY,/ip4/127.0.0.1/tcp/15602/p2p/12D3KooWGdr8M5KX8KuKaXSiKfHJstdVnRkadYmupF7tFk2HrRoA,/ip4/127.0.0.1/tcp/15603/p2p/12D3KooWC7uE9w3RN4Vh1FJAZa8SbE8yMWR6wCVBajcWpyWguV73"

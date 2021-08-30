#!/bin/bash
./create_snapshot_private_tangle.sh
rm -rf privatedb
rm coordinator.state
export COO_PRV_KEYS=651941eddb3e68cb1f6ef4ef5b04625dcf5c70de1fdc4b1c9eadb2c219c074e0ed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c,0e324c6ff069f31890d496e9004636fd73d8e8b5bea08ec58a4178ca85462325f6752f5f46a53364e2ee9c4d662d762a81efd51010282a75cd6bd03f28ef349c
go run ../main.go -c config_private_tangle.json \
--cooBootstrap \
--cooStartIndex 0 \
--migratorBootstrap \
--protocol.networkID="private_tangle1" \
--restAPI.bindAddress="0.0.0.0:14265" \
--dashboard.bindAddress="localhost:8081" \
--db.path="privatedb" \
--node.disablePlugins="" \
--node.enablePlugins="Spammer,Coordinator,MQTT,Receipts,Migrator" \
--receipts.backup.enabled="true" \
--receipts.backup.path="receipts" \
--receipts.validator.validate="true" \
--receipts.validator.api.address="http://127.0.0.1:14266" \
--receipts.validator.api.timeout="10s" \
--receipts.validator.coordinator.address="GVODMYUGDSAJKWXMVDNTKVE99DCBKARJ9CTHCPYTHOPEMSITGOAZJLINYIZTOSIMBWXCIZJJNUQBXVXMW" \
--receipts.validator.coordinator.merkleTreeDepth="18" \
--migrator.stateFilePath="./migrator.state" \
--snapshots.fullPath="snapshots/private_tangle1/full_snapshot.bin" \
--snapshots.deltaPath="snapshots/private_tangle1/delta_snapshot.bin" \
--p2p.bindMultiAddresses="/ip4/127.0.0.1/tcp/15601" \
--profiling.bindAddress="127.0.0.1:6061" \
--prometheus.bindAddress="localhost:9311" \
--prometheus.fileServiceDiscovery.target="localhost:9311" \
--p2p.db.path="p2pstore" \
--p2p.identityPrivateKey="1f46fad4f538a031d4f87f490f6bca4319dfd0307636a5759a22b5e8874bd608f9156ba976a12918c16a481c38c88a7b5351b769adc30390e93b6c0a63b09b79" \

#!/bin/bash
rm coordinator.state
rm -Rf alphanetdb/
./create_snapshot_alphanet.sh
export COO_PRV_KEYS=651941eddb3e68cb1f6ef4ef5b04625dcf5c70de1fdc4b1c9eadb2c219c074e0ed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c,0e324c6ff069f31890d496e9004636fd73d8e8b5bea08ec58a4178ca85462325f6752f5f46a53364e2ee9c4d662d762a81efd51010282a75cd6bd03f28ef349c
go run -tags "pow_avx" main.go -c config_alphanet.json -n peering_alphanet.json --cooBootstrap --cooStartIndex 0 --node.enablePlugins="Spammer","Coordinator"

#!/bin/bash
export COO_PRV_KEY=651941eddb3e68cb1f6ef4ef5b04625dcf5c70de1fdc4b1c9eadb2c219c074e0ed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c
go run -tags "pow_avx" main.go -c config_alphanet -n peering_alphanet --node.enablePlugins="Coordinator"

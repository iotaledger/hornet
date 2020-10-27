#!/bin/bash
rm -Rf alphanetdb2/
go run -tags "pow_avx" main.go -c config_alphanet2.json -n peering_alphanet2.json

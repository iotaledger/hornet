del /f /q /s alphanetdb3
go run -tags "pow_avx" main.go -c config_alphanet3.json ^
--profiling.bindAddress="127.0.0.1:6062" ^
--p2p.bindMultiAddresses="/ip4/127.0.0.1/tcp/15602" ^
--p2p.peerStore.path="./p2pstore3" ^
--p2p.identityPrivateKey="5126767a84e1ced849dbbf2be809fd40f90bcfb81bd0d3309e2e25e34f803bf265500854f1f0e8fd3c389cf7b6b59cfd422b9319f257e2a8d3a772973560acdd" ^
--p2p.peers="/ip4/127.0.0.1/tcp/15600/p2p/12D3KooWSagdVaCrS14GeJhM8CbQr41AW2PiYMgptTyAybCbQuEY,/ip4/127.0.0.1/tcp/15601/p2p/12D3KooWCKwcTWevoRKa2kEBputeGASvEBuDfRDSbe8t1DWugUmL"

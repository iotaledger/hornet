del /f /q /s privatedb3
go run "..\main.go" -c config_private_tangle.json ^
--protocol.networkID="private_tangle1" ^
--restAPI.bindAddress="0.0.0.0:14267" ^
--dashboard.bindAddress="localhost:8083" ^
--db.path="privatedb3" ^
--node.disablePlugins="Autopeering" ^
--node.enablePlugins="Spammer,MQTT,Debug,Prometheus" ^
--snapshots.fullPath="snapshots/private_tangle3/full_snapshot.bin" ^
--snapshots.deltaPath="snapshots/private_tangle3/delta_snapshot.bin" ^
--p2p.bindMultiAddresses="/ip4/127.0.0.1/tcp/15602" ^
--profiling.bindAddress="127.0.0.1:6062" ^
--prometheus.bindAddress="localhost:9313" ^
--prometheus.fileServiceDiscovery.target="localhost:9313" ^
--p2p.db.path="p2pstore3" ^
--p2p.identityPrivateKey="5126767a84e1ced849dbbf2be809fd40f90bcfb81bd0d3309e2e25e34f803bf265500854f1f0e8fd3c389cf7b6b59cfd422b9319f257e2a8d3a772973560acdd" ^
--p2p.peers="/ip4/127.0.0.1/tcp/15600/p2p/12D3KooWSagdVaCrS14GeJhM8CbQr41AW2PiYMgptTyAybCbQuEY,/ip4/127.0.0.1/tcp/15601/p2p/12D3KooWCKwcTWevoRKa2kEBputeGASvEBuDfRDSbe8t1DWugUmL,/ip4/127.0.0.1/tcp/15603/p2p/12D3KooWC7uE9w3RN4Vh1FJAZa8SbE8yMWR6wCVBajcWpyWguV73"

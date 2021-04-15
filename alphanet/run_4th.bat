del /f /q /s alphanetdb4
go run "..\main.go" -c config_alphanet.json ^
--protocol.networkID="alphanet1" ^
--restAPI.bindAddress="0.0.0.0:14268" ^
--dashboard.bindAddress="localhost:8084" ^
--db.path="alphanetdb4" ^
--node.disablePlugins="Autopeering" ^
--node.enablePlugins="Spammer,MQTT,Debug,Prometheus" ^
--snapshots.fullPath="snapshots/alphanet4/full_export.bin" ^
--snapshots.deltaPath="snapshots/alphanet4/delta_export.bin" ^
--p2p.bindMultiAddresses="/ip4/127.0.0.1/tcp/15603" ^
--profiling.bindAddress="127.0.0.1:6063" ^
--prometheus.bindAddress="localhost:9314" ^
--prometheus.fileServiceDiscovery.target="localhost:9314" ^
--p2p.peerStore.path="./p2pstore4" ^
--p2p.identityPrivateKey="996dceaeddcb5fc21480646f38ac53c4f5668fd33f3c0bfecfd004861d4a9dc722355dabd7f31a1266423abcf6c1db6228eb8283deb55731915ed06bd2ca387e" ^
--p2p.peers="/ip4/127.0.0.1/tcp/15600/p2p/12D3KooWSagdVaCrS14GeJhM8CbQr41AW2PiYMgptTyAybCbQuEY,/ip4/127.0.0.1/tcp/15601/p2p/12D3KooWCKwcTWevoRKa2kEBputeGASvEBuDfRDSbe8t1DWugUmL,/ip4/127.0.0.1/tcp/15602/p2p/12D3KooWGdr8M5KX8KuKaXSiKfHJstdVnRkadYmupF7tFk2HrRoA"

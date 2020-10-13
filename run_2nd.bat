del /f /q /s alphanetdb2
go run -tags "pow_avx" main.go -c config_alphanet2 ^
--profiling.bindAddress="127.0.0.1:6161" ^
--p2p.bindAddresses="/ip4/127.0.0.1/tcp/15601" ^
--p2p.peerStore.path="./p2pstore2" ^
--p2p.peers="/ip4/127.0.0.1/tcp/15600/p2p/12D3KooWFJ8Nq6gHLLvigTpPSbyMmLk35k1TcpJof8Y4y8yFAB32,/ip4/127.0.0.1/tcp/15601/p2p/12D3KooWPEFNwbzoRWARYGYP4kR5LkR1PunAzkv4Ammo5CYq7SQq"
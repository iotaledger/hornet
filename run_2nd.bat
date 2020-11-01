del /f /q /s alphanetdb2
go run -tags "pow_avx" main.go -c config_alphanet2.json \
--profiling.bindAddress="127.0.0.1:6061" ^
--p2p.bindMultiAddresses="/ip4/127.0.0.1/tcp/15601" ^
--p2p.peerStore.path="./p2pstore2" ^
--p2p.identityPrivateKey="a06b288ce7fc3b6f1e716f6f7d72050b53417aae4b305a68883550a3bb28597f254b082515a79391a7f13009b4133851a0c4d48e0e948809c3b46ff3e2500b4f" ^
--p2p.peers="/ip4/127.0.0.1/tcp/15600/p2p/12D3KooWSagdVaCrS14GeJhM8CbQr41AW2PiYMgptTyAybCbQuEY,/ip4/127.0.0.1/tcp/15602/p2p/12D3KooWGdr8M5KX8KuKaXSiKfHJstdVnRkadYmupF7tFk2HrRoA"

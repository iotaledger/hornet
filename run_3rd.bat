del /f /q /s alphanetdb3
go run -tags "pow_avx" main.go -c config_alphanet3 ^
--profiling.bindAddress="127.0.0.1:6262" ^
--p2p.bindMultiAddresses="/ip4/127.0.0.1/tcp/15602" ^
--p2p.peerStore.path="./p2pstore3" ^
--p2p.peers="/ip4/127.0.0.1/tcp/15601/p2p/12D3KooWCM6qcDMYHmn827QB6swNw9mFxCBWtAex4KeXjbLzsRTG"
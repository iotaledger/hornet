module github.com/gohornet/hornet

go 1.16

replace github.com/fhmq/hmq => github.com/luca-moser/hmq v0.0.0-20210322100045-d93c5b165ed2

replace github.com/linxGnu/grocksdb => github.com/gohornet/grocksdb v1.6.34-0.20210518222204-d6ea5eedcfb9

require (
	github.com/DataDog/zstd v1.4.8 // indirect
	github.com/Microsoft/go-winio v0.4.16 // indirect
	github.com/Shopify/sarama v1.29.0 // indirect
	github.com/StackExchange/wmi v0.0.0-20210224194228-fe8f1750fd46 // indirect
	github.com/benbjohnson/clock v1.1.0 // indirect
	github.com/bits-and-blooms/bitset v1.2.0
	github.com/blang/vfs v1.0.0
	github.com/cockroachdb/errors v1.8.4 // indirect
	github.com/cockroachdb/pebble v0.0.0-20210518170852-86efa0d0ce7b
	github.com/cockroachdb/redact v1.0.9 // indirect
	github.com/containerd/containerd v1.4.4 // indirect
	github.com/dgraph-io/ristretto v0.0.3 // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/dgryski/go-farm v0.0.0-20200201041132-a6ae2369ad13 // indirect
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v20.10.5+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-units v0.4.0 // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/eclipse/paho.mqtt.golang v1.3.4
	github.com/fhmq/hmq v0.0.0-20210318020249-ccbe364f9fbe
	github.com/gin-gonic/gin v1.7.1 // indirect
	github.com/go-echarts/go-echarts v1.0.0
	github.com/go-ole/go-ole v1.2.5 // indirect
	github.com/go-playground/validator/v10 v10.6.1 // indirect
	github.com/gobuffalo/packr/v2 v2.8.1
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/gorilla/websocket v1.4.2
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-version v1.3.0 // indirect
	github.com/iotaledger/hive.go v0.0.0-20210527103851-70e96f4e355a
	github.com/iotaledger/iota.go v1.0.0
	github.com/iotaledger/iota.go/v2 v2.0.0
	github.com/ipfs/go-ds-badger v0.2.6
	github.com/ipfs/go-ipns v0.1.0 // indirect
	github.com/json-iterator/go v1.1.11 // indirect
	github.com/karrick/godirwalk v1.16.1 // indirect
	github.com/klauspost/cpuid/v2 v2.0.6 // indirect
	github.com/knadh/koanf v1.0.0 // indirect
	github.com/koron/go-ssdp v0.0.2 // indirect
	github.com/labstack/echo/v4 v4.3.0
	github.com/leodido/go-urn v1.2.1 // indirect
	github.com/libp2p/go-libp2p v0.14.0
	github.com/libp2p/go-libp2p-asn-util v0.0.0-20210422100720-09a655867a6c // indirect
	github.com/libp2p/go-libp2p-connmgr v0.2.4
	github.com/libp2p/go-libp2p-core v0.8.5
	github.com/libp2p/go-libp2p-discovery v0.5.0
	github.com/libp2p/go-libp2p-kad-dht v0.12.0
	github.com/libp2p/go-libp2p-peerstore v0.2.7
	github.com/libp2p/go-libp2p-yamux v0.5.4 // indirect
	github.com/libp2p/go-tcp-transport v0.2.2 // indirect
	github.com/linxGnu/grocksdb v1.6.33 // indirect
	github.com/miekg/dns v1.1.42 // indirect
	github.com/moby/term v0.0.0-20201216013528-df9cb8a40635 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/mr-tron/base58 v1.2.0 // indirect
	github.com/multiformats/go-multiaddr v0.3.2
	github.com/onsi/ginkgo v1.15.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/pelletier/go-toml v1.8.1 // indirect
	github.com/pkg/errors v0.9.1
	github.com/polydawn/refmt v0.0.0-20201211092308-30ac6d18308e // indirect
	github.com/prometheus/client_golang v1.10.0
	github.com/prometheus/common v0.25.0 // indirect
	github.com/segmentio/fasthash v1.0.3 // indirect
	github.com/shirou/gopsutil v3.21.4+incompatible
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/tcnksm/go-latest v0.0.0-20170313132115-e3007ae9052e
	github.com/tidwall/gjson v1.8.0 // indirect
	github.com/tklauser/go-sysconf v0.3.6 // indirect
	github.com/ugorji/go v1.2.6 // indirect
	github.com/wollac/iota-crypto-demo v0.0.0-20210301113242-d76f3e4d14cb
	gitlab.com/powsrv.io/go/client v0.0.0-20210203192329-84583796cd46
	go.etcd.io/bbolt v1.3.5
	go.uber.org/atomic v1.7.0
	go.uber.org/dig v1.10.0
	go.uber.org/multierr v1.7.0 // indirect
	go.uber.org/zap v1.17.0 // indirect
	golang.org/x/crypto v0.0.0-20210513164829-c07d793c2f9a
	golang.org/x/exp v0.0.0-20210514180818-737f94c0881e // indirect
	golang.org/x/net v0.0.0-20210525063256-abc453219eb5
	golang.org/x/sys v0.0.0-20210531080801-fdfd190a6549 // indirect
	golang.org/x/term v0.0.0-20210503060354-a79de5458b56 // indirect
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba
	google.golang.org/genproto v0.0.0-20210518161634-ec7691c0a37d // indirect
	google.golang.org/grpc v1.37.1 // indirect
	gotest.tools/v3 v3.0.3 // indirect
)

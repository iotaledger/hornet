module github.com/gohornet/hornet

go 1.16

replace github.com/fhmq/hmq => github.com/luca-moser/hmq v0.0.0-20210322100045-d93c5b165ed2

require (
	github.com/DataDog/zstd v1.4.8 // indirect
	github.com/Microsoft/go-winio v0.4.16 // indirect
	github.com/Shopify/sarama v1.28.0 // indirect
	github.com/StackExchange/wmi v0.0.0-20210224194228-fe8f1750fd46 // indirect
	github.com/benbjohnson/clock v1.1.0 // indirect
	github.com/blang/vfs v1.0.0
	github.com/cockroachdb/errors v1.8.3 // indirect
	github.com/cockroachdb/pebble v0.0.0-20210406003833-3d4c32f510a8
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
	github.com/eclipse/paho.mqtt.golang v1.3.3
	github.com/fhmq/hmq v0.0.0-20210318020249-ccbe364f9fbe
	github.com/flynn/noise v0.0.0-20210331153838-4bdb43be3117 // indirect
	github.com/go-echarts/go-echarts v1.0.0
	github.com/go-ole/go-ole v1.2.5 // indirect
	github.com/gobuffalo/packr/v2 v2.8.1
	github.com/golang/snappy v0.0.3 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/uuid v1.2.0 // indirect
	github.com/gorilla/websocket v1.4.2
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-version v1.3.0 // indirect
	github.com/iotaledger/hive.go v0.0.0-20210423125659-4446146a4c6b
	github.com/iotaledger/iota.go v1.0.0-beta.15.0.20210406071024-a52cf8c2c21e
	github.com/iotaledger/iota.go/v2 v2.0.0-20210409074803-07a6438d40cf
	github.com/ipfs/go-ds-badger v0.2.6
	github.com/karrick/godirwalk v1.16.1 // indirect
	github.com/klauspost/compress v1.11.13 // indirect
	github.com/klauspost/cpuid/v2 v2.0.6 // indirect
	github.com/knadh/koanf v0.16.0 // indirect
	github.com/koron/go-ssdp v0.0.2 // indirect
	github.com/labstack/echo/v4 v4.2.2
	github.com/libp2p/go-libp2p v0.13.1-0.20210319000852-ffd67fd3dcf6
	github.com/libp2p/go-libp2p-asn-util v0.0.0-20210211060025-0db24c10d3bd // indirect
	github.com/libp2p/go-libp2p-connmgr v0.2.4
	github.com/libp2p/go-libp2p-core v0.8.5
	github.com/libp2p/go-libp2p-discovery v0.5.0
	github.com/libp2p/go-libp2p-kad-dht v0.11.1
	github.com/libp2p/go-libp2p-noise v0.1.3 // indirect
	github.com/libp2p/go-libp2p-peerstore v0.2.6
	github.com/linxGnu/grocksdb v1.6.33 // indirect
	github.com/mattn/go-colorable v0.1.8 // indirect
	github.com/mitchellh/copystructure v1.1.2 // indirect
	github.com/mitchellh/mapstructure v1.4.1 // indirect
	github.com/moby/term v0.0.0-20201216013528-df9cb8a40635 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/multiformats/go-multiaddr v0.3.1
	github.com/multiformats/go-multiaddr-dns v0.3.0 // indirect
	github.com/multiformats/go-multihash v0.0.15 // indirect
	github.com/onsi/ginkgo v1.15.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/pelletier/go-toml v1.8.1 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.10.0
	github.com/prometheus/common v0.20.0 // indirect
	github.com/segmentio/fasthash v1.0.3 // indirect
	github.com/shirou/gopsutil v3.21.3+incompatible
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/tcnksm/go-latest v0.0.0-20170313132115-e3007ae9052e
	github.com/tidwall/gjson v1.7.4 // indirect
	github.com/tklauser/go-sysconf v0.3.5 // indirect
	github.com/ugorji/go v1.2.5 // indirect
	github.com/willf/bitset v1.1.11
	github.com/wollac/iota-crypto-demo v0.0.0-20210301113242-d76f3e4d14cb
	gitlab.com/powsrv.io/go/client v0.0.0-20210203192329-84583796cd46
	go.etcd.io/bbolt v1.3.5
	go.uber.org/atomic v1.7.0
	go.uber.org/dig v1.10.0
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2
	golang.org/x/exp v0.0.0-20210405174845-4513512abef3 // indirect
	golang.org/x/net v0.0.0-20210410081132-afb366fc7cd1
	golang.org/x/sys v0.0.0-20210403161142-5e06dd20ab57 // indirect
	golang.org/x/term v0.0.0-20210406210042-72f3dc4e9b72 // indirect
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba
	google.golang.org/genproto v0.0.0-20210406143921-e86de6bf7a46 // indirect
	google.golang.org/grpc v1.37.0 // indirect
	gotest.tools/v3 v3.0.3 // indirect
)

module github.com/gohornet/hornet

go 1.16

replace github.com/labstack/gommon => github.com/muXxer/gommon v0.3.1-0.20210805133353-359faea1baf6

replace github.com/linxGnu/grocksdb => github.com/gohornet/grocksdb v1.6.38-0.20211012114404-55f425442260

// Exclude incompatible versions for now, until libp2p makes their whole stack compatible with go-libp2p-core v0.10.0
exclude github.com/libp2p/go-libp2p-core v0.10.0

exclude github.com/libp2p/go-libp2p-nat v0.1.0

exclude github.com/libp2p/go-libp2p-swarm v0.6.0

exclude github.com/libp2p/go-libp2p-noise v0.3.0

exclude github.com/libp2p/go-libp2p-transport-upgrader v0.5.0

exclude github.com/libp2p/go-libp2p-tls v0.3.0

exclude github.com/libp2p/go-conn-security-multistream v0.3.0

exclude github.com/libp2p/go-tcp-transport v0.3.0

exclude github.com/libp2p/go-nat v0.1.0

exclude github.com/libp2p/go-libp2p-autonat v0.5.0

require (
	github.com/DataDog/zstd v1.4.8 // indirect
	github.com/Microsoft/go-winio v0.5.0 // indirect
	github.com/Shopify/sarama v1.30.0 // indirect
	github.com/StackExchange/wmi v1.2.1 // indirect
	github.com/bits-and-blooms/bitset v1.2.1
	github.com/blang/vfs v1.0.0
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/cockroachdb/errors v1.8.6 // indirect
	github.com/cockroachdb/pebble v0.0.0-20211012000212-5393ca16ac52
	github.com/cockroachdb/redact v1.1.3 // indirect
	github.com/containerd/containerd v1.5.5 // indirect
	github.com/dgraph-io/ristretto v0.1.0 // indirect
	github.com/docker/docker v20.10.8+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/dustin/go-humanize v1.0.0
	github.com/eclipse/paho.mqtt.golang v1.3.5
	github.com/fhmq/hmq v0.0.0-20210810024638-1d6979189a22
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/gin-gonic/gin v1.7.4 // indirect
	github.com/go-echarts/go-echarts v1.0.0
	github.com/go-playground/validator/v10 v10.9.0 // indirect
	github.com/gobuffalo/logger v1.0.4 // indirect
	github.com/gobuffalo/packr/v2 v2.8.1
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/golang/glog v1.0.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/gorilla/websocket v1.4.2
	github.com/hashicorp/go-version v1.3.0 // indirect
	github.com/iotaledger/go-ds-kvstore v0.0.0-20210819121432-6e2ce2d41200
	github.com/iotaledger/hive.go v0.0.0-20211011085923-fd2eb0a47bf8
	github.com/iotaledger/iota.go v1.0.0
	github.com/iotaledger/iota.go/v2 v2.0.1-0.20210830162758-173bada804f9
	github.com/ipfs/go-cid v0.1.0 // indirect
	github.com/ipfs/go-datastore v0.4.6
	github.com/ipfs/go-ds-badger v0.2.7
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/karrick/godirwalk v1.16.1 // indirect
	github.com/knadh/koanf v1.2.4 // indirect
	github.com/labstack/echo/v4 v4.6.1
	github.com/labstack/gommon v0.3.0
	github.com/libp2p/go-libp2p v0.15.1
	github.com/libp2p/go-libp2p-connmgr v0.2.4
	github.com/libp2p/go-libp2p-core v0.9.0
	github.com/libp2p/go-libp2p-peerstore v0.3.0
	github.com/libp2p/go-reuseport-transport v0.1.0 // indirect
	github.com/linxGnu/grocksdb v1.6.38 // indirect
	github.com/mattn/go-colorable v0.1.11 // indirect
	github.com/mitchellh/mapstructure v1.4.2 // indirect
	github.com/moby/term v0.0.0-20210619224110-3f7ff695adc6 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/mr-tron/base58 v1.2.0
	github.com/multiformats/go-base32 v0.0.4 // indirect
	github.com/multiformats/go-multiaddr v0.4.1
	github.com/multiformats/go-multihash v0.0.16 // indirect
	github.com/pelletier/go-toml v1.9.3 // indirect
	github.com/pelletier/go-toml/v2 v2.0.0-beta.3
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/common v0.31.1 // indirect
	github.com/segmentio/fasthash v1.0.3 // indirect
	github.com/shirou/gopsutil v3.21.9+incompatible
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.1-0.20210427113832-6241f9ab9942
	github.com/tcnksm/go-latest v0.0.0-20170313132115-e3007ae9052e
	github.com/tidwall/gjson v1.9.3 // indirect
	github.com/tklauser/go-sysconf v0.3.9 // indirect
	github.com/ugorji/go v1.2.6 // indirect
	github.com/wollac/iota-crypto-demo v0.0.0-20210820085437-1a7b8ead2881
	gitlab.com/powsrv.io/go/client v0.0.0-20210203192329-84583796cd46
	go.uber.org/atomic v1.9.0
	go.uber.org/dig v1.13.0
	go.uber.org/zap v1.19.1 // indirect
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519
	golang.org/x/exp v0.0.0-20211011213208-1d87cf485e27 // indirect
	golang.org/x/net v0.0.0-20211011170408-caeb26a5c8c0
	golang.org/x/sys v0.0.0-20211007075335-d3039528d8ac // indirect
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac
	google.golang.org/genproto v0.0.0-20211011165927-a5fb3255271e // indirect
	google.golang.org/grpc v1.41.0 // indirect
)

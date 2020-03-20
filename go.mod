module github.com/gohornet/hornet

go 1.14

replace github.com/dgraph-io/badger/v2 v2.0.1 => github.com/muXxer/badger/v2 v2.0.3-hotfix

require (
	github.com/OneOfOne/xxhash v1.2.7 // indirect
	github.com/dgraph-io/badger/v2 v2.0.1
	github.com/dustin/go-humanize v1.0.0
	github.com/eclipse/paho.mqtt.golang v1.2.1-0.20200121105743-0d940dd29fd2
	github.com/fhmq/hmq v0.0.0-20200119031921-55f1f1aa8001
	github.com/fsnotify/fsnotify v1.4.9
	github.com/gin-contrib/gzip v0.0.1
	github.com/gin-gonic/gin v1.6.1
	github.com/go-zeromq/zmq4 v0.9.0
	github.com/gobuffalo/packr/v2 v2.8.0
	github.com/google/go-github v17.0.0+incompatible // indirect
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/gorilla/websocket v1.4.2
	github.com/hashicorp/go-version v1.2.0 // indirect
	github.com/iotaledger/hive.go v0.0.0-20200326163241-4aa5b4d3b5b0
	github.com/iotaledger/iota.go v1.0.0-beta.14.0.20200204161139-dfb0acdec240
	github.com/labstack/echo/v4 v4.1.15
	github.com/labstack/gommon v0.3.0
	github.com/mitchellh/mapstructure v1.2.2
	github.com/pkg/errors v0.9.1
	github.com/projectcalico/libcalico-go v3.9.0-0.dev+incompatible
	github.com/shirou/gopsutil v2.20.2+incompatible
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.6.2
	github.com/stretchr/testify v1.5.1
	github.com/tcnksm/go-latest v0.0.0-20170313132115-e3007ae9052e
	go.uber.org/atomic v1.5.1
	golang.org/x/crypto v0.0.0-20200323165209-0ec3e9974c59
	golang.org/x/net v0.0.0-20200324143707-d3edc9973b7e
)

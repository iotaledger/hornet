package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/labstack/echo-contrib/prometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"github.com/gohornet/hornet/pkg/inx"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/configuration"
)

const (
	APIRoute = "mqtt/v1"

	// CfgMQTTBindAddress the bind address on which the MQTT broker listens on.
	CfgMQTTBindAddress = "mqtt.bindAddress"
	// CfgMQTTWSPort the port of the WebSocket MQTT broker.
	CfgMQTTWSPort = "mqtt.wsPort"
	// CfgMQTTWorkerCount the number of parallel workers the MQTT broker uses to publish messages.
	CfgMQTTWorkerCount = "mqtt.workerCount"
	// CfgMQTTTopicCleanupThreshold the number of deleted topics that trigger a garbage collection of the topic manager.
	CfgMQTTTopicCleanupThreshold = "mqtt.topicCleanupThreshold"
	// CfgPrometheusEnabled enable prometheus metrics.
	CfgPrometheusEnabled = "prometheus.enabled"
	// CfgPrometheusBindAddress bind address on which the Prometheus HTTP server listens.
	CfgPrometheusBindAddress = "prometheus.bindAddress"
)

var (
	config *configuration.Configuration
)

func setupPrometheus(bindAddress string) {
	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())

	// Enable metrics middleware
	p := prometheus.NewPrometheus("echo", nil)
	p.Use(e)

	go func() {
		if err := e.Start(bindAddress); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				panic(err)
			}
		}
	}()
}

func main() {

	port, err := utils.LoadStringFromEnvironment("INX_PORT")
	if err != nil {
		panic(err)
	}

	config, err = loadConfigFile("config.json")
	if err != nil {
		panic(err)
	}

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	opts = append(opts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
		Time:                20 * time.Second,
		PermitWithoutStream: true,
	}))
	conn, err := grpc.Dial(fmt.Sprintf("localhost:%s", port), opts...)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	client := inx.NewINXClient(conn)
	server, err := NewServer(client)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		server.Start(ctx)
	}()

	apiReq := &inx.APIRouteRequest{
		Route: APIRoute,
		Host:  "localhost",
		Port:  uint32(config.Int(CfgMQTTWSPort)),
	}

	if config.Bool(CfgPrometheusEnabled) {
		prometheusBindAddressParts := strings.Split(config.String(CfgPrometheusBindAddress), ":")
		if len(prometheusBindAddressParts) != 2 {
			panic(fmt.Sprintf("Invalid %s", CfgPrometheusBindAddress))
		}
		prometheusPort, err := strconv.ParseInt(prometheusBindAddressParts[1], 10, 32)
		if err != nil {
			panic(err)
		}
		setupPrometheus(config.String(CfgPrometheusBindAddress))
		apiReq.MetricsPort = uint32(prometheusPort)
	}

	fmt.Println("Registering API route")
	if _, err := client.RegisterAPIRoute(context.Background(), apiReq); err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan bool, 1)
	go func() {
		select {
		case <-signalChan:
			done <- true
		case <-ctx.Done():
			done <- true
		}
	}()
	<-done
	cancel()
	fmt.Println("Removing API route")
	if _, err := client.UnregisterAPIRoute(context.Background(), apiReq); err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	fmt.Println("exiting")
}

func flagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	fs.String(CfgMQTTBindAddress, "localhost:1883", "bind address on which the MQTT broker listens on")
	fs.Int(CfgMQTTWSPort, 1888, "port of the WebSocket MQTT broker")
	fs.Int(CfgMQTTWorkerCount, 100, "number of parallel workers the MQTT broker uses to publish messages")
	fs.Int(CfgMQTTTopicCleanupThreshold, 10000, "number of deleted topics that trigger a garbage collection of the topic manager")
	fs.Bool(CfgPrometheusEnabled, false, "enable prometheus metrics")
	fs.String(CfgPrometheusBindAddress, "localhost:9313", "bind address on which the Prometheus HTTP server listens.")
	return fs
}

func loadConfigFile(filePath string) (*configuration.Configuration, error) {
	config := configuration.New()

	if err := config.LoadFile(filePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("loading config file failed: %w", err)
	}

	if err := config.LoadFlagSet(flagSet()); err != nil {
		return nil, err
	}

	return config, nil
}

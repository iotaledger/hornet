package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gohornet/hornet/integration-tests/wfmock/pkg/http"

	_ "github.com/gohornet/hornet/integration-tests/wfmock/pkg/http/whiteflag"
)

func main() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	http.Initialize()
	http.Start()
	defer http.Shutdown()

	// wait for termination
	<-quit
	log.Println("exiting")
}

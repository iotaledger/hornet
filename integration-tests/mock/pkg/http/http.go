package http

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gohornet/hornet/integration-tests/mock/pkg/config"
	"github.com/labstack/echo/v4"
)

const (
	shutdownTimeout = 10 * time.Second
	webAPIBase      = ""
)

var (
	server   *echo.Echo
	wg       sync.WaitGroup
	handlers = make(map[string]HandlerFunc)
)

// HandlerFunc defines a function to serve the provided request.
type HandlerFunc func(interface{}, echo.Context) error

// ErrorReturn defines the type of an API error.
type ErrorReturn struct {
	Error string `json:"error"`
}

// RegisterHandler registers a new API command.
func RegisterHandler(command string, handlerFunc HandlerFunc) {
	handlers[command] = handlerFunc
}

// Initialize initializes the HTTP server. Must be called before Start.
func Initialize() {
	cfg := config.GetConfig()

	server = echo.New()
	server.HideBanner = true // do not show the welcome banner
	server.HidePort = true   // print our own log message
	server.Server.Addr = cfg.HTTP.BindAddress

	ok200 := func(c echo.Context) error { return c.String(http.StatusOK, "ok") }

	// actual routes
	server.GET("/healthcheck", ok200)
	server.POST(webAPIBase, func(c echo.Context) error {
		request := make(map[string]interface{})
		if err := c.Bind(&request); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorReturn{fmt.Sprintf("invalid request: %s", err)})
		}

		cmd, ok := request["command"].(string)
		if !ok {
			return c.JSON(http.StatusBadRequest, ErrorReturn{Error: "error parsing command"})
		}

		handler, ok := handlers[strings.ToLower(cmd)]
		if !ok {
			return c.JSON(http.StatusBadRequest, ErrorReturn{Error: fmt.Sprintf("command '%s' is unknown", cmd)})
		}
		return handler(request, c)
	})
}

// Start starts the HTTP server in a separate go routine.
func Start() {
	wg.Add(1)
	go func() {
		defer wg.Done()

		log.Printf("http server started on %s\n", server.Server.Addr)
		if err := server.StartServer(server.Server); err != nil && err != http.ErrServerClosed {
			log.Fatalf("error starting http server: %v", err)
		}
		log.Println("http server stopped")
	}()
}

// Shutdown gracefully stops the HTTP server.
func Shutdown() {
	log.Println("http server is shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Could not gracefully shutdown http server: %v\n", err)
	}
	// wait for the go routine to clean up
	wg.Wait()
}

package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	mqttBrokerTopicsManagerSize prometheus.Gauge
)

func setupPrometheus(bindAddress string, server *Server) {

	mqttBrokerTopicsManagerSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "mqtt_broker",
			Name:      "topics_manager_size",
			Help:      "Number of active topics in the topics manager.",
		})

	prometheus.MustRegister(mqttBrokerTopicsManagerSize)

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())

	e.GET("/metrics", func(c echo.Context) error {

		server.collectMQTTBroker()

		handler := promhttp.Handler()
		handler.ServeHTTP(c.Response().Writer, c.Request())
		return nil
	})

	go func() {
		if err := e.Start(bindAddress); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				panic(err)
			}
		}
	}()
}

func (s *Server) collectMQTTBroker() {
	mqttBrokerTopicsManagerSize.Set(float64(s.MQTTBroker.TopicsManagerSize()))
}

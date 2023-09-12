package service

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/facebookgo/grace/gracehttp"
	"github.com/gorilla/websocket"

	"github.com/airenas/go-app/pkg/goapp"

	"github.com/labstack/echo-contrib/prometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// Data keeps data required for service work
type Data struct {
	Port            int
	WSHandlerStatus *WSConnHandler
	WSHandlerSpeech *WSConnHandler
	Ctx             context.Context
}

// StartWebServer starts echo web service
func StartWebServer(data *Data) error {
	goapp.Log.Info().Msgf("Starting wrapper service at %d", data.Port)
	if err := validate(data); err != nil {
		return err
	}

	portStr := strconv.Itoa(data.Port)

	e := initRoutes(data)

	e.Server.Addr = ":" + portStr
	e.Server.ReadHeaderTimeout = 5 * time.Second
	e.Server.ReadTimeout = 10 * time.Second
	e.Server.WriteTimeout = 10 * time.Second

	gracehttp.SetLogger(log.New(goapp.Log, "", 0))

	return gracehttp.Serve(e.Server)
}

var promMdlw *prometheus.Prometheus

func init() {
	promMdlw = prometheus.NewPrometheus("wrapper", nil)
}

func initRoutes(data *Data) *echo.Echo {
	e := echo.New()
	e.Use(middleware.Logger())
	promMdlw.Use(e)

	e.GET("/live", live(data))
	e.GET("/client/ws/status", subscribe(data, data.WSHandlerStatus))
	e.GET("/client/ws/speech", subscribe(data, data.WSHandlerSpeech))

	goapp.Log.Info().Msg("Routes:")
	for _, r := range e.Routes() {
		goapp.Log.Info().Msgf("  %s %s", r.Method, r.Path)
	}
	return e
}

func live(data *Data) func(echo.Context) error {
	return func(c echo.Context) error {
		return c.JSONBlob(http.StatusOK, []byte(`{"service":"OK"}`))
	}
}

func validate(data *Data) error {
	if data.WSHandlerStatus == nil {
		return fmt.Errorf("no WSHandlerStatus")
	}
	if data.WSHandlerSpeech == nil {
		return fmt.Errorf("no WSHandlerSpeech")
	}
	return nil
}

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	}}

func subscribe(data *Data, handler *WSConnHandler) func(echo.Context) error {
	return func(c echo.Context) error {
		ws, err := wsUpgrader.Upgrade(c.Response(), c.Request(), nil)
		if err != nil {
			goapp.Log.Error().Err(err).Send()
			return err
		}
		defer ws.Close()

		return handler.HandleConnection(data.Ctx, ws, c.Request().URL.RawQuery)
	}
}

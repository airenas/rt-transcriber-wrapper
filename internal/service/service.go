package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/facebookgo/grace/gracehttp"
	"github.com/gorilla/websocket"

	"github.com/airenas/go-app/pkg/goapp"
	"github.com/airenas/rt-transcriber-wrapper/internal/api"
	"github.com/airenas/rt-transcriber-wrapper/internal/domain"

	"github.com/labstack/echo-contrib/prometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type WSHandler interface {
	HandleConnection(context.Context, *websocket.Conn, *http.Request, string) error
}

type AudioManager interface {
	GetAudio(id string) ([]byte, error)
}

type ConfigManager interface {
	GetConfig(userID string) (*domain.User, error)
	SaveConfig(user *domain.User) error
}

type TextManager interface {
	GetTexts(ctx context.Context, userID string) (*domain.Texts, error)
	SaveTexts(ctx context.Context, userID string, input *domain.Texts) error
}

const userHeader = "X-User-Info"

// Data keeps data required for service work
type Data struct {
	Port            int
	DevMode         bool
	WSHandlerStatus WSHandler
	WSHandlerSpeech WSHandler
	AudioManager    AudioManager
	ConfigManager   ConfigManager
	TextManager     TextManager
	Ctx             context.Context
}

// StartWebServer starts echo web service
func StartWebServer(data *Data) (<-chan struct{}, error) {
	goapp.Log.Info().Msgf("Starting wrapper service at %d", data.Port)
	if err := validate(data); err != nil {
		return nil, err
	}

	portStr := strconv.Itoa(data.Port)

	e := initRoutes(data)

	e.Server.Addr = ":" + portStr
	e.Server.ReadHeaderTimeout = 5 * time.Second
	e.Server.ReadTimeout = 10 * time.Second
	e.Server.WriteTimeout = 10 * time.Second

	gracehttp.SetLogger(log.New(goapp.Log, "", 0))

	res := make(chan struct{}, 1)
	go func() {
		defer close(res)
		if err := gracehttp.Serve(e.Server); err != nil {
			goapp.Log.Error().Err(err).Msg("can't start web server")
		}
		goapp.Log.Info().Msg("exit http routine")
	}()
	return res, nil
}

var promMdlw *prometheus.Prometheus

func init() {
	promMdlw = prometheus.NewPrometheus("wrapper", nil)
}

func initRoutes(data *Data) *echo.Echo {
	e := echo.New()
	e.Use(middleware.Logger())
	promMdlw.Use(e)

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{echo.GET, echo.POST, echo.OPTIONS},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-User-Info"},
		AllowCredentials: true,
	}))

	e.GET("/live", live(data))
	e.GET("/client/ws/status", subscribe(data, data.WSHandlerStatus))
	e.GET("/client/ws/speech", subscribe(data, data.WSHandlerSpeech))
	e.GET("/client/audio/:id", audioHandler(data))
	e.GET("/client/config", configHandler(data))
	e.POST("/client/config", configSaveHandler(data))
	e.GET("/client/text", txtHandler(data))
	e.POST("/client/text", txtSaveHandler(data))

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
	if data.AudioManager == nil {
		return fmt.Errorf("no AudioManager")
	}
	if data.ConfigManager == nil {
		return fmt.Errorf("no ConfigManager")
	}
	if data.TextManager == nil {
		return fmt.Errorf("no TextManager")
	}
	return nil
}

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	}}

func subscribe(data *Data, handler WSHandler) func(echo.Context) error {
	return func(c echo.Context) error {
		user := &user{ID: "dev-user"}
		if data.DevMode {
			goapp.Log.Warn().Msg("Running in DEV mode - no auth")
		} else {
			var err error
			if user, err = extractUserFromHeader(c.Request().Header); err != nil {
				return fmt.Errorf("can't extract user from header: %w", err)
			}
		}

		ws, err := wsUpgrader.Upgrade(c.Response(), c.Request(), nil)
		if err != nil {
			goapp.Log.Error().Err(err).Send()
			return err
		}
		defer ws.Close()

		return handler.HandleConnection(data.Ctx, ws, c.Request(), user.ID)
	}
}

func audioHandler(data *Data) echo.HandlerFunc {
	return func(c echo.Context) error {
		id := c.Param("id")
		user, err := extractUserFromHeader(c.Request().Header)
		if err != nil {
			return c.String(http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		}
		goapp.Log.Warn().Str("id", id).Str("user", user.ID).Msg("Getting audio")

		finalId := fmt.Sprintf("audio-%s-%s", user.ID, id)

		data, err := data.AudioManager.GetAudio(finalId)
		if err != nil {
			return c.String(http.StatusNotFound, "audio not found")
		}

		return c.Blob(http.StatusOK, "audio/wav", data)
	}
}

func configHandler(data *Data) echo.HandlerFunc {
	return func(c echo.Context) error {
		user, err := extractUserFromHeader(c.Request().Header)
		if err != nil {
			return c.String(http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		}
		goapp.Log.Info().Str("id", user.ID).Msg("Getting config")

		data, err := data.ConfigManager.GetConfig(user.ID)
		if err != nil {
			return c.String(http.StatusNotFound, "config not found")
		}
		res := api.Config{
			SkipTour: data.SkipTour,
		}

		return c.JSON(http.StatusOK, res)
	}
}

func configSaveHandler(data *Data) echo.HandlerFunc {
	return func(c echo.Context) error {
		user, err := extractUserFromHeader(c.Request().Header)
		if err != nil {
			return c.String(http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		}
		goapp.Log.Info().Str("id", user.ID).Msg("Save config")
		var input api.Config
		if err := c.Bind(&input); err != nil {
			return c.String(http.StatusBadRequest, "invalid input")
		}

		err = data.ConfigManager.SaveConfig(&domain.User{
			ID:       user.ID,
			SkipTour: input.SkipTour,
		})
		if err != nil {
			goapp.Log.Error().Err(err).Msg("can't save config")
			return c.String(http.StatusInternalServerError, "failed to save config")
		}

		return c.String(http.StatusOK, "ok")
	}
}

func txtSaveHandler(data *Data) echo.HandlerFunc {
	return func(c echo.Context) error {
		user, err := extractUserFromHeader(c.Request().Header)
		if err != nil {
			return c.String(http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		}
		goapp.Log.Info().Str("id", user.ID).Msg("save txt")
		var input api.Texts
		if err := c.Bind(&input); err != nil {
			return c.String(http.StatusBadRequest, "invalid input")
		}

		inData := mapToTexts(&input)
		if err = data.TextManager.SaveTexts(c.Request().Context(), user.ID, inData); err != nil {
			goapp.Log.Error().Err(err).Msg("can't save texts")
			return c.String(http.StatusInternalServerError, "failed to save texts")
		}
		goapp.Log.Info().Str("id", user.ID).Msg("saved txt")
		return c.String(http.StatusOK, "ok")
	}
}

func mapToTexts(input *api.Texts) *domain.Texts {
	res := &domain.Texts{}
	for _, p := range input.Parts {
		res.Parts = append(res.Parts, domain.Part{
			ID:   p.ID,
			Text: p.Text,
		})
	}
	return res
}

func txtHandler(data *Data) echo.HandlerFunc {
	return func(c echo.Context) error {
		user, err := extractUserFromHeader(c.Request().Header)
		if err != nil {
			return c.String(http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		}
		goapp.Log.Info().Str("id", user.ID).Msg("Getting texts")
		texts, err := data.TextManager.GetTexts(c.Request().Context(), user.ID)
		if err != nil {
			return c.String(http.StatusInternalServerError, "failed to get texts")
		}

		res := mapFromTexts(texts)

		return c.JSON(http.StatusOK, res)
	}
}

func mapFromTexts(texts *domain.Texts) *api.Texts {
	res := &api.Texts{}
	for _, p := range texts.Parts {
		res.Parts = append(res.Parts, api.Part{
			ID:   p.ID,
			Text: p.Text,
		})
	}
	return res
}

type user struct {
	ID string `json:"id"`
}

func extractUserFromHeader(header http.Header) (*user, error) {
	encoded := header.Get(userHeader)
	if encoded == "" {
		return nil, errors.New("missing X-User-Info header")
	}
	return extractUserTxt(encoded)
}

func extractUserTxt(txt string) (*user, error) {
	decoded, err := base64.StdEncoding.DecodeString(txt)
	if err != nil {
		return nil, errors.New("invalid base64 header")
	}

	var user user
	if err := json.Unmarshal(decoded, &user); err != nil {
		return nil, fmt.Errorf("invalid JSON in header: %w", err)
	}
	if strings.TrimSpace(user.ID) == "" {
		return nil, errors.New("missing user ID in header")
	}

	return &user, nil
}

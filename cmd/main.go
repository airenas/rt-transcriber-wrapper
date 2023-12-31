package main

import (
	"context"

	"github.com/airenas/go-app/pkg/goapp"
	"github.com/airenas/rt-transcriber-wrapper/internal/service"
	"github.com/airenas/rt-transcriber-wrapper/internal/handlers"
	"github.com/labstack/gommon/color"
)

func main() {
	goapp.StartWithDefault()

	printBanner()

	cfg := goapp.Config

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	data := &service.Data{}
	data.Ctx = ctx
	data.Port = cfg.GetInt("port")
	data.WSHandlerStatus = service.NewWSHandler(cfg.GetString("status.url"))
	data.WSHandlerSpeech = service.NewWSHandler(cfg.GetString("speech.url"))
	var err error
	data.WSHandlerSpeech.Middleware, err = handlers.NewJoiner(cfg.GetString("joiner.url"))
	if err != nil {
		goapp.Log.Fatal().Err(err).Msg("can't init joiner")
	}

	if err := service.StartWebServer(data); err != nil {
		goapp.Log.Fatal().Err(err).Msg("can't start web server")
	}
}

var (
	version = "DEV"
)

func printBanner() {
	banner :=
		`
    KALDI GSTREAMER WRAPPER v: %s
	
%s
________________________________________________________

`
	cl := color.New()
	cl.Printf(banner, cl.Red(version), cl.Green("https://github.com/airenas/rt-transcriber-wrapper"))
}

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/airenas/go-app/pkg/goapp"
	"github.com/airenas/rt-transcriber-wrapper/internal/handlers"
	"github.com/airenas/rt-transcriber-wrapper/internal/service"
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
	data.WSHandlerStatus = service.NewWSSimpleHandler(cfg.GetString("status.url"))
	audioManager := service.NewMemoryAudioManager()
	data.AudioManager = audioManager
	trHandler := service.NewWSTranscriptionHandler(cfg.GetString("speech.url"), audioManager)
	data.WSHandlerSpeech = trHandler
	hList, err := handlers.NewListHandler()
	if err != nil {
		goapp.Log.Fatal().Err(err).Msg("can't init list handler")
	}

	cleaner, err := handlers.NewCleaner()
	if err != nil {
		goapp.Log.Fatal().Err(err).Msg("can't init cleaner")
	}
	joiner, err := handlers.NewJoiner(cfg.GetString("joiner.url"))
	if err != nil {
		goapp.Log.Fatal().Err(err).Msg("can't init joiner")
	}
	punctuator, err := handlers.NewPunctuator(cfg.GetString("punctuator.url"))
	if err != nil {
		goapp.Log.Fatal().Err(err).Msg("can't init punctuator")
	}

	hList.Add(cleaner)
	hList.Add(joiner)
	hList.Add(punctuator)
	trHandler.Middleware = hList

	doneCh, err := service.StartWebServer(data)
	if err != nil {
		goapp.Log.Fatal().Err(err).Msg("can't start web server")
	}

	/////////////////////// Waiting for terminate
	waitCh := make(chan os.Signal, 2)
	signal.Notify(waitCh, os.Interrupt, syscall.SIGTERM)
	select {
	case <-waitCh:
		goapp.Log.Info().Msg("Got exit signal")
	case <-doneCh:
		goapp.Log.Info().Msg("Service exit")
	}
	cancelFunc()
	select {
	case <-doneCh:
		goapp.Log.Info().Msg("All code returned. Now exit. Bye")
	case <-time.After(time.Second * 15):
		goapp.Log.Warn().Msg("Timeout gracefull shutdown")
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

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/airenas/go-app/pkg/goapp"
	"github.com/airenas/rt-transcriber-wrapper/internal/db"
	"github.com/airenas/rt-transcriber-wrapper/internal/handlers"
	"github.com/airenas/rt-transcriber-wrapper/internal/service"
	"github.com/labstack/gommon/color"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog"
)

func main() {
	goapp.StartWithDefault()
	log.Logger = goapp.Log
	zerolog.DefaultContextLogger = &goapp.Log

	printBanner()

	cfg := goapp.Config

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	data := &service.Data{}
	data.Ctx = ctx
	data.Port = cfg.GetInt("port")
	data.DevMode = cfg.GetBool("devMode")

	maxWorkers := cfg.GetInt("status.maxWorkers")
	if maxWorkers < 1 {
		goapp.Log.Fatal().Int("status.maxWorkers", maxWorkers).Msg("invalid max workers config")
	}
	workers := service.NewWorkerTracker(maxWorkers)
	data.WSHandlerStatus = service.NewWSStatusHandler(workers)

	dataManager, err := db.NewRedisDataManager(cfg.GetString("redis.url"), cfg.GetString("redis.encryptionKey"), cfg.GetDuration("redis.ttl"))
	if err != nil {
		goapp.Log.Fatal().Err(err).Msg("can't init redis")
	}
	defer dataManager.Close()
	data.AudioManager = dataManager
	data.ConfigManager = dataManager
	data.TextManager = dataManager
	trHandler := service.NewWSTranscriptionHandler(cfg.GetString("speech.url"), dataManager, workers)
	data.WSHandlerSpeech = trHandler
	hList, err := handlers.NewListHandler()
	if err != nil {
		goapp.Log.Fatal().Err(err).Msg("can't init list handler")
	}

	// cleaner, err := handlers.NewCleaner()
	// if err != nil {
	// 	goapp.Log.Fatal().Err(err).Msg("can't init cleaner")
	// }
	// joiner, err := handlers.NewJoiner(cfg.GetString("joiner.url"))
	// if err != nil {
	// 	goapp.Log.Fatal().Err(err).Msg("can't init joiner")
	// }
	punctuator, err := handlers.NewPunctuator(cfg.GetString("punctuator.url"))
	if err != nil {
		goapp.Log.Fatal().Err(err).Msg("can't init punctuator")
	}

	// hList.Add(cleaner)
	// hList.Add(joiner)
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

package handlers

import (
	"context"
	"time"

	"github.com/airenas/go-app/pkg/goapp"
	"github.com/airenas/rt-transcriber-wrapper/internal/api"
	"github.com/airenas/rt-transcriber-wrapper/internal/utils"
)

type Handler interface {
	Process(context.Context, *api.FullResult) (*api.FullResult, error)
}

// List passes data to list of middleware
type ListHandler struct {
	hadlers []Handler
}

func NewListHandler() (*ListHandler, error) {
	res := &ListHandler{}
	return res, nil
}

func (sp *ListHandler) Process(ctx context.Context, data *api.FullResult) (*api.FullResult, error) {
	defer utils.MeasureTime("process", time.Now())
	dataCopy := data
	for i, h := range sp.hadlers {
		goapp.Log.Debug().Int("handler", i).Msg("Processing")
		if dataNew, err := h.Process(ctx, dataCopy); err != nil {
			goapp.Log.Error().Err(err).Msg("Can't process")
		} else {
			dataCopy = dataNew
		}
		goapp.Log.Debug().Int("handler", i).Msg("Finished")
	}
	dataCopy.Event = "TRANSCRIPTION"
	return dataCopy, nil
}

func (sp *ListHandler) Add(h Handler) {
	sp.hadlers = append(sp.hadlers, h)
}

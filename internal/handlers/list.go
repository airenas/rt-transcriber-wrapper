package handlers

import (
	"context"

	"github.com/airenas/go-app/pkg/goapp"
)

type Handler interface {
	Process(context.Context, string) (string, error)
}

// List passes data to list of middleware
type ListHandler struct {
	hadlers []Handler
}

func NewListHandler() (*ListHandler, error) {
	res := &ListHandler{}
	return res, nil
}

func (sp *ListHandler) Process(ctx context.Context, data string) (string, error) {
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
	return dataCopy, nil
}

func (sp *ListHandler) Add(h Handler) {
	sp.hadlers = append(sp.hadlers, h)
}

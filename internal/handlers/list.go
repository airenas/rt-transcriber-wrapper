package handlers

import (
	"context"
	"time"

	"github.com/airenas/rt-transcriber-wrapper/internal/domain"
	"github.com/airenas/rt-transcriber-wrapper/internal/utils"
	"github.com/rs/zerolog/log"
)

type Handler interface {
	Process(context.Context, *domain.K2Data) (*domain.K2Data, error)
}

// List passes data to list of middleware
type ListHandler struct {
	hadlers []Handler
}

func NewListHandler() (*ListHandler, error) {
	res := &ListHandler{}
	return res, nil
}

func (sp *ListHandler) Process(ctx context.Context, data *domain.K2Data) (*domain.K2Data, error) {
	defer utils.MeasureTime("process", time.Now())
	dataCopy := data
	for i, h := range sp.hadlers {
		log.Ctx(ctx).Trace().Int("handler", i).Msg("Processing")
		if dataNew, err := h.Process(ctx, dataCopy); err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("Can't process")
		} else {
			dataCopy = dataNew
		}
		log.Ctx(ctx).Trace().Int("handler", i).Msg("Finished")
	}
	return dataCopy, nil
}

func (sp *ListHandler) Add(h Handler) {
	sp.hadlers = append(sp.hadlers, h)
}

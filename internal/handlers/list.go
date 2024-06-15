package handlers

import (
	"bytes"
	"context"
	"encoding/json"
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

func (sp *ListHandler) Process(ctx context.Context, data string) (string, error) {
	defer utils.MeasureTime("process", time.Now())
	inData, err := decode(data)
	if err != nil {
		goapp.Log.Error().Err(err).Msg("Can't decode")
		return data, err
	}
	dataCopy := inData
	for i, h := range sp.hadlers {
		goapp.Log.Debug().Int("handler", i).Msg("Processing")
		if dataNew, err := h.Process(ctx, dataCopy); err != nil {
			goapp.Log.Error().Err(err).Msg("Can't process")
		} else {
			dataCopy = dataNew
		}
		goapp.Log.Debug().Int("handler", i).Msg("Finished")
	}
	return encode(inData)
}

func (sp *ListHandler) Add(h Handler) {
	sp.hadlers = append(sp.hadlers, h)
}

func decode(data string) (*api.FullResult, error) {
	res := &api.FullResult{}
	err := json.NewDecoder(bytes.NewBufferString(data)).Decode(&res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func encode(inData *api.FullResult) (string, error) {
	b := new(bytes.Buffer)
	if err := json.NewEncoder(b).Encode(inData); err != nil {
		return "", err
	}
	return b.String(), nil
}

package handlers

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/airenas/go-app/pkg/goapp"
	"github.com/airenas/rt-transcriber-wrapper/internal/api"
	"github.com/airenas/rt-transcriber-wrapper/internal/utils"
)

// Cleaner cleans text
type Cleaner struct {
	reSpaces *regexp.Regexp
}

// NewCleaner creates a text cleaner
func NewCleaner() (*Cleaner, error) {
	res := Cleaner{}
	re, err := regexp.Compile(`\s+`)
	if err != nil {
		return nil, err
	}
	res.reSpaces = re

	goapp.Log.Info().Msg("Cleaner")
	return &res, nil
}

func (sp *Cleaner) Process(ctx context.Context, data *api.FullResult) (*api.FullResult, error) {
	defer utils.MeasureTime("cleaner", time.Now())
	if len(data.Result.Hypotheses) > 0 {
		newText, err := sp.transform(data.Result.Hypotheses[0].Transcript)
		if err != nil {
			return nil, err
		}
		data.Result.Hypotheses[0].Transcript = newText
	}
	return data, nil
}

func (sp *Cleaner) transform(text string) (string, error) {
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "_", " ")
	text = sp.reSpaces.ReplaceAllString(text, " ")
	return text, nil
}

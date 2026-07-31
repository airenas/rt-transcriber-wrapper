package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/airenas/go-app/pkg/goapp"
	"github.com/airenas/rt-transcriber-wrapper/internal/domain"
	"github.com/airenas/rt-transcriber-wrapper/internal/utils"
	"github.com/rs/zerolog/log"
)

// Punctuator
type Punctuator struct {
	httpclient *http.Client
	getURL     string
	timeout    time.Duration
}

// NewPunctuator creates a punctuation middleware
func NewPunctuator(getURL string) (*Punctuator, error) {
	res := Punctuator{}
	if getURL == "" {
		return nil, fmt.Errorf("no getURL")
	}
	res.getURL = getURL
	res.timeout = time.Second * 10
	res.httpclient = asrHTTPClient()
	goapp.Log.Info().Str("url", getURL).Msg("Punctuator")
	return &res, nil
}

func (sp *Punctuator) Process(ctx context.Context, data *domain.K2Data) (*domain.K2Data, error) {
	defer utils.MeasureTime("punctuator", time.Now())
	if len(data.NewWords) > 0 {
		from := calculateFrom(data)
		text := getText(data.Words[from:])
		punctuated, err := sp.transform(ctx, text)
		if err != nil {
			return nil, err
		}
		if err := setBackPunctuated(data, from, punctuated); err != nil {
			return nil, err
		}
	}
	return data, nil
}

func setBackPunctuated(data *domain.K2Data, from int, punctuated string) error {
	words := parsePunctuated(punctuated)
	if len(words) != len(data.Words[from:]) {
		return fmt.Errorf("words count mismatch. expected: %d, got: %d", len(data.Words[from:]), len(words))
	}
	for i, w := range words {
		word := data.Words[from+i]
		if !strings.HasPrefix(strings.ToLower(w), strings.ToLower(word.Text)) {
			return fmt.Errorf("words mismatch. expected: %s, got: %s", word.Text, w)
		}
		word.Punctuated = w
		word.SentenceEnd = sentenceEnd(w)
	}
	return nil
}

func parsePunctuated(punctuated string) []string {
	res := []string{}
	for _, w := range strings.Split(punctuated, " ") {
		w = strings.TrimSpace(w)
		if len(w) > 0 {
			if w == "-" && len(res) > 0 {
				res[len(res)-1] += w
			} else {
				res = append(res, w)
			}
		}
	}
	return res
}

func calculateFrom(data *domain.K2Data) int {
	if data.FinalTo == 0 {
		return 0
	}
	sentences := 0
	for i := data.FinalTo - 2; i >= 0; i-- {
		if data.Words[i].SentenceEnd {
			sentences++
			if data.FinalTo-i > 20 && sentences >= 2 {
				return i + 1
			}
		}
	}
	return 0
}

func sentenceEnd(word string) bool {
	if len(word) == 0 {
		return false
	}
	lastChar := word[len(word)-1]
	return lastChar == '.' || lastChar == '?' || lastChar == '!'
}

func (sp *Punctuator) transform(ctx context.Context, text string) (string, error) {
	log.Ctx(ctx).Debug().Str("text", text).Msg("punctuating")
	ctx, cancelF := context.WithTimeout(ctx, sp.timeout)
	defer cancelF()

	b := new(bytes.Buffer)
	err := json.NewEncoder(b).Encode(punctRequest{Text: text})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, sp.getURL, b)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)
	resp, err := sp.httpclient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1000))
		_ = resp.Body.Close()
	}()
	if err := goapp.ValidateHTTPResp(resp, 100); err != nil {
		err = fmt.Errorf("can't invoke '%s': %w", req.URL.String(), err)
		return "", err
	}
	res := &punctResponse{}
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return "", err
	}
	log.Ctx(ctx).Debug().Str("result", res.Result).Msg("punctuation result")
	return res.Result, nil
}

type punctRequest struct {
	Text string `json:"text"`
}

type punctResponse struct {
	Text   string `json:"text"`
	Result string `json:"result"`
}

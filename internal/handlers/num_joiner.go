package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/airenas/go-app/pkg/goapp"
	"github.com/airenas/rt-transcriber-wrapper/internal/api"
	"github.com/airenas/rt-transcriber-wrapper/internal/utils"
)

// Joiner communicates with num joiner service service
type Joiner struct {
	httpclient *http.Client
	getURL     string
	timeout    time.Duration
}

// NewClient creates a transcriber client
func NewJoiner(getURL string) (*Joiner, error) {
	res := Joiner{}
	if getURL == "" {
		return nil, fmt.Errorf("no getURL")
	}
	res.getURL = getURL
	res.timeout = time.Second * 3
	res.httpclient = asrHTTPClient()
	goapp.Log.Info().Str("url", getURL).Msg("Joiner")
	return &res, nil
}

func (sp *Joiner) Process(ctx context.Context, data *api.FullResult) (*api.FullResult, error) {
	defer utils.MeasureTime("joiner", time.Now())
	if len(data.Result.Hypotheses) > 0 {
		newText, err := sp.transform(ctx, data.Result.Hypotheses[0].Transcript)
		if err != nil {
			return nil, err
		}
		data.Result.Hypotheses[0].Transcript = newText
	}
	return data, nil
}

func (sp *Joiner) transform(ctx context.Context, text string) (string, error) {
	ctx, cancelF := context.WithTimeout(ctx, sp.timeout)
	defer cancelF()

	b := new(bytes.Buffer)
	err := json.NewEncoder(b).Encode(request{Text: text})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, sp.getURL, b)
	if err != nil {
		return "", err
	}
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
	res := &response{}
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return "", err
	}
	return res.Result, nil
}

type request struct {
	Text string `json:"text"`
}

type response struct {
	Result string `json:"result"`
}

func asrHTTPClient() *http.Client {
	return &http.Client{Transport: newTransport()}
}

func newTransport() http.RoundTripper {
	res := http.DefaultTransport.(*http.Transport).Clone()
	res.MaxConnsPerHost = 5
	res.MaxIdleConns = 2
	res.MaxIdleConnsPerHost = 2
	res.IdleConnTimeout = 90 * time.Second
	return res
}

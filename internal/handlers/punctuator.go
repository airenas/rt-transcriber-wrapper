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
	"github.com/airenas/rt-transcriber-wrapper/internal/api"
	"github.com/airenas/rt-transcriber-wrapper/internal/utils"
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

func (sp *Punctuator) Process(ctx context.Context, data string) (string, error) {
	inData, err := decode(data)
	if err != nil {
		return "", err
	}
	if len(inData.Result.Hypotheses) > 0 {
		ctx, ctxData := utils.CustomContext(ctx)
		str := inData.Result.Hypotheses[0].Transcript
		if inData.Result.Final {
			ctxData.Finals = append(ctxData.Finals, str)
		} else {
			ctxData.PartialResult = str
		}
		goapp.Log.Debug().Str("text", strings.Join(ctxData.Finals, " ") + " " + ctxData.PartialResult).Msg("all text")
		newText, err := sp.transform(ctx, str)
		if err != nil {
			return "", err
		}
		inData.Result.Hypotheses[0].Transcript = newText
	}
	return encode(inData)

}

func decode(data string) (*api.FullResult, error) {
	goapp.Log.Debug().Msg("call punctuator")
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

func (sp *Punctuator) transform(ctx context.Context, text string) (string, error) {
	goapp.Log.Debug().Str("text", text).Msg("punctuating")
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
	goapp.Log.Debug().Str("text", res.PunctuatedText).Msg("punctuation result")
	return res.PunctuatedText, nil
}

type punctRequest struct {
	Text string `json:"text"`
}

type punctResponse struct {
	PunctuatedText string   `json:"punctuatedText"`
	Original       []string `json:"original"`
	Punctuated     []string `json:"punctuated"`
}
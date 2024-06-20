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
	"unicode"

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

// Punctuator
type punctData struct {
	ctxData *utils.CustomData

	segment     int
	original    string
	final       bool
	text        string
	fromSegment int
	fromWord    int
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

func (sp *Punctuator) Process(ctx context.Context, data *api.FullResult) (*api.FullResult, error) {
	defer utils.MeasureTime("punctuator", time.Now())
	if len(data.Result.Hypotheses) > 0 {
		ctx, ctxData := utils.CustomContext(ctx)
		punctData := &punctData{ctxData: ctxData}
		punctData.original = strings.TrimSpace(data.Result.Hypotheses[0].Transcript)
		punctData.segment = data.Segment
		punctData.final = data.Result.Final
		goapp.Log.Debug().Str("text", punctData.original).Int("segment", punctData.segment).Msg("got")
		if punctData.original != "" {
			err := fillPunctData(punctData)
			if err != nil {
				return nil, err
			}
			original, punctuated, err := sp.transform(ctx, punctData.text)
			if err != nil {
				return nil, err
			}
			newText, segments, err := fillPuntResult(punctData, original, punctuated)
			if err != nil {
				return nil, err
			}
			data.Result.Hypotheses[0].Transcript = newText
			data.OldUpdates = segments
		}
	}
	return data, nil
}

func fillPuntResult(punctData *punctData, original []string, punctuated []string) (string, []*api.ShortResult, error) {
	if len(original) != len(punctuated) {
		return "", nil, fmt.Errorf("wrong punctuated data, len(orig): %d, len(punct): %d", len(original), len(punctuated))
	}
	ctxData := punctData.ctxData
	iS, iW := punctData.fromSegment, punctData.fromWord
	i := 0
	changes := make(map[int]bool)
	for i < len(original) {
		if iS >= len(ctxData.Segments) {
			ctxData.Segments = append(ctxData.Segments, &utils.Segments{ID: punctData.segment, Final: false})
		}
		segment := ctxData.Segments[iS]
		if iW >= len(segment.Processed) {
			if segment.Final {
				iS++
				iW = 0
				continue
			} else {
				segment.Processed = append(segment.Processed, &utils.ProcessData{Original: original[i], Punctuated: punctuated[i]})
			}
		} else {
			if segment.Final && segment.Processed[iW].Original != original[i] {
				return "", nil, fmt.Errorf("wrong original word. expected: %s, got: %s", segment.Processed[iW].Original, original[i])
			} else {
				segment.Processed[iW].Original = original[i]
			}
			if segment.Processed[iW].Punctuated != punctuated[i] {
				segment.Processed[iW].Punctuated = punctuated[i]
				if segment.ID != punctData.segment {
					changes[segment.ID] = true
				}
			}
		}
		iW++
		i++
	}
	res := ""
	if len(ctxData.Segments) > 0 {
		ctxData.Segments[len(ctxData.Segments)-1].Final = punctData.final
		res = getSegmentText(ctxData.Segments[len(ctxData.Segments)-1])
	}

	var resOldChanges []*api.ShortResult
	if len(changes) > 0 {
		for _, segment := range ctxData.Segments {
			if changes[segment.ID] {
				resOldChanges = append(resOldChanges, &api.ShortResult{Segment: segment.ID,
					Transcript: getSegmentText(segment), Final: segment.Final})
				goapp.Log.Debug().Int("segment", segment.ID).Msg("changed")	
			}
		}
	}
	return res, resOldChanges, nil
}

func getSegmentText(segments *utils.Segments) string {
	res := strings.Builder{}
	for _, p := range segments.Processed {
		if res.Len() > 0 {
			res.WriteString(" ")
		}
		res.WriteString(p.Punctuated)
	}
	return res.String()
}

func fillPunctData(punctData *punctData) error {
	segments := punctData.ctxData.Segments
	punctData.fromSegment = 0
	punctData.fromWord = 0
	nextWord, nextSegmentIndex, nextWordIndex := "", 0, 0
	words := []string{}
mainLoop:
	for i := len(segments) - 1; i >= 0; i-- {
		segment := segments[i]
		if !segment.Final {
			continue
		}
		for j := len(segment.Processed) - 1; j >= 0; j-- {
			pData := segment.Processed[j]
			if len(words) > 15 && isUpperOrNumber(nextWord) && sentenceEnd(pData.Punctuated) {
				goapp.Log.Debug().Str("word", nextWord).Str("punct", pData.Punctuated).Msg("sentence end")
				break mainLoop
			}
			nextWord = pData.Punctuated
			nextSegmentIndex = i
			nextWordIndex = j
			words = append(words, pData.Original)
		}
	}
	res := strings.Builder{}
	wl := len(words) - 1
	for i := 0; i <= wl; i++ {
		w := words[wl-i]
		if res.Len() > 0 {
			res.WriteString(" ")
		}
		res.WriteString(w)
	}
	if res.Len() > 0 {
		res.WriteString(" ")
	}
	res.WriteString(punctData.original)
	punctData.text = res.String()
	punctData.fromSegment = nextSegmentIndex
	punctData.fromWord = nextWordIndex
	return nil
}

func isUpperOrNumber(word string) bool {
	if len(word) == 0 {
		return false
	}
	firstRune := rune(word[0])
	return unicode.IsUpper(firstRune) || unicode.IsDigit(firstRune)
}

func sentenceEnd(word string) bool {
	if len(word) == 0 {
		return false
	}
	lastChar := word[len(word)-1]
	return lastChar == '.' || lastChar == '?' || lastChar == '!'
}

func (sp *Punctuator) transform(ctx context.Context, text string) ([]string, []string, error) {
	goapp.Log.Debug().Str("text", text).Msg("punctuating")
	ctx, cancelF := context.WithTimeout(ctx, sp.timeout)
	defer cancelF()

	b := new(bytes.Buffer)
	err := json.NewEncoder(b).Encode(punctRequest{Text: text})
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequest(http.MethodPost, sp.getURL, b)
	if err != nil {
		return nil, nil, err
	}
	req = req.WithContext(ctx)
	resp, err := sp.httpclient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1000))
		_ = resp.Body.Close()
	}()
	if err := goapp.ValidateHTTPResp(resp, 100); err != nil {
		err = fmt.Errorf("can't invoke '%s': %w", req.URL.String(), err)
		return nil, nil, err
	}
	res := &punctResponse{}
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, nil, err
	}
	goapp.Log.Debug().Str("text", res.PunctuatedText).Msg("punctuation result")
	return res.Original, res.Punctuated, nil
}

type punctRequest struct {
	Text string `json:"text"`
}

type punctResponse struct {
	PunctuatedText string   `json:"punctuatedText"`
	Original       []string `json:"original"`
	Punctuated     []string `json:"punctuated"`
}

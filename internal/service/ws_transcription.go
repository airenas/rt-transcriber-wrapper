package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/airenas/go-app/pkg/goapp"
	"github.com/airenas/rt-transcriber-wrapper/internal/api"
	"github.com/airenas/rt-transcriber-wrapper/internal/handlers"
	"github.com/gorilla/websocket"
)

type WsConn interface {
	ReadMessage() (messageType int, p []byte, err error)
	Close() error
	WriteJSON(v interface{}) error
}

type Handler interface {
	Process(context.Context, *api.FullResult) (*api.FullResult, error)
}

// WSTranscriptionHandler implements connection management
type WSTranscriptionHandler struct {
	timeOut    time.Duration
	backendURL string
	Middleware Handler
	audioSaver AudioSaver
}

type ConnState struct {
	mu     sync.RWMutex
	active bool
}

type AudioSaver interface {
	SaveAudio(id string, data [][]byte) error
}

// NewWSTranscriptionHandler creates handler
func NewWSTranscriptionHandler(url string, audioSaver AudioSaver) *WSTranscriptionHandler {
	res := &WSTranscriptionHandler{}
	res.timeOut = time.Minute * 5
	res.backendURL = url
	res.audioSaver = audioSaver
	goapp.Log.Info().Str("be url", url).Send()
	return res
}

// HandleConnection loops until connection active and save connection with provided ID as key
func (kp *WSTranscriptionHandler) HandleConnection(ctx context.Context, conn *websocket.Conn, req *http.Request, userID string) error {
	query := req.URL.RawQuery
	goapp.Log.Info().Str("query", query).Msg("got")

	defer conn.Close()
	url := kp.backendURL
	if query != "" {
		url = fmt.Sprintf("%s?%s", url, query)
	}
	goapp.Log.Info().Str("url", kp.backendURL).Msg("deal")

	c, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		return fmt.Errorf("can't dial to URL: %w", err)
	}
	defer c.Close()
	closeCtx, cf := context.WithCancel(ctx)
	defer cf()
	wg := &sync.WaitGroup{}

	writeFunc := func(res *api.FullResult) error {
		msg, err := encode(res)
		if err != nil {
			return err
		}
		return conn.WriteMessage(websocket.TextMessage, []byte(msg))
	}
	session := handlers.NewRecordSession(kp.audioSaver, userID, writeFunc)

	wg.Add(2)

	closeFunc := func() {
		wg.Done()
		cf()
	}

	passForward := func(_ctx context.Context, input *data) (out []*data, in []*data, err error) {
		if input.t != websocket.TextMessage {
			if input.t == websocket.BinaryMessage {
				session.KeepAudio(input.msg)
			}
			out = append(out, input)
			return out, in, nil
		}

		inp := string(input.msg)
		if inp == api.EventStart || inp == api.EventStartAuto {
			session.Start(inp == api.EventStartAuto)
			res := &api.FullResult{
				Event: api.EventStart, TranscriptionID: session.Transcription.ID,
			}
			msg, err := encode(res)
			if err != nil {
				return nil, nil, err
			}
			in = append(in, &data{t: websocket.TextMessage, msg: []byte(msg)})
			return out, in, nil
		}
		if inp == api.EventStop {
			session.Stop()
			return out, in, nil
		}

		out = append(out, input)
		return out, in, nil
	}

	passBackward := func(_ctx context.Context, input *data) (out []*data, in []*data, err error) {
		if input.t != websocket.TextMessage {
			out = append(out, input)
			return out, in, nil
		}

		msg := string(input.msg)
		inpData, err := decode(msg)
		if err != nil {
			goapp.Log.Error().Err(err).Msg("decode err")
			out = append(out, input)
			return out, in, nil
		}

		inpMsgs, err := session.Process(_ctx, inpData, kp.Middleware)
		if err != nil {
			goapp.Log.Error().Err(err).Msg("session err")
			out = append(out, input)
			return out, in, nil
		}
		for _, inpMsg := range inpMsgs {
			goapp.Log.Trace().Interface("msg", inpMsg).Msg("processed")
			res, err := encode(inpMsg)
			if err != nil {
				goapp.Log.Error().Err(err).Msg("encode err")
				continue
			}
			out = append(out, &data{t: websocket.TextMessage, msg: []byte(res)})
		}
		return out, in, nil
	}

	go proxyFunc(ctx, &proxyData{
		in:          conn,
		out:         c,
		forward:     true,
		closeCtx:    closeCtx,
		closeFunc:   closeFunc,
		processFunc: passForward,
	})

	go proxyFunc(ctx, &proxyData{
		in:          c,
		out:         conn,
		forward:     false,
		closeCtx:    closeCtx,
		closeFunc:   closeFunc,
		processFunc: passBackward,
	})

	wg.Wait()
	session.SaveAudio()
	goapp.Log.Info().Msg("handleConnection finish")
	return nil
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

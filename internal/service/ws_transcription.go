package service

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/airenas/go-app/pkg/goapp"
	"github.com/airenas/rt-transcriber-wrapper/internal/api"
	"github.com/airenas/rt-transcriber-wrapper/internal/handlers"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
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
	workers    *WorkerTracker
}

// type ConnState struct {
// 	mu     sync.RWMutex
// 	active bool
// }

type AudioSaver interface {
	SaveAudio(ctx context.Context, id string, data [][]byte) error
}

// NewWSTranscriptionHandler creates handler
func NewWSTranscriptionHandler(url string, audioSaver AudioSaver, workers *WorkerTracker) *WSTranscriptionHandler {
	res := &WSTranscriptionHandler{}
	res.timeOut = time.Minute * 5
	res.backendURL = url
	res.audioSaver = audioSaver
	res.workers = workers
	goapp.Log.Info().Str("be url", url).Send()
	return res
}

// HandleConnection loops until connection active and save connection with provided ID as key
func (kp *WSTranscriptionHandler) HandleConnection(ctx context.Context, conn *websocket.Conn, req *http.Request, userID string) error {
	if kp.workers == nil {
		return fmt.Errorf("worker tracker is not configured")
	}
	if err := kp.workers.Reserve(ctx); err != nil {
		return err
	}
	defer kp.workers.Release(context.Background())
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
	stopRequested := &atomic.Bool{}

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
			out = append(out, &data{t: websocket.BinaryMessage, msg: int16ToFloat32(input.msg)})
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
			session.Stop(_ctx)
			return out, in, nil
		}
		if inp == "EOS" {
			if stopRequested.CompareAndSwap(false, true) {
				log.Ctx(ctx).Info().Msg("EOS received, closing connection after 1 sec")
				// close connection after 1 sec to allow to send final result
				time.AfterFunc(time.Second, cf)
			}
		}
		// skip other msgs
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
	if err := session.SaveAudio(ctx); err != nil {
		goapp.Log.Error().Err(err).Msg("failed to save audio")
	}

	goapp.Log.Info().Msg("handleConnection finish")
	return nil
}

func decode(data string) (*api.FullResult, error) {
	log.Debug().Str("msg", data).Msg("decode")
	resNew := &api.K2Result{}
	err := json.NewDecoder(bytes.NewBufferString(data)).Decode(&resNew)
	if err != nil {
		return nil, err
	}
	res := mapToFullResult(resNew)
	return res, nil
}

func mapToFullResult(resNew *api.K2Result) *api.FullResult {
	return &api.FullResult{
		Status:       0,
		Event:        "TRANSCRIPTION",
		SegmentStart: resNew.StartTime,
		Segment:      resNew.Segment,
		Result: api.Result{
			Hypotheses: []api.Hypothesis{
				{
					Transcript: resNew.Text,
				},
			},
			Final: resNew.IsFinal,
		},
	}
}

func encode(inData *api.FullResult) (string, error) {
	b := new(bytes.Buffer)
	if err := json.NewEncoder(b).Encode(inData); err != nil {
		return "", err
	}
	return b.String(), nil
}

func int16ToFloat32(data []byte) []byte {
	// bytes -> int16 samples
	samples := make([]int16, len(data)/2)

	for i := 0; i < len(samples); i++ {
		samples[i] = int16(binary.LittleEndian.Uint16(data[i*2:]))
	}

	// int16 -> float32
	floats := make([]float32, len(samples))
	for i, s := range samples {
		floats[i] = float32(s) / 32768.0
	}

	// float32 -> bytes
	out := make([]byte, len(floats)*4)
	for i, f := range floats {
		binary.LittleEndian.PutUint32(
			out[i*4:],
			math.Float32bits(f),
		)
	}

	return out
}

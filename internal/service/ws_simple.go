package service

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/airenas/go-app/pkg/goapp"
	"github.com/gorilla/websocket"
)

type WSSimpleHandler struct {
	timeOut    time.Duration
	backendURL string
}

// NewWSSimpleHandler creates simple proxy handler
func NewWSSimpleHandler(url string) *WSSimpleHandler {
	res := &WSSimpleHandler{}
	res.timeOut = time.Minute * 5
	res.backendURL = url
	goapp.Log.Info().Str("be url", url).Send()
	return res
}

// HandleConnection loops until connection active and save connection with provided ID as key
func (kp *WSSimpleHandler) HandleConnection(ctx context.Context, conn *websocket.Conn, req *http.Request) error {
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
	wg.Add(2)
	closeFunc := func() {
		wg.Done()
		cf()
	}

	go proxyFunc(ctx, &proxyData{
		in:          conn,
		out:         c,
		forward:     true,
		closeCtx:    closeCtx,
		closeFunc:   closeFunc,
		processFunc: pass,
	})
	go proxyFunc(ctx, &proxyData{
		in:          c,
		out:         conn,
		forward:     false,
		closeCtx:    closeCtx,
		closeFunc:   closeFunc,
		processFunc: pass,
	})

	wg.Wait()
	goapp.Log.Info().Msg("handleConnection finished")
	return nil
}

func pass(ctx context.Context, input *data) (out []*data, in []*data, err error) {
	out = []*data{input}
	return out, in, nil
}

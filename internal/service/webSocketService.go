package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/airenas/go-app/pkg/goapp"
	"github.com/airenas/rt-transcriber-wrapper/internal/utils"
	"github.com/gorilla/websocket"
)

type WsConn interface {
	ReadMessage() (messageType int, p []byte, err error)
	Close() error
	WriteJSON(v interface{}) error
}

type Handler interface {
	Process(context.Context, string) (string, error)
}

// WSConnKeeper implements connection management
type WSConnHandler struct {
	timeOut    time.Duration
	backendURL string
	Middleware Handler
}

// NewWSConnKeeper creates manager
func NewWSHandler(url string) *WSConnHandler {
	res := &WSConnHandler{}
	res.timeOut = time.Minute * 5
	res.backendURL = url
	goapp.Log.Info().Str("be url", url).Send()
	return res
}

// HandleConnection loops until connection active and save connection with provided ID as key
func (kp *WSConnHandler) HandleConnection(ctx context.Context, conn *websocket.Conn, query string) error {
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
	proxyF := func(in, out *websocket.Conn, forward bool, handler Handler) {
		defer wg.Done()
		defer cf()
		readCh := readWebSocket(ctx, in)
		ctx, _ := utils.CustomContext(ctx)
		for {
			var d data
			var ok bool
			select {
			case <-closeCtx.Done():
				goapp.Log.Info().Msg("context canceled")
				return
			case d, ok = <-readCh:
				if !ok {
					goapp.Log.Info().Msg("channel closed")
					return
				}
			}
			goapp.Log.Debug().Bool("forward", forward).Int("type", d.t).Send()
			if d.t == websocket.TextMessage && handler != nil {
				goapp.Log.Debug().Str("msg", string(d.msg)).Send()
				res, err := handler.Process(ctx, string(d.msg))
				if err != nil {
					goapp.Log.Error().Err(err).Msg("handler err")
				} else {
					d.msg = []byte(res)
				}
			}
			if err := out.WriteMessage(d.t, d.msg); err != nil {
				goapp.Log.Error().Err(err).Msg("write error")
				return
			}
		}

	}
	wg.Add(2)
	go proxyF(c, conn, false, kp.Middleware)
	go proxyF(conn, c, true, nil)
	wg.Wait()
	goapp.Log.Info().Msg("handleConnection finish")
	return nil
}

type data struct {
	t   int
	msg []byte
}

func readWebSocket(ctx context.Context, in *websocket.Conn) <-chan data {
	resCh := make(chan data)
	go func() {
		defer close(resCh)
		defer goapp.Log.Debug().Msg("read routine ended")
		for {
			goapp.Log.Debug().Msg("handleConnection")
			mType, message, err := in.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseAbnormalClosure) || errors.Is(err, net.ErrClosed) {
					goapp.Log.Info().Msg("connection closed")
					return
				}
				goapp.Log.Error().Err(err).Send()
				return
			}
			msg := data{t: mType, msg: message}

			select {
			case resCh <- msg:
				timer := time.NewTimer(20 * time.Millisecond)
				select {
				case <-timer.C:
				case <-ctx.Done():
					timer.Stop()
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return resCh
}

package service

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/airenas/go-app/pkg/goapp"
	"github.com/gorilla/websocket"
)

type data struct {
	t   int
	msg []byte
}

type proxyData struct {
	in          *websocket.Conn
	closeCtx    context.Context
	out         *websocket.Conn
	forward     bool
	closeFunc   func()
	processFunc func(ctx context.Context, input *data) (out []*data, in []*data, err error)
}

func proxyFunc(ctx context.Context, prData *proxyData) {
	defer prData.closeFunc()
	readCh := readWebSocket(ctx, prData.in)
loop:

	for {
		var d data
		var ok bool
		select {
		case <-prData.closeCtx.Done():
			goapp.Log.Info().Msg("context canceled")
			break loop
		case d, ok = <-readCh:
			goapp.Log.Debug().Bool("forward", prData.forward).Int("type", d.t).Send()
			if d.t == websocket.TextMessage {
				goapp.Log.Trace().Str("msg", string(d.msg)).Send()
			}
			if !ok {
				goapp.Log.Info().Msg("channel closed")
				break loop
			}
			outs, ins, err := prData.processFunc(ctx, &d)
			if err != nil {
				goapp.Log.Error().Err(err).Msg("process error")
				break loop
			}
			for _, out := range outs {
				if err := prData.out.WriteMessage(out.t, out.msg); err != nil {
					goapp.Log.Error().Err(err).Msg("write error")
					break loop
				}
			}
			for _, in := range ins {
				if err := prData.in.WriteMessage(in.t, in.msg); err != nil {
					goapp.Log.Error().Err(err).Msg("write error")
					break loop
				}
			}

		}
		goapp.Log.Debug().Bool("forward", prData.forward).Msg("proxy finished")
	}
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
				if websocket.IsCloseError(err, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) ||
					errors.Is(err, net.ErrClosed) {
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

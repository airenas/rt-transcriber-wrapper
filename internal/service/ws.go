package service

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
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
			log.Ctx(ctx).Info().Msg("context canceled")
			break loop
		case d, ok = <-readCh:
			log.Ctx(ctx).Trace().Bool("forward", prData.forward).Int("type", d.t).Send()
			if d.t == websocket.TextMessage {
				log.Ctx(ctx).Trace().Str("msg", string(d.msg)).Send()
			}
			if !ok {
				log.Ctx(ctx).Info().Msg("channel closed")
				break loop
			}
			outs, ins, err := prData.processFunc(ctx, &d)
			if err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("process error")
				break loop
			}
			for _, out := range outs {
				if err := prData.out.WriteMessage(out.t, out.msg); err != nil {
					log.Ctx(ctx).Error().Err(err).Msg("write error")
					break loop
				}
			}
			for _, in := range ins {
				if err := prData.in.WriteMessage(in.t, in.msg); err != nil {
					log.Ctx(ctx).Error().Err(err).Msg("write error")
					break loop
				}
			}

		}
		log.Ctx(ctx).Trace().Bool("forward", prData.forward).Msg("proxy finished")
	}
}

func readWebSocket(ctx context.Context, in *websocket.Conn) <-chan data {
	resCh := make(chan data)
	go func() {
		defer close(resCh)
		defer log.Ctx(ctx).Debug().Msg("read routine ended")
		for {
			log.Ctx(ctx).Trace().Msg("handleConnection")
			mType, message, err := in.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) ||
					errors.Is(err, net.ErrClosed) {
					log.Ctx(ctx).Info().Msg("connection closed")
					return
				}
				log.Ctx(ctx).Error().Err(err).Send()
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

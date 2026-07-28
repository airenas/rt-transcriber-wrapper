package service

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/airenas/go-app/pkg/goapp"
	"github.com/gorilla/websocket"
)

type WSStatusHandler struct {
	readTimeout  time.Duration
	pingInterval time.Duration
	workers      *WorkerTracker
}

// NewWSStatusHandler creates a status service handler that publishes worker capacity snapshots.
func NewWSStatusHandler(workers *WorkerTracker) *WSStatusHandler {
	res := &WSStatusHandler{workers: workers}
	res.readTimeout = 90 * time.Second
	res.pingInterval = 30 * time.Second
	return res
}

// HandleConnection streams worker status updates to the connected client.
func (kp *WSStatusHandler) HandleConnection(ctx context.Context, conn *websocket.Conn, req *http.Request, _userID string) error {
	goapp.Log.Debug().Str("query", req.URL.RawQuery).Msg("status ws connected")
	defer conn.Close()

	if kp.workers == nil {
		return errors.New("worker tracker is not configured")
	}

	updates, cancelSub := kp.workers.Subscribe(ctx)
	defer cancelSub()

	readErrCh := make(chan error, 1)
	conn.SetReadLimit(1024)
	if err := conn.SetReadDeadline(time.Now().Add(kp.readTimeout)); err != nil {
		return err
	}
	conn.SetPongHandler(func(_ string) error {
		return conn.SetReadDeadline(time.Now().Add(kp.readTimeout))
	})

	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				readErrCh <- err
				return
			}
		}
	}()

	ticker := time.NewTicker(kp.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			goapp.Log.Debug().Msg("status ws context canceled")
			return nil
		case err := <-readErrCh:
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return nil
			}
			return err
		case snapshot, ok := <-updates:
			if !ok {
				return nil
			}
			if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
				return err
			}
			if err := conn.WriteJSON(snapshot); err != nil {
				return err
			}
		case now := <-ticker.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, now.Add(5*time.Second)); err != nil {
				return err
			}
		}
	}
}


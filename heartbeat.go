package gmqtt

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type HeartBeat struct {
	ticks        time.Duration
	timer        *time.Timer
	lastSent     time.Time
	lastResponse time.Time
	mu           sync.Mutex
	pingTrigger  func(ctx context.Context)
	pingLost     func(ctx context.Context)
	w            sync.WaitGroup
	logger       slog.Logger
}

func (h *HeartBeat) Start(ctx context.Context, ka time.Duration) {
	if ka == 0 {
		return
	}
	h.w.Add(1)
	h.lastResponse = time.Now()
	h.lastSent = time.Now()
	h.ticks = ka / 2
	h.timer = time.NewTimer(h.ticks)
	go func() {
		defer h.w.Done()
		defer h.timer.Stop()
		for {
			select {
			case <-h.timer.C:
				now := time.Now()
				h.logger.Info("timer ticks")
				h.mu.Lock()
				lastResponse := h.lastResponse
				lastSent := h.lastSent
				h.mu.Unlock()

				if lastSent.Add(h.ticks).Before(now) {
					h.logger.Info("sending a ping")
					h.pingTrigger(ctx)

					h.mu.Lock()
					h.lastSent = now
					h.timer.Reset(h.ticks)
					h.mu.Unlock()

				} else {
					h.mu.Lock()
					if !h.timer.Stop() {
						select {
						case <-h.timer.C:
						default:
						}
					}
					h.timer.Reset(h.ticks)
					h.mu.Unlock()
				}

				if lastResponse.Add(ka).Before(now) {
					h.logger.Info("heartbeat lost")
					h.pingLost(ctx)
					return
				}

			case <-ctx.Done():
				return
			}
		}
	}()
}

func (h *HeartBeat) Stop() {
	if h.timer != nil {
		h.timer.Stop()
	}
	h.w.Wait()
}

func (h *HeartBeat) PacketSent() {
	if h.ticks == 0 {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.lastSent = time.Now()
}

func (h *HeartBeat) PacketReceived() {
	if h.ticks == 0 {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.lastResponse = time.Now()
}

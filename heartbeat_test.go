package gmqtt

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

type heartBeatTest struct {
	pingTriggerCalled bool
	pingLostCalled    bool
	mu                sync.Mutex
	hb                *HeartBeat
	cancel            context.CancelFunc
	ctx               context.Context
}

func setupHeartBeat() *heartBeatTest {
	ctx, cancel := context.WithCancel(context.Background())
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hbt := &heartBeatTest{
		cancel: cancel,
		ctx:    ctx,
	}
	hbt.hb = &HeartBeat{
		logger: *logger,
		pingTrigger: func(ctx context.Context) {
			hbt.mu.Lock()
			hbt.pingTriggerCalled = true
			hbt.mu.Unlock()
		},
		pingLost: func(ctx context.Context) {
			hbt.mu.Lock()
			hbt.pingLostCalled = true
			hbt.mu.Unlock()
			hbt.cancel()
		},
	}
	return hbt
}

func TestHeartBeat_Start_Stop(t *testing.T) {
	tests := []struct {
		name       string
		keepAlive  time.Duration
		actions    func(hbt *heartBeatTest, ctx context.Context)
		assertions func(hbt *heartBeatTest, t *testing.T)
	}{
		{
			name:      "Keep-alive 0, no triggers",
			keepAlive: 0,
			actions: func(hbt *heartBeatTest, ctx context.Context) {
				hbt.hb.Start(ctx, 0)
				time.Sleep(2 * time.Second)
			},
			assertions: func(hbt *heartBeatTest, t *testing.T) {
				hbt.mu.Lock()
				defer hbt.mu.Unlock()
				if hbt.pingTriggerCalled || hbt.pingLostCalled {
					t.Error("Heartbeat should not trigger when keep-alive is 0")
				}
			},
		},
		{
			name:      "Normal operation, ping trigger and packet received",
			keepAlive: 100 * time.Millisecond,
			actions: func(hbt *heartBeatTest, ctx context.Context) {
				hbt.hb.Start(ctx, 100*time.Millisecond)
				time.Sleep(60 * time.Millisecond)
				hbt.mu.Lock()
				if !hbt.pingTriggerCalled {
					t.Error("Expected pingTrigger to be called within 60ms")
				}
				hbt.pingTriggerCalled = false
				hbt.mu.Unlock()
				hbt.hb.PacketReceived()
				time.Sleep(60 * time.Millisecond)
				hbt.cancel()
			},
			assertions: func(hbt *heartBeatTest, t *testing.T) {
				hbt.mu.Lock()
				defer hbt.mu.Unlock()
				if !hbt.pingTriggerCalled {
					t.Error("Expected pingTrigger to be called again after packet received")
				}
			},
		},
		{
			name:      "Ping lost after timeout",
			keepAlive: 100 * time.Millisecond,
			actions: func(hbt *heartBeatTest, ctx context.Context) {
				hbt.hb.Start(ctx, 100*time.Millisecond)
				time.Sleep(200 * time.Millisecond)
			},
			assertions: func(hbt *heartBeatTest, t *testing.T) {
				hbt.mu.Lock()
				defer hbt.mu.Unlock()
				if !hbt.pingLostCalled {
					t.Error("Expected pingLost to be called after keep-alive timeout")
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			hbt := setupHeartBeat()
			defer hbt.cancel()
			defer hbt.hb.Stop()

			tt.actions(hbt, hbt.ctx)
			tt.assertions(hbt, t)
		})
	}
}

func TestHeartBeat_PacketSent_Received(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := &HeartBeat{
		logger: *logger,
	}

	h.ticks = 0
	h.PacketSent()
	h.PacketReceived()

	h.ticks = 100 * time.Millisecond
	now := time.Now()

	h.lastSent = now.Add(-50 * time.Millisecond)
	h.lastResponse = now.Add(-50 * time.Millisecond)

	h.PacketSent()
	if !h.lastSent.After(now) {
		t.Error("PacketSent should update lastSent to current time")
	}

	h.PacketReceived()
	if !h.lastResponse.After(now) {
		t.Error("PacketReceived should update lastResponse to current time")
	}
}

func TestHeartBeat_ConcurrentAccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := &HeartBeat{
		logger: *logger,
	}

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				h.PacketSent()
				h.PacketReceived()
			}
		}()
	}
	wg.Wait()
}

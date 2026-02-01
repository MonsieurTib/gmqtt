package gmqtt

import (
	"log/slog"
	"testing"
	"time"

	"github.com/MonsieurTib/gmqtt/internal/protocol"
)

func setupSession(t *testing.T) *session {
	t.Helper()
	return newSession(65535, slog.Default())
}

func storePublishWithExpiry(t *testing.T, s *session, topic string, expirySeconds uint32) (uint16, chan *protocol.PubAckPacket) {
	t.Helper()
	expiry := expirySeconds
	pub, err := protocol.NewPublish(protocol.PublishOptions{
		Qos:   1,
		Topic: topic,
		PublishProperties: &protocol.PublishProperties{
			MessageExpiryInterval: &expiry,
		},
		Payload: []byte("test"),
	})
	if err != nil {
		t.Fatalf("failed to create publish: %v", err)
	}

	id, ch, err := s.storePublish(pub)
	if err != nil {
		t.Fatalf("failed to store publish: %v", err)
	}
	return id, ch
}

func storePublishWithoutExpiry(t *testing.T, s *session, topic string) (uint16, chan *protocol.PubAckPacket) {
	t.Helper()
	pub, err := protocol.NewPublish(protocol.PublishOptions{
		Qos:     1,
		Topic:   topic,
		Payload: []byte("test"),
	})
	if err != nil {
		t.Fatalf("failed to create publish: %v", err)
	}

	id, ch, err := s.storePublish(pub)
	if err != nil {
		t.Fatalf("failed to store publish: %v", err)
	}
	return id, ch
}

func uint32Ptr(v uint32) *uint32 { return &v }

type inflightTestMsg struct {
	topic             string
	expirySeconds     *uint32
	backdateBy        time.Duration
	replaceWithPubRel bool
}

type inflightTestState struct {
	ids      []uint16
	channels []chan *protocol.PubAckPacket
}

func TestSessionGetInflightMessages(t *testing.T) {
	tests := []struct {
		name          string
		description   string
		messages      []inflightTestMsg
		expectedCount int
		assertions    func(t *testing.T, s *session, messages []protocol.PubPacket, state *inflightTestState)
	}{
		{
			name:        "drops_expired_messages",
			description: "Messages past their expiry should be dropped and removed from inflight map",
			messages: []inflightTestMsg{
				{topic: "test/expired", expirySeconds: uint32Ptr(1), backdateBy: 2 * time.Second},
			},
			expectedCount: 0,
			assertions: func(t *testing.T, s *session, _ []protocol.PubPacket, state *inflightTestState) {
				s.mu.Lock()
				_, exists := s.inflightMap[state.ids[0]]
				count := s.inflightMessages.Count()
				s.mu.Unlock()

				if exists {
					t.Error("expired message should be removed from inflight map")
				}
				if count != 0 {
					t.Errorf("expected 0 messages in linked list, got %d", count)
				}
			},
		},
		{
			name:        "no_expiry_messages_unaffected",
			description: "Messages without expiry should never expire regardless of elapsed time",
			messages: []inflightTestMsg{
				{topic: "test/no-expiry", backdateBy: 1 * time.Hour},
			},
			expectedCount: 1,
			assertions: func(t *testing.T, _ *session, messages []protocol.PubPacket, _ *inflightTestState) {
				pub, ok := messages[0].(*protocol.Publish)
				if !ok {
					t.Fatal("expected message to be *protocol.Publish")
				}

				if pub.Properties() != nil && pub.Properties().MessageExpiryInterval != nil {
					t.Error("message without expiry should not gain an expiry interval")
				}
			},
		},
		{
			name:        "mixed_expiry_and_non_expiry",
			description: "Only expired messages should be dropped; surviving and no-expiry messages should remain",
			messages: []inflightTestMsg{
				{topic: "test/expired", expirySeconds: uint32Ptr(1), backdateBy: 5 * time.Second},
				{topic: "test/surviving", expirySeconds: uint32Ptr(60), backdateBy: 10 * time.Second},
				{topic: "test/no-expiry", backdateBy: 1 * time.Hour},
			},
			expectedCount: 2,
			assertions: func(t *testing.T, s *session, _ []protocol.PubPacket, state *inflightTestState) {
				s.mu.Lock()
				_, expiredExists := s.inflightMap[state.ids[0]]
				_, survivingExists := s.inflightMap[state.ids[1]]
				_, noExpiryExists := s.inflightMap[state.ids[2]]
				s.mu.Unlock()

				if expiredExists {
					t.Error("expired message should be removed from inflight map")
				}
				if !survivingExists {
					t.Error("surviving message should remain in inflight map")
				}
				if !noExpiryExists {
					t.Error("no-expiry message should remain in inflight map")
				}
			},
		},
		{
			name:        "expiry_at_exact_boundary",
			description: "Messages at exactly the expiry boundary (elapsed >= expiry) should be dropped",
			messages: []inflightTestMsg{
				{topic: "test/boundary", expirySeconds: uint32Ptr(5), backdateBy: 5 * time.Second},
			},
			expectedCount: 0,
		},
		{
			name:        "pubrel_not_affected_by_expiry",
			description: "PubRel messages should not expire since the QoS 2 flow must complete",
			messages: []inflightTestMsg{
				{topic: "test/qos2", expirySeconds: uint32Ptr(1), backdateBy: 5 * time.Second, replaceWithPubRel: true},
			},
			expectedCount: 1,
			assertions: func(t *testing.T, _ *session, messages []protocol.PubPacket, _ *inflightTestState) {
				if _, ok := messages[0].(*protocol.PubRel); !ok {
					t.Error("expected message to be *protocol.PubRel")
				}
			},
		},
		{
			name:        "expired_message_channel_closed",
			description: "Response channels of expired messages should be closed so callers don't block",
			messages: []inflightTestMsg{
				{topic: "test/chan-close", expirySeconds: uint32Ptr(1), backdateBy: 2 * time.Second},
			},
			expectedCount: 0,
			assertions: func(t *testing.T, _ *session, _ []protocol.PubPacket, state *inflightTestState) {
				select {
				case _, ok := <-state.channels[0]:
					if ok {
						t.Error("expected response channel to be closed, but received a value")
					}
				default:
					t.Error("expected response channel to be closed, but it was still open/blocking")
				}
			},
		},
		{
			name:          "empty_session",
			description:   "An empty session should return no messages",
			messages:      nil,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setupSession(t)

			state := &inflightTestState{}
			for _, msg := range tt.messages {
				var id uint16
				var ch chan *protocol.PubAckPacket

				if msg.expirySeconds != nil {
					id, ch = storePublishWithExpiry(t, s, msg.topic, *msg.expirySeconds)
				} else {
					id, ch = storePublishWithoutExpiry(t, s, msg.topic)
				}

				if msg.replaceWithPubRel {
					pubRel, err := protocol.NewPubRel(id, protocol.PubQosStepSuccess, nil)
					if err != nil {
						t.Fatalf("failed to create pubrel: %v", err)
					}
					s.rec(id, pubRel)
				}

				if msg.backdateBy > 0 {
					s.mu.Lock()
					s.inflightMap[id].sentAt = time.Now().Add(-msg.backdateBy)
					s.mu.Unlock()
				}

				state.ids = append(state.ids, id)
				state.channels = append(state.channels, ch)
			}

			messages := s.getInflightMessages()

			if len(messages) != tt.expectedCount {
				t.Fatalf("%s: expected %d messages, got %d", tt.description, tt.expectedCount, len(messages))
			}

			if tt.assertions != nil {
				tt.assertions(t, s, messages, state)
			}
		})
	}
}

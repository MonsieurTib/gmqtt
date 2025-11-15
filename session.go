package gmqtt

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/MonsieurTib/gmqtt/internal/collection"
	"github.com/MonsieurTib/gmqtt/internal/protocol"
)

type inflightMessage struct {
	node     *collection.Node[*protocol.Publish]
	respChan chan protocol.Packet
	sentAt   time.Time
}

type session struct {
	inflightMessages *collection.DoubleLinkedList[*protocol.Publish]
	inflightMap      map[uint16]*inflightMessage
	nextID           uint16
	mu               sync.Mutex
	receiveMaximum   uint16
	logger           *slog.Logger
}

func newSession(receiveMaximum uint16, logger *slog.Logger) *session {
	max := receiveMaximum
	if max == 0 {
		max = 65535
	}

	session := &session{
		inflightMessages: &collection.DoubleLinkedList[*protocol.Publish]{},
		inflightMap:      make(map[uint16]*inflightMessage, max),
		nextID:           1,
		receiveMaximum:   max,
		logger:           logger,
	}

	return session
}

func (s *session) storePublish(p *protocol.Publish) (uint16, chan protocol.Packet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.inflightMap) >= int(s.receiveMaximum) {
		return 0, nil, fmt.Errorf("cannot store more packets: receive maximum reached")
	}

	id := s.allocID()
	p.PacketID = &id

	node := s.inflightMessages.Add(p)
	respChan := make(chan protocol.Packet, 1)

	s.inflightMap[id] = &inflightMessage{
		node:     node,
		respChan: respChan,
		sentAt:   time.Now(),
	}

	return id, respChan, nil
}

func (s *session) ack(packetID uint16, response protocol.Packet) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	msg, exists := s.inflightMap[packetID]
	if !exists {
		s.logger.Warn("ack received for unknown packet id", "packet_id", packetID)
		return false
	}

	s.inflightMessages.Remove(msg.node)
	msg.respChan <- response
	close(msg.respChan)

	delete(s.inflightMap, packetID)

	s.logger.Debug("packet acknowledged and removed from session",
		"packet_id", packetID,
		"duration_ms", time.Since(msg.sentAt).Milliseconds())

	return true
}

func (s *session) remove(packetID uint16) {
	s.mu.Lock()
	defer s.mu.Unlock()

	msg, exists := s.inflightMap[packetID]
	if !exists {
		return
	}

	s.inflightMessages.Remove(msg.node)
	close(msg.respChan)
	delete(s.inflightMap, packetID)
}

func (s *session) allocID() uint16 {
	for {
		id := s.nextID
		s.nextID++
		if s.nextID > s.receiveMaximum {
			s.nextID = 1
		}
		if _, used := s.inflightMap[id]; !used {
			return id
		}
	}
}

func (s *session) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, msg := range s.inflightMap {
		close(msg.respChan)
		delete(s.inflightMap, id)
	}

	s.inflightMessages = &collection.DoubleLinkedList[*protocol.Publish]{}
}

func (s *session) getInflightMessages() []*protocol.Publish {
	s.mu.Lock()
	defer s.mu.Unlock()

	messages := make([]*protocol.Publish, 0, s.inflightMessages.Count())

	current := s.inflightMessages.Head
	for current != nil {
		messages = append(messages, current.Data())
		current = current.Next
	}

	return messages
}

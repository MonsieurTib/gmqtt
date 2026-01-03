package gmqtt

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/MonsieurTib/gmqtt/internal/protocol"
)

var (
	ErrConnackUnspecifiedError           = errors.New("unspecified error")
	ErrConnackMalformedPacket            = errors.New("malformed packet")
	ErrConnackProtocolError              = errors.New("protocol error")
	ErrConnackImplementationError        = errors.New("implementation error")
	ErrConnackUnsupportedProtocolVersion = errors.New("unsupported protocol version")
	ErrConnackClientIdentifierNotValid   = errors.New("client identifier not valid")
	ErrConnackBadUsernameOrPassword      = errors.New("bad username or password")
	ErrConnackNotAuthorized              = errors.New("not authorized")
	ErrConnackServerUnavailable          = errors.New("server unavailable")
	ErrConnackServerBusy                 = errors.New("server busy")
	ErrConnackBanned                     = errors.New("banned")
	ErrConnackBadAuthenticationMethod    = errors.New("bad authentication method")
	ErrConnackTopicNameInvalid           = errors.New("topic name invalid")
	ErrConnackPacketToLarge              = errors.New("packet too large")
	ErrConnackQuotaExceeded              = errors.New("quota exceeded")
	ErrConnackPayloadFormatInvalid       = errors.New("payload format invalid")
	ErrConnackRetainNotSupported         = errors.New("retain not supported")
	ErrConnackQosNotSupported            = errors.New("qos not supported")
	ErrConnackUseAnotherServer           = errors.New("use another server")
	ErrConnackServerMoved                = errors.New("server moved")
	ErrConnackConnectionRateExceeded     = errors.New("connection rate exceeded")
)

var ConnackReasonErrors = map[byte]error{
	protocol.ConnackUnspecifiedError:           ErrConnackUnspecifiedError,
	protocol.ConnackMalformedPacket:            ErrConnackMalformedPacket,
	protocol.ConnackProtocolError:              ErrConnackProtocolError,
	protocol.ConnackImplementationError:        ErrConnackImplementationError,
	protocol.ConnackUnsupportedProtocolVersion: ErrConnackUnsupportedProtocolVersion,
	protocol.ConnackClientIdentifierNotValid:   ErrConnackClientIdentifierNotValid,
	protocol.ConnackBadUsernameOrPassword:      ErrConnackBadUsernameOrPassword,
	protocol.ConnackNotAuthorized:              ErrConnackNotAuthorized,
	protocol.ConnackServerUnavailable:          ErrConnackServerUnavailable,
	protocol.ConnackServerBusy:                 ErrConnackServerBusy,
	protocol.ConnackBanned:                     ErrConnackBanned,
	protocol.ConnackBadAuthenticationMethod:    ErrConnackBadAuthenticationMethod,
	protocol.ConnackTopicNameInvalid:           ErrConnackTopicNameInvalid,
	protocol.ConnackPacketToLarge:              ErrConnackPacketToLarge,
	protocol.ConnackQuotaExceeded:              ErrConnackQuotaExceeded,
	protocol.ConnackPayloadFormatInvalid:       ErrConnackPayloadFormatInvalid,
	protocol.ConnackRetainNotSupported:         ErrConnackRetainNotSupported,
	protocol.ConnackQosNotSupported:            ErrConnackQosNotSupported,
	protocol.ConnackUseAnotherServer:           ErrConnackUseAnotherServer,
	protocol.ConnackServerMoved:                ErrConnackServerMoved,
	protocol.ConnackConnectionRateExceeded:     ErrConnackConnectionRateExceeded,
}

type QoS byte

const (
	QoSAtMostOnce QoS = iota
	QoSAtLeastOnce
	QoSExactlyOnce
)

type UserProperty struct {
	Key, Value string
}

type ClientConfig struct {
	Broker    string
	ClientID  string
	Username  string
	Password  string
	KeepAlive time.Duration
	Timeout   time.Duration

	CleanStart        bool
	ConnectProperties *ConnectProperties
	Will              *WillConfig
	OnConnectionLost  func(error)
	Logger            *slog.Logger

	Network     string
	Dialer      *net.Dialer
	TLSConfig   *tls.Config
	DialContext func(ctx context.Context, network, address string) (net.Conn, error)
}

type WillConfig struct {
	Topic      string
	Message    string
	QoS        QoS
	Retain     bool
	Properties *WillProperties
}

type ConnectProperties struct {
	SessionExpiryInterval      uint32
	ReceiveMaximum             uint16
	MaximumPacketSize          uint32
	TopicAliasMaximum          uint16
	RequestResponseInformation byte
	RequestProblemInformation  byte
	UserProperties             []UserProperty
	AuthenticationMethod       string
	AuthenticationData         []byte
}

type WillProperties struct {
	WillDelayInterval      uint32
	PayloadFormatIndicator bool
	MessageExpiryInterval  uint32
	ContentType            string
	ResponseTopic          string
	CorrelationData        []byte
	UserProperties         []UserProperty
}

type Publish struct {
	Qos     QoS
	Topic   string
	Payload []byte
	Retain  bool
}

type PublishResponse struct {
	ReasonCode   byte
	ReasonString string
}

type Subscription struct {
	TopicFilter       string
	QoS               QoS
	NoLocal           bool
	RetainAsPublished bool
	RetainHandling    byte
}

type SubscribeResponse struct {
	ReasonCodes  map[string]byte
	ReasonString string
}

type Client struct {
	config        *ClientConfig
	conn          net.Conn
	connectSignal chan *protocol.ConnAck
	mu            sync.Mutex
	wg            sync.WaitGroup
	cancelFunc    context.CancelFunc
	connected     bool
	heartBeat     *HeartBeat
	session       *session
}

func NewClient(config *ClientConfig) (*Client, error) {
	if config == nil {
		return nil, fmt.Errorf("client configuration is required")
	}

	config.setDefaults()

	if config.ConnectProperties == nil {
		config.ConnectProperties = &ConnectProperties{}
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid client configuration: %w", err)
	}

	c := &Client{
		config:        config,
		connectSignal: make(chan *protocol.ConnAck, 1),
	}
	c.heartBeat = &HeartBeat{
		logger: *config.Logger,
		pingTrigger: func(ctx context.Context) {
			c.sendPacket(ctx, &protocol.PingReq{})
		},
		pingLost: func(ctx context.Context) {
			c.Disconnect(ctx)
		},
	}
	return c, nil
}

func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, err := c.dial(ctx)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	c.conn = conn

	ctx, cancel := context.WithCancel(ctx)
	c.cancelFunc = cancel
	c.wg.Add(1)

	go c.readLoop(ctx)

	var willTopic, willMessage string
	var willQoS byte
	var willRetain bool

	var willProperties *protocol.WillProperties

	if c.config.Will != nil {
		willTopic = c.config.Will.Topic
		willMessage = c.config.Will.Message
		willQoS = byte(c.config.Will.QoS)
		willRetain = c.config.Will.Retain
		willProperties = convertWillProperties(c.config.Will.Properties)
	}

	options := protocol.ConnectOptions{
		KeepAlive:         uint16(c.config.KeepAlive.Seconds()),
		ClientID:          c.config.ClientID,
		CleanStart:        c.config.CleanStart,
		WillTopic:         willTopic,
		WillMessage:       willMessage,
		WillQoS:           willQoS,
		WillRetain:        willRetain,
		WillProperties:    willProperties,
		Username:          c.config.Username,
		Password:          c.config.Password,
		ConnectProperties: convertConnectProperties(c.config.ConnectProperties),
	}

	connect := protocol.NewConnect(options)
	packet, err := connect.Encode()
	if err != nil {
		c.close()
		return fmt.Errorf("failed to encode connect packet: %w", err)
	}
	_, err = packet.WriteTo(c.conn)
	if err != nil {
		return err
	}

	select {
	case connack := <-c.connectSignal:
		if connack.ReasonCode != protocol.ConnackSuccess {
			c.close()
			if err, exists := ConnackReasonErrors[connack.ReasonCode]; exists {
				return fmt.Errorf("connection refused: %w", err)
			}
			return fmt.Errorf("connection refused: unknown reason code 0x%02X", connack.ReasonCode)
		}
		c.config.Logger.Info("connection accepted",
			"session_present", connack.SessionPresent,
			"connack", connack)
		c.connected = true

		if connack.Properties.ServerKeepAlive != nil {
			c.config.KeepAlive = time.Duration(*connack.Properties.ServerKeepAlive) * time.Second
		}
		if connack.Properties.ReceiveMaximum != nil {
			c.config.ConnectProperties.ReceiveMaximum = *connack.Properties.ReceiveMaximum
		} else {
			c.config.ConnectProperties.ReceiveMaximum = 65535
		}

		if connack.SessionPresent && c.session != nil {
			// Restoring session and Resend all inflight messages with DUP flag
			inflightMessages := c.session.getInflightMessages()
			c.config.Logger.Info("resuming existing session",
				"inflight_count", len(inflightMessages))

			if len(inflightMessages) > 0 {
				for _, msg := range inflightMessages {
					switch p := msg.(type) {
					case *protocol.Publish:
						p.MarkAsDuplicated()
						c.config.Logger.Debug("resending inflight message",
							"packetID", p.GetPacketID(),
							"topic", p.Topic())

						if err := c.sendPacket(ctx, p); err != nil {
							c.config.Logger.Error("failed to resend inflight message",
								"packetID", p.GetPacketID(),
								"err", err)
						}
					case *protocol.PubRel:
						c.config.Logger.Debug("resending pubrel message",
							"packetID", p.GetPacketID())

						if err := c.sendPacket(ctx, p); err != nil {
							c.config.Logger.Error("failed to resend pubrel message",
								"packetID", p.GetPacketID(),
								"err", err)
						}
					}
				}
			}
		} else {
			if c.session != nil {
				c.config.Logger.Info("discarding previous session due to SessionPresent=false")
				c.session.cleanup()
			}
			c.session = newSession(c.config.ConnectProperties.ReceiveMaximum, c.config.Logger)
		}

		c.heartBeat.Start(ctx, c.config.KeepAlive)

		p := &protocol.PingReq{}
		if p, err := p.Encode(); err == nil {
			p.WriteTo(c.conn)
			c.heartBeat.PacketSent()
		}
		return nil
	case <-ctx.Done():
		c.close()
		return ctx.Err()
	}
}

func (c *Client) Publish(ctx context.Context, p Publish) (*PublishResponse, error) {
	packet, err := protocol.NewPublish(protocol.PublishOptions{
		Qos:     byte(p.Qos),
		Topic:   p.Topic,
		Payload: p.Payload,
		Retain:  p.Retain,
	})
	if err != nil {
		return nil, err
	}

	if p.Qos == QoSAtMostOnce {
		err = c.sendPacket(ctx, packet)
		return nil, err
	}

	packetID, respChan, err := c.session.storePublish(packet)
	if err != nil {
		return nil, err
	}
	c.config.Logger.Info("Publish stored", "packetID", packetID)

	if err = c.sendPacket(ctx, packet); err != nil {
		c.session.remove(packetID)
		return nil, err
	}

	select {
	case resp := <-respChan:
		pr := &PublishResponse{
			ReasonCode: resp.ReasonCode,
		}
		if resp.Properties != nil {
			pr.ReasonString = resp.Properties.ReasonString
		}
		return pr, nil
	case <-ctx.Done():
		if c.session != nil {
			c.session.remove(packetID)
		}
		return nil, fmt.Errorf("publish cancelled for packetID %d", packetID)
	}
}

func (c *Client) Subscribe(ctx context.Context, subscriptions ...Subscription) (*SubscribeResponse, error) {
	if len(subscriptions) == 0 {
		return nil, fmt.Errorf("at least one subscription is required")
	}

	subs := make([]protocol.Subscription, len(subscriptions))
	for i, sub := range subscriptions {
		subs[i] = protocol.Subscription{
			TopicFilter:       sub.TopicFilter,
			QoS:               byte(sub.QoS),
			NoLocal:           sub.NoLocal,
			RetainAsPublished: sub.RetainAsPublished,
			RetainHandling:    sub.RetainHandling,
		}
	}

	packet := &protocol.Subscribe{
		Subscriptions: subs,
	}

	packetID, respChan, err := c.session.storeSubscribe(packet)
	if err != nil {
		return nil, err
	}
	c.config.Logger.Debug("subscribe stored", "packetID", packetID)

	if err = c.sendPacket(ctx, packet); err != nil {
		c.session.removeSubscribe(packetID)
		return nil, err
	}

	select {
	case resp := <-respChan:
		reasonCodes := make(map[string]byte, len(subscriptions))
		for i, sub := range subscriptions {
			if i < len(resp.ReasonCodes) {
				reasonCodes[sub.TopicFilter] = resp.ReasonCodes[i]
			}
		}
		sr := &SubscribeResponse{
			ReasonCodes: reasonCodes,
		}
		if resp.Properties != nil {
			sr.ReasonString = resp.Properties.ReasonString
		}
		return sr, nil
	case <-ctx.Done():
		c.session.removeSubscribe(packetID)
		return nil, fmt.Errorf("subscribe cancelled for packetID %d", packetID)
	}
}

func (c *Client) sendPacket(ctx context.Context, p protocol.Packet) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("connection is nil")
	}

	if deadline, ok := ctx.Deadline(); ok {
		if err := c.conn.SetWriteDeadline(deadline); err != nil {
			return fmt.Errorf("failed to set write deadline: %w", err)
		}
	} else if c.config.Timeout > 0 {
		if err := c.conn.SetWriteDeadline(time.Now().Add(c.config.Timeout)); err != nil {
			return fmt.Errorf("failed to set write deadline: %w", err)
		}
	}
	defer c.conn.SetWriteDeadline(time.Time{})

	data, err := p.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode packet: %w", err)
	}

	_, err = data.WriteTo(c.conn)
	if err != nil {
		return err
	}
	c.heartBeat.PacketSent()

	return nil
}

func (c *Client) readLoop(ctx context.Context) {
	defer c.wg.Done()
	var packetType [1]byte

	for {
		select {
		case <-ctx.Done():
			return
		default:
			if c.conn == nil {
				return
			}
			_, err := c.conn.Read(packetType[:])
			if err != nil {
				if errors.Is(err, io.EOF) {
					c.config.Logger.Info("EOF received from server")
				} else {
					c.config.Logger.Error("failed to read from connection", "err", err)
				}

				if c.config.OnConnectionLost != nil {
					c.config.OnConnectionLost(err)
				}

				c.safeClose()
				return
			}
			c.heartBeat.PacketReceived()
			switch packetType[0] >> 4 {
			case protocol.TypeConnack:
				connack := &protocol.ConnAck{}
				err := connack.Decode(c.conn)
				if err != nil {
					c.config.Logger.Error("failed to decode connack packet", "err", err)
					if c.config.OnConnectionLost != nil {
						c.config.OnConnectionLost(err)
					}
					c.safeClose()
					return
				}
				c.connectSignal <- connack
			case protocol.TypePingResp:
				pingResp := &protocol.PingRes{}
				err := pingResp.Decode(c.conn)
				if err != nil {
					c.config.Logger.Error("failed to decode pingRes packet")
					c.safeClose()
					return
				}
				c.config.Logger.Info("ping response received..")
			case protocol.TypeDisconnect:
				disconnect := &protocol.Disconnect{}
				err := disconnect.Decode(c.conn)
				if err != nil {
					c.config.Logger.Error("failed to decode disconnect packet", "err", err)
					c.safeClose()
					return
				}
				c.config.Logger.Info(
					"disconnect received from server",
					"reason_code",
					disconnect.ReasonCode,
				)
				if c.config.OnConnectionLost != nil {
					c.config.OnConnectionLost(
						fmt.Errorf("server disconnected: reason code %d", disconnect.ReasonCode),
					)
				}
				c.safeClose()
				return
			case protocol.TypePubAck:
				pubAck := &protocol.AckPacket{}
				err := pubAck.Decode(c.conn)
				if err != nil {
					c.config.Logger.Error("failed to decode pubAck packet", "err", err)
					return
				}
				c.config.Logger.Info("PubAck received", "packetID", pubAck.PacketID)
				c.session.ack(pubAck.PacketID, pubAck)
			case protocol.TypePubRec:
				pubRec := &protocol.PubRec{}
				err := pubRec.Decode(c.conn)
				if err != nil {
					c.config.Logger.Error("failed to decode pubRec packet", "err", err)
					return
				}
				c.config.Logger.Debug("PubRec received", "packetID", pubRec.PacketID)

				relReasonCode := protocol.PubQosStepSuccess
				pubRel, err := protocol.NewPubRel(pubRec.PacketID, byte(relReasonCode), nil)
				if err != nil {
					c.config.Logger.Error("failed to create pubrel", "err", err)
					return
				}

				if found := c.session.rec(pubRec.PacketID, pubRel); !found {
					pubRel.ReasonCode = protocol.PubQosStepPackedIDNotFound
				}
				c.sendPacket(ctx, pubRel)
			case protocol.TypePubComp:
				pubComp := &protocol.AckPacket{}
				err := pubComp.Decode(c.conn)
				if err != nil {
					c.config.Logger.Error("failed to decode pubComp packet", "err", err)
					return
				}
				c.config.Logger.Debug("PubComp received", "packetID", pubComp.PacketID)
				c.session.ack(pubComp.PacketID, pubComp)
			case protocol.TypeSubAck:
				subAck := &protocol.SubAck{}
				err := subAck.Decode(c.conn)
				if err != nil {
					c.config.Logger.Error("failed to decode subAck packet", "err", err)
					return
				}
				c.config.Logger.Debug("SubAck received", "packetID", subAck.PacketID, "reasonCodes", subAck.ReasonCodes)
				c.session.ackSubscribe(subAck.PacketID, subAck)
			default:
				// TODO: Handle other packet types
				c.config.Logger.Warn("unsupported packet type", "type", packetType[0]>>4)
			}
		}
	}
}

// close does not lock c.mu because all callers (Connect, safeClose) already hold the lock
func (c *Client) close() {
	if c.cancelFunc != nil {
		c.cancelFunc()
		c.cancelFunc = nil
	}
	c.heartBeat.Stop()

	if c.session != nil {
		if c.config.ConnectProperties == nil ||
			c.config.ConnectProperties.SessionExpiryInterval == 0 {
			c.session.cleanup()
			c.session = nil
		}
	}

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			c.config.Logger.Error("failed to close connection", "err", err)
		}
	}

	c.conn = nil
	c.connected = false
}

func (c *Client) safeClose() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.close()
}

func (c *Client) Disconnect(ctx context.Context) error {
	c.config.Logger.Info("disconnecting")
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return nil
	}
	disconnect := protocol.NewDisconnect(protocol.DisconnectNormalDisconnection, nil)
	packet, err := disconnect.Encode()
	if err != nil {
		c.config.Logger.Error("failed to encode disconnect packet", "err", err)
	} else {
		if _, err = packet.WriteTo(c.conn); err != nil {
			c.config.Logger.Error("failed to write disconnect packet", "err", err)
		}
	}

	c.mu.Unlock()

	c.wg.Wait()

	c.safeClose()

	return nil
}

func convertConnectProperties(cp *ConnectProperties) *protocol.ConnectProperties {
	if cp == nil {
		return nil
	}

	var userProps []protocol.UserProperty
	for _, up := range cp.UserProperties {
		userProps = append(userProps, protocol.UserProperty{
			Key:   up.Key,
			Value: up.Value,
		})
	}

	result := &protocol.ConnectProperties{
		UserProperty:         userProps,
		AuthenticationMethod: cp.AuthenticationMethod,
		AuthenticationData:   cp.AuthenticationData,
	}

	if cp.SessionExpiryInterval > 0 {
		result.SessionExpiryInterval = &cp.SessionExpiryInterval
	}
	if cp.ReceiveMaximum > 0 {
		result.ReceiveMaximum = &cp.ReceiveMaximum
	}
	if cp.MaximumPacketSize > 0 {
		result.MaximumPacketSize = &cp.MaximumPacketSize
	}
	if cp.TopicAliasMaximum > 0 {
		result.TopicAliasMaximum = &cp.TopicAliasMaximum
	}
	if cp.RequestResponseInformation != 0 {
		result.RequestResponseInformation = &cp.RequestResponseInformation
	}
	if cp.RequestProblemInformation != 0 {
		result.RequestProblemInformation = &cp.RequestProblemInformation
	}

	return result
}

func convertWillProperties(wp *WillProperties) *protocol.WillProperties {
	if wp == nil {
		return nil
	}

	var userProps []protocol.UserProperty
	for _, up := range wp.UserProperties {
		userProps = append(userProps, protocol.UserProperty{
			Key:   up.Key,
			Value: up.Value,
		})
	}

	result := &protocol.WillProperties{
		ContentType:     wp.ContentType,
		ResponseTopic:   wp.ResponseTopic,
		CorrelationData: wp.CorrelationData,
		UserProperty:    userProps,
	}

	if wp.WillDelayInterval > 0 {
		result.WillDelayInterval = &wp.WillDelayInterval
	}
	if wp.PayloadFormatIndicator {
		result.PayloadFormatIndicator = &wp.PayloadFormatIndicator
	}
	if wp.MessageExpiryInterval > 0 {
		result.MessageExpiryInterval = &wp.MessageExpiryInterval
	}

	return result
}

func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

func (c *Client) dial(ctx context.Context) (net.Conn, error) {
	netw := c.config.Network
	if netw == "" {
		netw = "tcp"
	}

	if c.config.DialContext != nil {
		return c.config.DialContext(ctx, netw, c.config.Broker)
	}

	d := c.config.Dialer
	if d == nil {
		d = &net.Dialer{Timeout: c.config.Timeout, KeepAlive: 30 * time.Second}
	}

	if c.config.TLSConfig == nil {
		return d.DialContext(ctx, netw, c.config.Broker)
	}

	cfg := c.config.TLSConfig.Clone()
	if cfg.ServerName == "" {
		host, _, _ := net.SplitHostPort(c.config.Broker)
		if host == "" {
			host = c.config.Broker
		}
		cfg.ServerName = host
	}

	return tls.DialWithDialer(d, netw, c.config.Broker, cfg)
}

func (c *ClientConfig) Validate() error {
	if c.Broker == "" {
		return fmt.Errorf("broker address is required")
	}

	if c.KeepAlive < 0 || c.KeepAlive > 65535*time.Second {
		return fmt.Errorf("keep alive must be between 0 and 65535 seconds")
	}

	if c.Will != nil {
		if c.Will.Topic == "" {
			return fmt.Errorf("will topic cannot be empty")
		}
		if c.Will.QoS > 2 {
			return fmt.Errorf("invalid will QoS level")
		}
	}

	return nil
}

func (c *ClientConfig) setDefaults() {
	if c.ClientID == "" {
		c.ClientID = fmt.Sprintf("gmqtt-%d", time.Now().UnixNano())
	}

	if c.Timeout == 0 {
		c.Timeout = 30 * time.Second
	}
	if c.KeepAlive == 0 {
		c.KeepAlive = 30 * time.Second
	}
	if c.Logger == nil {
		c.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
}

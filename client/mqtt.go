package client

import (
	"bytes"
	"compress/zlib"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// MQTToT Constants
const (
	MQTTBrokerHost = "edge-mqtt.facebook.com"
	MQTTBrokerPort = 443

	// MQTT Control Packet Types
	MQTT_CONNECT    = 1
	MQTT_CONNACK    = 2
	MQTT_PUBLISH    = 3
	MQTT_PUBACK     = 4
	MQTT_SUBSCRIBE  = 8
	MQTT_SUBACK     = 9
	MQTT_PINGREQ    = 12
	MQTT_PINGRESP   = 13
	MQTT_DISCONNECT = 14

	// Instagram specific topics
	TopicMessageSync     = "/ig_message_sync"
	TopicRealtimeSubject = "/ig_realtime_sub"
	TopicSendMessage     = "/ig_send_message"
	TopicSendMessageResp = "/ig_send_message_response"

	// App IDs (IGAppID is in client.go)
	IGWebAppID = "936619743392459"

	// Capabilities
	CapabilitiesFlags = 0x3F // QoS 0, 1, 2 support
)

// MQTTClient handles Instagram's MQTToT real-time connection
type MQTTClient struct {
	mu sync.RWMutex

	client    *Client
	conn      net.Conn
	connected bool
	debug     bool

	// Packet tracking
	packetID uint16

	// Message handlers
	messageHandler func(topic string, payload []byte)

	// Keep alive
	keepAlive  time.Duration
	lastPing   time.Time
	pingTicker *time.Ticker
	stopChan   chan struct{}

	// Response channels for synchronous operations
	connackChan chan *ConnackPacket
	pubackChan  chan uint16
	subackChan  chan *SubackPacket
}

// ConnackPacket represents MQTT CONNACK response
type ConnackPacket struct {
	SessionPresent bool
	ReturnCode     byte
	Payload        []byte
}

// SubackPacket represents MQTT SUBACK response
type SubackPacket struct {
	PacketID    uint16
	ReturnCodes []byte
}

// MQTTMessage represents a message received via MQTT
type MQTTMessage struct {
	Topic     string
	Payload   []byte
	QoS       byte
	PacketID  uint16
	Retained  bool
	Duplicate bool
}

// ThriftField represents a field in thrift binary protocol
type ThriftField struct {
	Type  byte
	ID    int16
	Value interface{}
}

// NewMQTTClient creates a new MQTT client for Instagram
func NewMQTTClient(client *Client, debug bool) *MQTTClient {
	return &MQTTClient{
		client:      client,
		debug:       debug,
		keepAlive:   60 * time.Second,
		stopChan:    make(chan struct{}),
		connackChan: make(chan *ConnackPacket, 1),
		pubackChan:  make(chan uint16, 10),
		subackChan:  make(chan *SubackPacket, 1),
	}
}

// Connect establishes connection to Instagram's MQTT broker
func (m *MQTTClient) Connect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.connected {
		return nil
	}

	// Create TLS connection
	tlsConfig := &tls.Config{
		ServerName: MQTTBrokerHost,
		MinVersion: tls.VersionTLS12,
	}

	addr := fmt.Sprintf("%s:%d", MQTTBrokerHost, MQTTBrokerPort)

	if m.debug {
		fmt.Printf("[MQTT] Connecting to %s\n", addr)
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %w", err)
	}

	m.conn = conn

	// Start reading packets
	go m.readLoop()

	// Send CONNECT packet
	if err := m.sendConnect(); err != nil {
		m.conn.Close()
		return fmt.Errorf("failed to send CONNECT: %w", err)
	}

	// Wait for CONNACK
	select {
	case connack := <-m.connackChan:
		if connack.ReturnCode != 0 {
			m.conn.Close()
			return fmt.Errorf("connection refused, code: %d", connack.ReturnCode)
		}
		if m.debug {
			fmt.Printf("[MQTT] Connected successfully, session present: %v\n", connack.SessionPresent)
		}
	case <-time.After(10 * time.Second):
		m.conn.Close()
		return fmt.Errorf("CONNACK timeout")
	}

	m.connected = true

	// Start keep alive
	m.startKeepAlive()

	return nil
}

// sendConnect sends the MQTToT CONNECT packet
func (m *MQTTClient) sendConnect() error {
	// Build the connect payload (Thrift binary format, zlib compressed)
	payload := m.buildConnectPayload()

	// Compress with zlib
	var compressed bytes.Buffer
	zlibWriter := zlib.NewWriter(&compressed)
	zlibWriter.Write(payload)
	zlibWriter.Close()

	compressedPayload := compressed.Bytes()

	// Build MQTT CONNECT packet with custom payload
	packet := m.buildConnectPacket(compressedPayload)

	if m.debug {
		fmt.Printf("[MQTT] Sending CONNECT packet (%d bytes)\n", len(packet))
	}

	_, err := m.conn.Write(packet)
	return err
}

// buildConnectPayload creates the Thrift-encoded connect payload
func (m *MQTTClient) buildConnectPayload() []byte {
	var buf bytes.Buffer

	// Simplified connect info - Instagram uses Thrift binary protocol
	// This is a minimal implementation

	userID := m.client.UserID()
	sessionID := m.client.GetSessionID()

	connectInfo := map[string]interface{}{
		"u":          userID,             // User ID
		"s":          sessionID,          // Session ID
		"cp":         CapabilitiesFlags,  // Capabilities
		"ecp":        0,                  // Extended capabilities
		"chat_on":    true,               // Enable chat
		"fg":         false,              // Foreground
		"d":          m.client.UUID,      // Device ID
		"ct":         "cookie_auth",      // Connection type
		"mqtt_sid":   "",                 // MQTT session ID
		"aid":        IGAppID,            // App ID
		"st":         []string{},         // Subscription topics
		"pm":         []string{},         // Presence messages
		"dc":         "",                 // Data center
		"no_auto_fg": true,               // No auto foreground
		"a":          m.client.UserAgent, // User agent
	}

	// Encode as JSON for now (simplified - real implementation uses Thrift)
	jsonData, _ := json.Marshal(connectInfo)
	buf.Write(jsonData)

	return buf.Bytes()
}

// buildConnectPacket builds the MQTT CONNECT packet
func (m *MQTTClient) buildConnectPacket(payload []byte) []byte {
	var packet bytes.Buffer

	// Variable header
	var varHeader bytes.Buffer

	// Protocol name "MQTToT"
	protocolName := []byte("MQTToT")
	binary.Write(&varHeader, binary.BigEndian, uint16(len(protocolName)))
	varHeader.Write(protocolName)

	// Protocol level (3 for MQTT 3.1.1 compatible)
	varHeader.WriteByte(3)

	// Connect flags
	// Bit 7: Username flag (1)
	// Bit 6: Password flag (1)
	// Bit 5: Will retain (0)
	// Bit 4-3: Will QoS (00)
	// Bit 2: Will flag (0)
	// Bit 1: Clean session (1)
	// Bit 0: Reserved (0)
	connectFlags := byte(0xC2) // Username + Password + Clean Session
	varHeader.WriteByte(connectFlags)

	// Keep alive (60 seconds)
	binary.Write(&varHeader, binary.BigEndian, uint16(60))

	// Payload
	var payloadBuf bytes.Buffer

	// Client ID (compressed payload goes here for MQTToT)
	binary.Write(&payloadBuf, binary.BigEndian, uint16(len(payload)))
	payloadBuf.Write(payload)

	// Username (empty for MQTToT - auth is in payload)
	binary.Write(&payloadBuf, binary.BigEndian, uint16(0))

	// Password (empty for MQTToT)
	binary.Write(&payloadBuf, binary.BigEndian, uint16(0))

	// Calculate remaining length
	remainingLength := varHeader.Len() + payloadBuf.Len()

	// Fixed header
	packet.WriteByte(MQTT_CONNECT << 4) // CONNECT packet type
	m.writeRemainingLength(&packet, remainingLength)

	// Write variable header and payload
	packet.Write(varHeader.Bytes())
	packet.Write(payloadBuf.Bytes())

	return packet.Bytes()
}

// writeRemainingLength writes the MQTT remaining length field
func (m *MQTTClient) writeRemainingLength(buf *bytes.Buffer, length int) {
	for {
		encodedByte := byte(length % 128)
		length = length / 128
		if length > 0 {
			encodedByte |= 0x80
		}
		buf.WriteByte(encodedByte)
		if length == 0 {
			break
		}
	}
}

// readLoop continuously reads packets from the connection
func (m *MQTTClient) readLoop() {
	for {
		select {
		case <-m.stopChan:
			return
		default:
			if err := m.readPacket(); err != nil {
				if m.debug {
					fmt.Printf("[MQTT] Read error: %v\n", err)
				}
				m.handleDisconnect()
				return
			}
		}
	}
}

// readPacket reads and processes a single MQTT packet
func (m *MQTTClient) readPacket() error {
	// Set read deadline
	m.conn.SetReadDeadline(time.Now().Add(m.keepAlive * 2))

	// Read fixed header
	fixedHeader := make([]byte, 1)
	if _, err := io.ReadFull(m.conn, fixedHeader); err != nil {
		return err
	}

	packetType := fixedHeader[0] >> 4
	flags := fixedHeader[0] & 0x0F

	// Read remaining length
	remainingLength, err := m.readRemainingLength()
	if err != nil {
		return err
	}

	// Read payload
	payload := make([]byte, remainingLength)
	if remainingLength > 0 {
		if _, err := io.ReadFull(m.conn, payload); err != nil {
			return err
		}
	}

	// Process packet
	return m.processPacket(packetType, flags, payload)
}

// readRemainingLength reads the MQTT remaining length field
func (m *MQTTClient) readRemainingLength() (int, error) {
	multiplier := 1
	value := 0

	for {
		b := make([]byte, 1)
		if _, err := io.ReadFull(m.conn, b); err != nil {
			return 0, err
		}

		value += int(b[0]&127) * multiplier
		multiplier *= 128

		if multiplier > 128*128*128 {
			return 0, fmt.Errorf("malformed remaining length")
		}

		if b[0]&128 == 0 {
			break
		}
	}

	return value, nil
}

// processPacket handles different MQTT packet types
func (m *MQTTClient) processPacket(packetType byte, flags byte, payload []byte) error {
	switch packetType {
	case MQTT_CONNACK:
		return m.handleConnack(payload)
	case MQTT_PUBLISH:
		return m.handlePublish(flags, payload)
	case MQTT_PUBACK:
		return m.handlePuback(payload)
	case MQTT_SUBACK:
		return m.handleSuback(payload)
	case MQTT_PINGRESP:
		if m.debug {
			fmt.Println("[MQTT] PINGRESP received")
		}
		return nil
	default:
		if m.debug {
			fmt.Printf("[MQTT] Unknown packet type: %d\n", packetType)
		}
		return nil
	}
}

// handleConnack processes CONNACK packet
func (m *MQTTClient) handleConnack(payload []byte) error {
	if len(payload) < 2 {
		return fmt.Errorf("CONNACK payload too short")
	}

	connack := &ConnackPacket{
		SessionPresent: payload[0]&0x01 == 1,
		ReturnCode:     payload[1],
	}

	// MQTToT CONNACK can have additional payload
	if len(payload) > 2 {
		connack.Payload = payload[2:]
	}

	if m.debug {
		fmt.Printf("[MQTT] CONNACK: session_present=%v, code=%d\n",
			connack.SessionPresent, connack.ReturnCode)
	}

	select {
	case m.connackChan <- connack:
	default:
	}

	return nil
}

// handlePublish processes incoming PUBLISH packets
func (m *MQTTClient) handlePublish(flags byte, payload []byte) error {
	if len(payload) < 2 {
		return fmt.Errorf("PUBLISH payload too short")
	}

	qos := (flags >> 1) & 0x03
	retain := flags&0x01 == 1
	dup := (flags>>3)&0x01 == 1

	// Read topic length
	topicLen := binary.BigEndian.Uint16(payload[:2])
	if len(payload) < int(2+topicLen) {
		return fmt.Errorf("PUBLISH topic too short")
	}

	topic := string(payload[2 : 2+topicLen])
	offset := 2 + int(topicLen)

	var packetID uint16
	if qos > 0 {
		if len(payload) < offset+2 {
			return fmt.Errorf("PUBLISH packet ID missing")
		}
		packetID = binary.BigEndian.Uint16(payload[offset : offset+2])
		offset += 2
	}

	messagePayload := payload[offset:]

	// Decompress if needed (Instagram often sends zlib compressed data)
	decompressed, err := m.tryDecompress(messagePayload)
	if err == nil {
		messagePayload = decompressed
	}

	if m.debug {
		fmt.Printf("[MQTT] PUBLISH: topic=%s, qos=%d, retain=%v, len=%d\n",
			topic, qos, retain, len(messagePayload))
	}

	// Send PUBACK for QoS 1
	if qos == 1 {
		m.sendPuback(packetID)
	}

	// Invoke message handler
	if m.messageHandler != nil {
		go m.messageHandler(topic, messagePayload)
	}

	_ = dup // Suppress unused warning
	return nil
}

// handlePuback processes PUBACK packet
func (m *MQTTClient) handlePuback(payload []byte) error {
	if len(payload) < 2 {
		return fmt.Errorf("PUBACK payload too short")
	}

	packetID := binary.BigEndian.Uint16(payload[:2])

	if m.debug {
		fmt.Printf("[MQTT] PUBACK: packet_id=%d\n", packetID)
	}

	select {
	case m.pubackChan <- packetID:
	default:
	}

	return nil
}

// handleSuback processes SUBACK packet
func (m *MQTTClient) handleSuback(payload []byte) error {
	if len(payload) < 3 {
		return fmt.Errorf("SUBACK payload too short")
	}

	suback := &SubackPacket{
		PacketID:    binary.BigEndian.Uint16(payload[:2]),
		ReturnCodes: payload[2:],
	}

	if m.debug {
		fmt.Printf("[MQTT] SUBACK: packet_id=%d, codes=%v\n",
			suback.PacketID, suback.ReturnCodes)
	}

	select {
	case m.subackChan <- suback:
	default:
	}

	return nil
}

// tryDecompress attempts to decompress zlib data
func (m *MQTTClient) tryDecompress(data []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

// Subscribe subscribes to MQTT topics
func (m *MQTTClient) Subscribe(topics []string) error {
	m.mu.Lock()
	packetID := m.nextPacketID()
	m.mu.Unlock()

	var packet bytes.Buffer

	// Variable header - packet ID
	var varHeader bytes.Buffer
	binary.Write(&varHeader, binary.BigEndian, packetID)

	// Payload - topics with QoS
	var payload bytes.Buffer
	for _, topic := range topics {
		binary.Write(&payload, binary.BigEndian, uint16(len(topic)))
		payload.WriteString(topic)
		payload.WriteByte(1) // QoS 1
	}

	remainingLength := varHeader.Len() + payload.Len()

	// Fixed header
	packet.WriteByte(MQTT_SUBSCRIBE<<4 | 0x02) // SUBSCRIBE with QoS 1
	m.writeRemainingLength(&packet, remainingLength)
	packet.Write(varHeader.Bytes())
	packet.Write(payload.Bytes())

	if m.debug {
		fmt.Printf("[MQTT] SUBSCRIBE: topics=%v\n", topics)
	}

	m.mu.Lock()
	_, err := m.conn.Write(packet.Bytes())
	m.mu.Unlock()

	if err != nil {
		return err
	}

	// Wait for SUBACK
	select {
	case suback := <-m.subackChan:
		if suback.PacketID != packetID {
			return fmt.Errorf("SUBACK packet ID mismatch")
		}
		for _, code := range suback.ReturnCodes {
			if code == 0x80 {
				return fmt.Errorf("subscription failed")
			}
		}
	case <-time.After(10 * time.Second):
		return fmt.Errorf("SUBACK timeout")
	}

	return nil
}

// Publish publishes a message to a topic
func (m *MQTTClient) Publish(topic string, payload []byte, qos byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.connected {
		return fmt.Errorf("not connected")
	}

	var packet bytes.Buffer

	// Variable header
	var varHeader bytes.Buffer
	binary.Write(&varHeader, binary.BigEndian, uint16(len(topic)))
	varHeader.WriteString(topic)

	var packetID uint16
	if qos > 0 {
		packetID = m.nextPacketID()
		binary.Write(&varHeader, binary.BigEndian, packetID)
	}

	// Compress payload with zlib
	var compressed bytes.Buffer
	zlibWriter := zlib.NewWriter(&compressed)
	zlibWriter.Write(payload)
	zlibWriter.Close()

	compressedPayload := compressed.Bytes()

	remainingLength := varHeader.Len() + len(compressedPayload)

	// Fixed header
	flags := byte(qos << 1)
	packet.WriteByte(MQTT_PUBLISH<<4 | flags)
	m.writeRemainingLength(&packet, remainingLength)
	packet.Write(varHeader.Bytes())
	packet.Write(compressedPayload)

	if m.debug {
		fmt.Printf("[MQTT] PUBLISH: topic=%s, qos=%d, len=%d\n",
			topic, qos, len(payload))
	}

	_, err := m.conn.Write(packet.Bytes())
	if err != nil {
		return err
	}

	// Wait for PUBACK if QoS 1
	if qos == 1 {
		select {
		case ackID := <-m.pubackChan:
			if ackID != packetID {
				return fmt.Errorf("PUBACK packet ID mismatch")
			}
		case <-time.After(10 * time.Second):
			return fmt.Errorf("PUBACK timeout")
		}
	}

	return nil
}

// SendDirectMessage sends a direct message via MQTT
func (m *MQTTClient) SendDirectMessage(threadID string, text string) error {
	// Build the message payload
	messagePayload := map[string]interface{}{
		"thread_id":      threadID,
		"text":           text,
		"client_context": m.client.generateUUID(),
		"action":         "send_item",
		"item_type":      "text",
	}

	payload, err := json.Marshal(messagePayload)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return m.Publish(TopicSendMessage, payload, 1)
}

// SetMessageHandler sets the callback for incoming messages
func (m *MQTTClient) SetMessageHandler(handler func(topic string, payload []byte)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messageHandler = handler
}

// sendPuback sends a PUBACK packet
func (m *MQTTClient) sendPuback(packetID uint16) error {
	packet := make([]byte, 4)
	packet[0] = MQTT_PUBACK << 4
	packet[1] = 2
	binary.BigEndian.PutUint16(packet[2:], packetID)

	m.mu.Lock()
	_, err := m.conn.Write(packet)
	m.mu.Unlock()

	return err
}

// sendPingreq sends a PINGREQ packet
func (m *MQTTClient) sendPingreq() error {
	packet := []byte{MQTT_PINGREQ << 4, 0}

	m.mu.Lock()
	_, err := m.conn.Write(packet)
	m.mu.Unlock()

	if m.debug {
		fmt.Println("[MQTT] PINGREQ sent")
	}

	return err
}

// startKeepAlive starts the keep alive ping loop
func (m *MQTTClient) startKeepAlive() {
	m.pingTicker = time.NewTicker(m.keepAlive / 2)

	go func() {
		for {
			select {
			case <-m.stopChan:
				m.pingTicker.Stop()
				return
			case <-m.pingTicker.C:
				if err := m.sendPingreq(); err != nil {
					if m.debug {
						fmt.Printf("[MQTT] Ping error: %v\n", err)
					}
					m.handleDisconnect()
					return
				}
				m.lastPing = time.Now()
			}
		}
	}()
}

// handleDisconnect handles connection loss
func (m *MQTTClient) handleDisconnect() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.connected {
		return
	}

	m.connected = false
	if m.conn != nil {
		m.conn.Close()
	}

	if m.debug {
		fmt.Println("[MQTT] Disconnected")
	}
}

// Disconnect closes the MQTT connection
func (m *MQTTClient) Disconnect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.connected {
		return nil
	}

	// Send DISCONNECT packet
	packet := []byte{MQTT_DISCONNECT << 4, 0}
	m.conn.Write(packet)

	// Stop background goroutines
	close(m.stopChan)

	// Close connection
	m.connected = false
	return m.conn.Close()
}

// IsConnected returns the connection status
func (m *MQTTClient) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected
}

// nextPacketID generates the next packet ID
func (m *MQTTClient) nextPacketID() uint16 {
	m.packetID++
	if m.packetID == 0 {
		m.packetID = 1
	}
	return m.packetID
}

// SubscribeToDirectMessages subscribes to DM-related topics
func (m *MQTTClient) SubscribeToDirectMessages() error {
	topics := []string{
		TopicMessageSync,
		TopicRealtimeSubject,
		TopicSendMessageResp,
		"/pubsub",
		"/t_ms",          // Message sync
		"/thread_typing", // Typing indicators
		"/orca_presence", // Presence
	}

	return m.Subscribe(topics)
}

package client

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"myDvpn/clientPeer/proto"
	"myDvpn/utils"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// PersistentStreamManager manages the persistent control stream to SuperNode
type PersistentStreamManager struct {
	peerID       string
	role         string
	region       string
	supernodeAddr string
	keyPair      *utils.KeyPair
	logger       *logrus.Logger
	
	conn         *grpc.ClientConn
	client       proto.ControlStreamClient
	stream       proto.ControlStream_PersistentControlStreamClient
	sessionID    string
	
	// Command handling
	commandHandlers map[proto.CommandType]func(*proto.Command) *proto.CommandResponse
	
	// State
	isConnected     bool
	lastHeartbeat   time.Time
	reconnectDelay  time.Duration
}

// NewPersistentStreamManager creates a new persistent stream manager
func NewPersistentStreamManager(peerID, role, region, supernodeAddr string, logger *logrus.Logger) (*PersistentStreamManager, error) {
	keyPair, err := utils.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	psm := &PersistentStreamManager{
		peerID:          peerID,
		role:            role,
		region:          region,
		supernodeAddr:   supernodeAddr,
		keyPair:         keyPair,
		logger:          logger,
		reconnectDelay:  5 * time.Second,
		commandHandlers: make(map[proto.CommandType]func(*proto.Command) *proto.CommandResponse),
	}

	// Register default command handlers
	psm.registerCommandHandlers()

	return psm, nil
}

// Start starts the persistent stream connection
func (psm *PersistentStreamManager) Start() error {
	if err := psm.connect(); err != nil {
		return fmt.Errorf("failed to establish initial connection: %w", err)
	}

	// Start background tasks
	go psm.messageHandler()
	go psm.heartbeatLoop()
	go psm.reconnectLoop()

	psm.logger.WithFields(logrus.Fields{
		"peer_id": psm.peerID,
		"role":    psm.role,
		"region":  psm.region,
	}).Info("Persistent stream manager started")

	return nil
}

// Stop stops the persistent stream connection
func (psm *PersistentStreamManager) Stop() {
	psm.isConnected = false
	
	if psm.stream != nil {
		psm.stream.CloseSend()
	}
	
	if psm.conn != nil {
		psm.conn.Close()
	}

	psm.logger.WithField("peer_id", psm.peerID).Info("Persistent stream manager stopped")
}

// connect establishes connection and authenticates
func (psm *PersistentStreamManager) connect() error {
	// Establish gRPC connection
	conn, err := grpc.Dial(psm.supernodeAddr, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("failed to connect to SuperNode: %w", err)
	}

	psm.conn = conn
	psm.client = proto.NewControlStreamClient(conn)

	// Open persistent stream
	stream, err := psm.client.PersistentControlStream(context.Background())
	if err != nil {
		return fmt.Errorf("failed to open persistent stream: %w", err)
	}

	psm.stream = stream

	// Authenticate
	if err := psm.authenticate(); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	psm.isConnected = true
	psm.lastHeartbeat = time.Now()

	return nil
}

// authenticate sends authentication request
func (psm *PersistentStreamManager) authenticate() error {
	// Generate nonce
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}
	nonceB64 := base64.StdEncoding.EncodeToString(nonce)

	// Create signature
	message := fmt.Sprintf("%s||%s||%s||%s", psm.peerID, psm.role, psm.region, nonceB64)
	signature := psm.keyPair.Sign([]byte(message))
	signatureB64 := utils.SignatureToBase64(signature)

	// Send auth request
	authReq := &proto.ControlMessage{
		MessageId: fmt.Sprintf("auth-%d", time.Now().UnixNano()),
		Timestamp: time.Now().Unix(),
		Payload: &proto.ControlMessage_AuthRequest{
			AuthRequest: &proto.AuthRequest{
				PeerId:     psm.peerID,
				Role:       psm.role,
				PubkeyB64:  utils.PublicKeyToBase64(psm.keyPair.PublicKey),
				Region:     psm.region,
				Signature:  signatureB64,
				Nonce:      nonceB64,
			},
		},
	}

	if err := psm.stream.Send(authReq); err != nil {
		return fmt.Errorf("failed to send auth request: %w", err)
	}

	// Wait for auth response
	msg, err := psm.stream.Recv()
	if err != nil {
		return fmt.Errorf("failed to receive auth response: %w", err)
	}

	authResp, ok := msg.Payload.(*proto.ControlMessage_AuthResponse)
	if !ok {
		return fmt.Errorf("unexpected message type for auth response")
	}

	if !authResp.AuthResponse.Success {
		return fmt.Errorf("authentication failed: %s", authResp.AuthResponse.Message)
	}

	psm.sessionID = authResp.AuthResponse.SessionId

	psm.logger.WithFields(logrus.Fields{
		"peer_id":    psm.peerID,
		"session_id": psm.sessionID,
	}).Info("Authentication successful")

	return nil
}

// messageHandler handles incoming messages
func (psm *PersistentStreamManager) messageHandler() {
	for psm.isConnected {
		if psm.stream == nil {
			time.Sleep(1 * time.Second)
			continue
		}

		msg, err := psm.stream.Recv()
		if err == io.EOF {
			psm.logger.Info("Stream closed by server")
			psm.isConnected = false
			break
		}
		if err != nil {
			psm.logger.WithError(err).Error("Error receiving message")
			psm.isConnected = false
			break
		}

		psm.handleMessage(msg)
	}
}

// handleMessage handles a received message
func (psm *PersistentStreamManager) handleMessage(msg *proto.ControlMessage) {
	switch payload := msg.Payload.(type) {
	case *proto.ControlMessage_PongResponse:
		psm.handlePongResponse(payload.PongResponse)
		
	case *proto.ControlMessage_Command:
		psm.handleCommand(payload.Command)
		
	case *proto.ControlMessage_InfoResponse:
		psm.handleInfoResponse(payload.InfoResponse)
		
	default:
		psm.logger.WithField("message_type", fmt.Sprintf("%T", payload)).Warn("Unknown message type received")
	}
}

// handlePongResponse handles pong responses
func (psm *PersistentStreamManager) handlePongResponse(pong *proto.PongResponse) {
	latency := time.Now().UnixMilli() - pong.OriginalTimestamp
	psm.lastHeartbeat = time.Now()

	psm.logger.WithFields(logrus.Fields{
		"peer_id":   psm.peerID,
		"latency_ms": latency,
	}).Debug("Received pong response")
}

// handleCommand handles commands from SuperNode
func (psm *PersistentStreamManager) handleCommand(cmd *proto.Command) {
	handler, exists := psm.commandHandlers[cmd.Type]
	if !exists {
		psm.logger.WithField("command_type", cmd.Type).Warn("No handler for command type")
		return
	}

	// Execute command
	response := handler(cmd)

	// Send response
	respMsg := &proto.ControlMessage{
		MessageId: fmt.Sprintf("cmd-resp-%d", time.Now().UnixNano()),
		Timestamp: time.Now().Unix(),
		Payload: &proto.ControlMessage_CommandResponse{
			CommandResponse: response,
		},
	}

	if err := psm.stream.Send(respMsg); err != nil {
		psm.logger.WithError(err).Error("Failed to send command response")
	}
}

// handleInfoResponse handles info responses
func (psm *PersistentStreamManager) handleInfoResponse(info *proto.InfoResponse) {
	psm.logger.WithFields(logrus.Fields{
		"peer_id": info.PeerId,
		"info":    info.Info,
	}).Info("Received info response")
}

// sendHeartbeat sends a ping request
func (psm *PersistentStreamManager) sendHeartbeat() error {
	if psm.stream == nil {
		return fmt.Errorf("stream not available")
	}

	ping := &proto.ControlMessage{
		MessageId: fmt.Sprintf("ping-%d", time.Now().UnixNano()),
		Timestamp: time.Now().Unix(),
		Payload: &proto.ControlMessage_PingRequest{
			PingRequest: &proto.PingRequest{
				Timestamp: time.Now().UnixMilli(),
				PeerId:    psm.peerID,
			},
		},
	}

	return psm.stream.Send(ping)
}

// heartbeatLoop sends periodic heartbeats
func (psm *PersistentStreamManager) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for psm.isConnected {
		select {
		case <-ticker.C:
			if err := psm.sendHeartbeat(); err != nil {
				psm.logger.WithError(err).Error("Failed to send heartbeat")
				psm.isConnected = false
			}
		}
	}
}

// reconnectLoop handles reconnection logic
func (psm *PersistentStreamManager) reconnectLoop() {
	for {
		if !psm.isConnected {
			psm.logger.Info("Attempting to reconnect...")
			
			if err := psm.connect(); err != nil {
				psm.logger.WithError(err).Error("Reconnection failed, retrying...")
				time.Sleep(psm.reconnectDelay)
				
				// Exponential backoff
				if psm.reconnectDelay < 60*time.Second {
					psm.reconnectDelay *= 2
				}
			} else {
				psm.logger.Info("Reconnection successful")
				psm.reconnectDelay = 5 * time.Second // Reset delay
				go psm.messageHandler()
				go psm.heartbeatLoop()
			}
		}
		
		time.Sleep(5 * time.Second)
	}
}

// registerCommandHandlers registers default command handlers
func (psm *PersistentStreamManager) registerCommandHandlers() {
	psm.commandHandlers[proto.CommandType_SETUP_EXIT] = psm.handleSetupExitCommand
	psm.commandHandlers[proto.CommandType_ROTATE_PEER] = psm.handleRotatePeerCommand
	psm.commandHandlers[proto.CommandType_RELAY_SETUP] = psm.handleRelaySetupCommand
	psm.commandHandlers[proto.CommandType_DISCONNECT] = psm.handleDisconnectCommand
}

// Command handlers
func (psm *PersistentStreamManager) handleSetupExitCommand(cmd *proto.Command) *proto.CommandResponse {
	psm.logger.WithField("command_id", cmd.CommandId).Info("Handling SETUP_EXIT command")
	
	// For client peer, this would typically be handled differently
	// This is a placeholder implementation
	return &proto.CommandResponse{
		CommandId: cmd.CommandId,
		Success:   true,
		Message:   "Setup exit command received",
		Result:    make(map[string]string),
	}
}

func (psm *PersistentStreamManager) handleRotatePeerCommand(cmd *proto.Command) *proto.CommandResponse {
	psm.logger.WithField("command_id", cmd.CommandId).Info("Handling ROTATE_PEER command")
	
	return &proto.CommandResponse{
		CommandId: cmd.CommandId,
		Success:   true,
		Message:   "Rotate peer command received",
		Result:    make(map[string]string),
	}
}

func (psm *PersistentStreamManager) handleRelaySetupCommand(cmd *proto.Command) *proto.CommandResponse {
	psm.logger.WithField("command_id", cmd.CommandId).Info("Handling RELAY_SETUP command")
	
	return &proto.CommandResponse{
		CommandId: cmd.CommandId,
		Success:   true,
		Message:   "Relay setup command received",
		Result:    make(map[string]string),
	}
}

func (psm *PersistentStreamManager) handleDisconnectCommand(cmd *proto.Command) *proto.CommandResponse {
	psm.logger.WithField("command_id", cmd.CommandId).Info("Handling DISCONNECT command")
	
	// Gracefully disconnect
	go func() {
		time.Sleep(1 * time.Second)
		psm.Stop()
	}()
	
	return &proto.CommandResponse{
		CommandId: cmd.CommandId,
		Success:   true,
		Message:   "Disconnect command received",
		Result:    make(map[string]string),
	}
}

// IsConnected returns connection status
func (psm *PersistentStreamManager) IsConnected() bool {
	return psm.isConnected
}

// RegisterCommandHandler registers a custom command handler
func (psm *PersistentStreamManager) RegisterCommandHandler(cmdType proto.CommandType, handler func(*proto.Command) *proto.CommandResponse) {
	psm.commandHandlers[cmdType] = handler
}

// GetSessionID returns the current session ID
func (psm *PersistentStreamManager) GetSessionID() string {
	return psm.sessionID
}
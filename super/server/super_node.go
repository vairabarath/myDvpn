package server

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"myDvpn/base/proto"
	controlProto "myDvpn/clientPeer/proto"
	"myDvpn/utils"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SuperNode represents a SuperNode server
type SuperNode struct {
	controlProto.UnimplementedControlStreamServer
	controlProto.UnimplementedSuperNodeServer

	id            string
	region        string
	listenAddr    string
	streamManager *StreamManager
	baseNodeAddr  string
	baseClient    proto.BaseNodeClient
	logger        *logrus.Logger
	server        *grpc.Server

	// WireGuard interface for relay
	relayInterface string
	relayPort     int
}

// NewSuperNode creates a new SuperNode
func NewSuperNode(id, region, listenAddr, baseNodeAddr string, logger *logrus.Logger) *SuperNode {
	return &SuperNode{
		id:             id,
		region:         region,
		listenAddr:     listenAddr,
		streamManager:  NewStreamManager(logger),
		baseNodeAddr:   baseNodeAddr,
		logger:         logger,
		relayInterface: fmt.Sprintf("wg-relay-%s", id),
		relayPort:     51820 + len(id)%1000, // Simple port allocation
	}
}

// Start starts the SuperNode server
func (sn *SuperNode) Start() error {
	// Connect to BaseNode
	conn, err := grpc.Dial(sn.baseNodeAddr, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("failed to connect to BaseNode: %w", err)
	}
	sn.baseClient = proto.NewBaseNodeClient(conn)

	// Register with BaseNode
	if err := sn.registerWithBaseNode(); err != nil {
		return fmt.Errorf("failed to register with BaseNode: %w", err)
	}

	// Start gRPC server
	listener, err := net.Listen("tcp", sn.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", sn.listenAddr, err)
	}

	sn.server = grpc.NewServer()
	controlProto.RegisterControlStreamServer(sn.server, sn)
	controlProto.RegisterSuperNodeServer(sn.server, sn)

	sn.logger.WithFields(logrus.Fields{
		"id":     sn.id,
		"region": sn.region,
		"addr":   sn.listenAddr,
	}).Info("Starting SuperNode server")

	// Start background tasks
	go sn.heartbeatLoop()
	go sn.staleStreamChecker()

	return sn.server.Serve(listener)
}

// Stop stops the SuperNode server
func (sn *SuperNode) Stop() {
	if sn.server != nil {
		sn.server.GracefulStop()
	}
}

// PersistentControlStream handles the persistent control stream
func (sn *SuperNode) PersistentControlStream(stream controlProto.ControlStream_PersistentControlStreamServer) error {
	var peerID string
	authenticated := false

	sn.logger.Info("New control stream connected")

	defer func() {
		if authenticated && peerID != "" {
			sn.streamManager.UnregisterStream(peerID)
		}
	}()

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			sn.logger.WithField("peer_id", peerID).Info("Control stream closed by client")
			return nil
		}
		if err != nil {
			sn.logger.WithFields(logrus.Fields{
				"peer_id": peerID,
				"error":   err,
			}).Error("Error receiving message")
			return err
		}

		switch payload := msg.Payload.(type) {
		case *controlProto.ControlMessage_AuthRequest:
			var err error
			peerID, _, err = sn.handleAuthRequest(payload.AuthRequest, stream)
			if err != nil {
				sn.logger.WithError(err).Error("Authentication failed")
				sn.streamManager.IncrementAuthFailures()
				return status.Errorf(codes.Unauthenticated, "authentication failed: %v", err)
			}
			authenticated = true

		case *controlProto.ControlMessage_PingRequest:
			if !authenticated {
				return status.Errorf(codes.Unauthenticated, "not authenticated")
			}
			if err := sn.handlePingRequest(payload.PingRequest, stream); err != nil {
				sn.logger.WithError(err).Error("Failed to handle ping")
			}

		case *controlProto.ControlMessage_CommandResponse:
			if !authenticated {
				return status.Errorf(codes.Unauthenticated, "not authenticated")
			}
			sn.handleCommandResponse(peerID, payload.CommandResponse)

		case *controlProto.ControlMessage_InfoRequest:
			if !authenticated {
				return status.Errorf(codes.Unauthenticated, "not authenticated")
			}
			if err := sn.handleInfoRequest(payload.InfoRequest, stream); err != nil {
				sn.logger.WithError(err).Error("Failed to handle info request")
			}

		default:
			sn.logger.WithField("peer_id", peerID).Warn("Unknown message type received")
		}
	}
}

// handleAuthRequest handles authentication requests
func (sn *SuperNode) handleAuthRequest(req *controlProto.AuthRequest, stream controlProto.ControlStream_PersistentControlStreamServer) (string, string, error) {
	// Validate role
	role := PeerRole(req.Role)
	if role != RoleClient && role != RoleExit && role != RoleHybrid {
		return "", "", fmt.Errorf("invalid role: %s", req.Role)
	}

	// Verify signature
	if err := sn.verifyAuthSignature(req); err != nil {
		return "", "", fmt.Errorf("signature verification failed: %w", err)
	}

	// Register stream
	sessionID, err := sn.streamManager.RegisterStream(req.PeerId, role, req.Region, req.PubkeyB64, stream)
	if err != nil {
		return "", "", fmt.Errorf("failed to register stream: %w", err)
	}

	// Send auth response
	response := &controlProto.ControlMessage{
		MessageId: fmt.Sprintf("auth-resp-%d", time.Now().UnixNano()),
		Timestamp: time.Now().Unix(),
		Payload: &controlProto.ControlMessage_AuthResponse{
			AuthResponse: &controlProto.AuthResponse{
				Success:   true,
				Message:   "Authentication successful",
				SessionId: sessionID,
			},
		},
	}

	if err := stream.Send(response); err != nil {
		return "", "", fmt.Errorf("failed to send auth response: %w", err)
	}

	sn.logger.WithFields(logrus.Fields{
		"peer_id":    req.PeerId,
		"role":       req.Role,
		"region":     req.Region,
		"session_id": sessionID,
	}).Info("Peer authenticated successfully")

	return req.PeerId, sessionID, nil
}

// verifyAuthSignature verifies the authentication signature
func (sn *SuperNode) verifyAuthSignature(req *controlProto.AuthRequest) error {
	// Reconstruct the signed message
	message := fmt.Sprintf("%s||%s||%s||%s", req.PeerId, req.Role, req.Region, req.Nonce)
	messageBytes := []byte(message)

	// Decode public key and signature
	pubKeyBytes, err := base64.StdEncoding.DecodeString(req.PubkeyB64)
	if err != nil {
		return fmt.Errorf("invalid public key encoding: %w", err)
	}

	signatureBytes, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}

	// Verify signature
	pubKey := ed25519.PublicKey(pubKeyBytes)
	if !ed25519.Verify(pubKey, messageBytes, signatureBytes) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

// handlePingRequest handles ping requests
func (sn *SuperNode) handlePingRequest(req *controlProto.PingRequest, stream controlProto.ControlStream_PersistentControlStreamServer) error {
	now := time.Now()
	latencyMs := float64(now.UnixMilli() - req.Timestamp)

	// Update heartbeat
	sn.streamManager.UpdateHeartbeat(req.PeerId, latencyMs)

	// Send pong response
	response := &controlProto.ControlMessage{
		MessageId: fmt.Sprintf("pong-%d", now.UnixNano()),
		Timestamp: now.Unix(),
		Payload: &controlProto.ControlMessage_PongResponse{
			PongResponse: &controlProto.PongResponse{
				Timestamp:         now.UnixMilli(),
				OriginalTimestamp: req.Timestamp,
				PeerId:           req.PeerId,
			},
		},
	}

	return stream.Send(response)
}

// handleCommandResponse handles command responses from peers
func (sn *SuperNode) handleCommandResponse(peerID string, resp *controlProto.CommandResponse) {
	sn.streamManager.UpdateCommandResult(peerID, resp.Success)

	sn.logger.WithFields(logrus.Fields{
		"peer_id":    peerID,
		"command_id": resp.CommandId,
		"success":    resp.Success,
		"message":    resp.Message,
	}).Info("Received command response")
}

// handleInfoRequest handles info requests
func (sn *SuperNode) handleInfoRequest(req *controlProto.InfoRequest, stream controlProto.ControlStream_PersistentControlStreamServer) error {
	info := make(map[string]string)

	// Provide requested information
	for _, field := range req.RequestedFields {
		switch field {
		case "active_peers":
			info[field] = fmt.Sprintf("%d", len(sn.streamManager.GetActiveStreams()))
		case "region":
			info[field] = sn.region
		case "supernode_id":
			info[field] = sn.id
		default:
			info[field] = "unknown"
		}
	}

	response := &controlProto.ControlMessage{
		MessageId: fmt.Sprintf("info-resp-%d", time.Now().UnixNano()),
		Timestamp: time.Now().Unix(),
		Payload: &controlProto.ControlMessage_InfoResponse{
			InfoResponse: &controlProto.InfoResponse{
				PeerId: req.PeerId,
				Info:   info,
			},
		},
	}

	return stream.Send(response)
}

// RequestExitPeer handles requests for exit peers from other SuperNodes
func (sn *SuperNode) RequestExitPeer(ctx context.Context, req *controlProto.RequestExitPeerRequest) (*controlProto.RequestExitPeerResponse, error) {
	// Find available exit peers (including hybrid peers)
	exitPeers := sn.streamManager.GetStreamsByRole(RoleExit)
	hybridPeers := sn.streamManager.GetStreamsByRole(RoleHybrid)
	
	// Combine exit and hybrid peers
	allExitPeers := append(exitPeers, hybridPeers...)
	
	if len(allExitPeers) == 0 {
		return &controlProto.RequestExitPeerResponse{
			Success: false,
			Message: "No exit peers available",
		}, nil
	}

	// Select the first available exit peer (simple selection)
	selectedPeer := allExitPeers[0]

	// Generate session ID for this connection
	sessionID := fmt.Sprintf("%s-%s-%d", req.ClientId, selectedPeer.PeerID, time.Now().Unix())

	// Send SETUP_EXIT command to the exit peer
	setupCommand := &controlProto.Command{
		CommandId: fmt.Sprintf("setup-exit-%d", time.Now().UnixNano()),
		Type:      controlProto.CommandType_SETUP_EXIT,
		Payload: map[string]string{
			"client_id":   req.ClientId,
			"session_id":  sessionID,
			"allowed_ips": "0.0.0.0/0", // Allow all traffic
		},
	}

	if err := sn.streamManager.SendCommandToPeer(selectedPeer.PeerID, setupCommand); err != nil {
		return &controlProto.RequestExitPeerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to setup exit peer: %v", err),
		}, status.Errorf(codes.Internal, "failed to setup exit peer: %v", err)
	}

	// Return exit peer info
	exitPeerInfo := &controlProto.ExitPeerInfo{
		PeerId:                   selectedPeer.PeerID,
		PublicKey:               selectedPeer.PublicKey,
		Endpoint:                fmt.Sprintf("%s:%d", sn.getPublicIP(), sn.relayPort),
		AllowedIps:              []string{"0.0.0.0/0"},
		SupportsDirectConnection: false, // We'll use relay for now
	}

	return &controlProto.RequestExitPeerResponse{
		Success:   true,
		Message:   "Exit peer allocated successfully",
		ExitPeer:  exitPeerInfo,
		SessionId: sessionID,
	}, nil
}

// registerWithBaseNode registers this SuperNode with the BaseNode
func (sn *SuperNode) registerWithBaseNode() error {
	ip, port, err := utils.ParseEndpoint(sn.listenAddr)
	if err != nil {
		return fmt.Errorf("invalid listen address: %w", err)
	}

	req := &proto.RegisterSuperNodeRequest{
		Region:       sn.region,
		SupernodeId:  sn.id,
		IpAddress:    ip,
		Port:         int32(port),
		CurrentLoad:  0,
		MaxCapacity:  1000,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := sn.baseClient.RegisterSuperNode(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to register with BaseNode: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("BaseNode registration failed: %s", resp.Message)
	}

	sn.logger.WithField("supernode_id", sn.id).Info("Successfully registered with BaseNode")
	return nil
}

// heartbeatLoop sends periodic heartbeats to BaseNode
func (sn *SuperNode) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if err := sn.registerWithBaseNode(); err != nil {
			sn.logger.WithError(err).Error("Failed to send heartbeat to BaseNode")
		}
	}
}

// staleStreamChecker removes stale streams
func (sn *SuperNode) staleStreamChecker() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		sn.streamManager.CheckStaleStreams(2 * time.Minute)
	}
}

// getPublicIP gets the public IP of this SuperNode
func (sn *SuperNode) getPublicIP() string {
	// Extract IP from listen address
	parts := strings.Split(sn.listenAddr, ":")
	if len(parts) > 0 && parts[0] != "" && parts[0] != "0.0.0.0" {
		return parts[0]
	}
	return "127.0.0.1" // Fallback for testing
}
package server

import (
	"fmt"
	"sync"
	"time"

	"myDvpn/clientPeer/proto"
	"github.com/sirupsen/logrus"
)

// PeerRole represents the role of a peer
type PeerRole string

const (
	RoleClient    PeerRole = "client"
	RoleExit      PeerRole = "exit"
	RoleSuperNode PeerRole = "supernode"
	RoleHybrid    PeerRole = "hybrid"
)

// StreamInfo contains information about an active stream
type StreamInfo struct {
	PeerID        string
	Role          PeerRole
	Region        string
	SessionID     string
	Stream        proto.ControlStream_PersistentControlStreamServer
	LastHeartbeat time.Time
	PublicKey     string
	IsActive      bool
	Stats         *PeerStats
	mutex         sync.RWMutex
}

// PeerStats holds statistics for a peer
type PeerStats struct {
	MessagesReceived   int64
	MessagesSent       int64
	CommandsExecuted   int64
	CommandsFailed     int64
	LatencyMs         float64
	ConnectedSince    time.Time
}

// StreamManager manages all active control streams
type StreamManager struct {
	streams    map[string]*StreamInfo // peer_id -> StreamInfo
	streamsMux sync.RWMutex
	logger     *logrus.Logger

	// Metrics
	activeStreams      int64
	authFailures       int64
	commandsProcessed  int64
	commandsSucceeded  int64
	commandsFailed     int64
}

// NewStreamManager creates a new stream manager
func NewStreamManager(logger *logrus.Logger) *StreamManager {
	return &StreamManager{
		streams: make(map[string]*StreamInfo),
		logger:  logger,
	}
}

// RegisterStream registers a new peer stream
func (sm *StreamManager) RegisterStream(peerID string, role PeerRole, region string, 
	publicKey string, stream proto.ControlStream_PersistentControlStreamServer) (string, error) {
	
	sm.streamsMux.Lock()
	defer sm.streamsMux.Unlock()

	// Generate session ID
	sessionID := fmt.Sprintf("%s-%d", peerID, time.Now().Unix())

	// Check if peer already has an active stream
	if existing, exists := sm.streams[peerID]; exists && existing.IsActive {
		sm.logger.WithFields(logrus.Fields{
			"peer_id": peerID,
			"role":    role,
		}).Warn("Peer already has active stream, replacing")
		
		existing.IsActive = false
	}

	// Create new stream info
	streamInfo := &StreamInfo{
		PeerID:        peerID,
		Role:          role,
		Region:        region,
		SessionID:     sessionID,
		Stream:        stream,
		LastHeartbeat: time.Now(),
		PublicKey:     publicKey,
		IsActive:      true,
		Stats: &PeerStats{
			ConnectedSince: time.Now(),
		},
	}

	sm.streams[peerID] = streamInfo
	sm.activeStreams++

	sm.logger.WithFields(logrus.Fields{
		"peer_id":    peerID,
		"role":       role,
		"region":     region,
		"session_id": sessionID,
	}).Info("Registered new peer stream")

	return sessionID, nil
}

// UnregisterStream removes a peer stream
func (sm *StreamManager) UnregisterStream(peerID string) {
	sm.streamsMux.Lock()
	defer sm.streamsMux.Unlock()

	if streamInfo, exists := sm.streams[peerID]; exists {
		streamInfo.IsActive = false
		delete(sm.streams, peerID)
		sm.activeStreams--

		sm.logger.WithFields(logrus.Fields{
			"peer_id": peerID,
			"role":    streamInfo.Role,
		}).Info("Unregistered peer stream")
	}
}

// GetStream gets stream info for a peer
func (sm *StreamManager) GetStream(peerID string) (*StreamInfo, bool) {
	sm.streamsMux.RLock()
	defer sm.streamsMux.RUnlock()

	streamInfo, exists := sm.streams[peerID]
	if exists && streamInfo.IsActive {
		return streamInfo, true
	}
	return nil, false
}

// GetActiveStreams returns all active streams
func (sm *StreamManager) GetActiveStreams() []*StreamInfo {
	sm.streamsMux.RLock()
	defer sm.streamsMux.RUnlock()

	var active []*StreamInfo
	for _, streamInfo := range sm.streams {
		if streamInfo.IsActive {
			active = append(active, streamInfo)
		}
	}
	return active
}

// GetStreamsByRole returns streams filtered by role
func (sm *StreamManager) GetStreamsByRole(role PeerRole) []*StreamInfo {
	sm.streamsMux.RLock()
	defer sm.streamsMux.RUnlock()

	var filtered []*StreamInfo
	for _, streamInfo := range sm.streams {
		if streamInfo.IsActive && streamInfo.Role == role {
			filtered = append(filtered, streamInfo)
		}
	}
	return filtered
}

// SendCommandToPeer sends a command to a specific peer
func (sm *StreamManager) SendCommandToPeer(peerID string, command *proto.Command) error {
	streamInfo, exists := sm.GetStream(peerID)
	if !exists {
		return fmt.Errorf("no active stream for peer %s", peerID)
	}

	streamInfo.mutex.Lock()
	defer streamInfo.mutex.Unlock()

	message := &proto.ControlMessage{
		MessageId: fmt.Sprintf("cmd-%d", time.Now().UnixNano()),
		Timestamp: time.Now().Unix(),
		Payload: &proto.ControlMessage_Command{
			Command: command,
		},
	}

	if err := streamInfo.Stream.Send(message); err != nil {
		sm.commandsFailed++
		return fmt.Errorf("failed to send command to peer %s: %w", peerID, err)
	}

	streamInfo.Stats.MessagesSent++
	sm.commandsProcessed++

	sm.logger.WithFields(logrus.Fields{
		"peer_id":    peerID,
		"command_id": command.CommandId,
		"command_type": command.Type,
	}).Info("Sent command to peer")

	return nil
}

// UpdateHeartbeat updates the last heartbeat time for a peer
func (sm *StreamManager) UpdateHeartbeat(peerID string, latencyMs float64) {
	if streamInfo, exists := sm.GetStream(peerID); exists {
		streamInfo.mutex.Lock()
		defer streamInfo.mutex.Unlock()
		
		streamInfo.LastHeartbeat = time.Now()
		streamInfo.Stats.LatencyMs = latencyMs
		streamInfo.Stats.MessagesReceived++
	}
}

// UpdateCommandResult updates command execution statistics
func (sm *StreamManager) UpdateCommandResult(peerID string, success bool) {
	if streamInfo, exists := sm.GetStream(peerID); exists {
		streamInfo.mutex.Lock()
		defer streamInfo.mutex.Unlock()
		
		streamInfo.Stats.CommandsExecuted++
		if success {
			sm.commandsSucceeded++
		} else {
			streamInfo.Stats.CommandsFailed++
			sm.commandsFailed++
		}
	}
}

// CheckStaleStreams removes streams that haven't sent heartbeat recently
func (sm *StreamManager) CheckStaleStreams(timeout time.Duration) {
	sm.streamsMux.Lock()
	defer sm.streamsMux.Unlock()

	now := time.Now()
	var staleStreams []string

	for peerID, streamInfo := range sm.streams {
		if streamInfo.IsActive && now.Sub(streamInfo.LastHeartbeat) > timeout {
			staleStreams = append(staleStreams, peerID)
		}
	}

	for _, peerID := range staleStreams {
		sm.logger.WithField("peer_id", peerID).Warn("Removing stale stream")
		streamInfo := sm.streams[peerID]
		streamInfo.IsActive = false
		delete(sm.streams, peerID)
		sm.activeStreams--
	}
}

// GetMetrics returns current metrics
func (sm *StreamManager) GetMetrics() map[string]interface{} {
	sm.streamsMux.RLock()
	defer sm.streamsMux.RUnlock()

	return map[string]interface{}{
		"active_streams_total":      sm.activeStreams,
		"stream_auth_failures_total": sm.authFailures,
		"commands_processed_total":  sm.commandsProcessed,
		"commands_succeeded_total":  sm.commandsSucceeded,
		"commands_failed_total":     sm.commandsFailed,
	}
}

// IncrementAuthFailures increments auth failure counter
func (sm *StreamManager) IncrementAuthFailures() {
	sm.authFailures++
}
package server

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"myDvpn/base/proto"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// BaseNode represents the base node server
type BaseNode struct {
	proto.UnimplementedBaseNodeServer

	listenAddr   string
	supernodes   map[string]*proto.SuperNodeInfo
	supernodesMux sync.RWMutex
	logger       *logrus.Logger
	server       *grpc.Server
}

// NewBaseNode creates a new BaseNode
func NewBaseNode(listenAddr string, logger *logrus.Logger) *BaseNode {
	return &BaseNode{
		listenAddr: listenAddr,
		supernodes: make(map[string]*proto.SuperNodeInfo),
		logger:     logger,
	}
}

// Start starts the BaseNode server
func (bn *BaseNode) Start() error {
	listener, err := net.Listen("tcp", bn.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", bn.listenAddr, err)
	}

	bn.server = grpc.NewServer()
	proto.RegisterBaseNodeServer(bn.server, bn)

	bn.logger.WithField("addr", bn.listenAddr).Info("Starting BaseNode server")

	// Start background cleanup task
	go bn.cleanupStaleSupernodes()

	return bn.server.Serve(listener)
}

// Stop stops the BaseNode server
func (bn *BaseNode) Stop() {
	if bn.server != nil {
		bn.server.GracefulStop()
	}
}

// RegisterSuperNode registers a SuperNode
func (bn *BaseNode) RegisterSuperNode(ctx context.Context, req *proto.RegisterSuperNodeRequest) (*proto.RegisterSuperNodeResponse, error) {
	bn.supernodesMux.Lock()
	defer bn.supernodesMux.Unlock()

	// Validate request
	if req.SupernodeId == "" {
		return &proto.RegisterSuperNodeResponse{
			Success: false,
			Message: "SuperNode ID is required",
		}, status.Errorf(codes.InvalidArgument, "SuperNode ID is required")
	}

	if req.Region == "" {
		return &proto.RegisterSuperNodeResponse{
			Success: false,
			Message: "Region is required",
		}, status.Errorf(codes.InvalidArgument, "Region is required")
	}

	// Update or create SuperNode info
	supernodeInfo := &proto.SuperNodeInfo{
		SupernodeId:   req.SupernodeId,
		Region:        req.Region,
		IpAddress:     req.IpAddress,
		Port:          req.Port,
		CurrentLoad:   req.CurrentLoad,
		MaxCapacity:   req.MaxCapacity,
		LastHeartbeat: time.Now().Unix(),
	}

	bn.supernodes[req.SupernodeId] = supernodeInfo

	bn.logger.WithFields(logrus.Fields{
		"supernode_id": req.SupernodeId,
		"region":       req.Region,
		"ip_address":   req.IpAddress,
		"port":         req.Port,
		"current_load": req.CurrentLoad,
		"max_capacity": req.MaxCapacity,
	}).Info("SuperNode registered/updated")

	return &proto.RegisterSuperNodeResponse{
		Success: true,
		Message: "SuperNode registered successfully",
	}, nil
}

// RequestExitRegion returns candidate SuperNodes for a specific region
func (bn *BaseNode) RequestExitRegion(ctx context.Context, req *proto.RequestExitRegionRequest) (*proto.RequestExitRegionResponse, error) {
	bn.supernodesMux.RLock()
	defer bn.supernodesMux.RUnlock()

	if req.TargetRegion == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Target region is required")
	}

	var candidates []*proto.SuperNodeInfo

	// Find SuperNodes in the target region
	for _, supernode := range bn.supernodes {
		if supernode.Region == req.TargetRegion {
			// Check if SuperNode is not overloaded
			if supernode.CurrentLoad < supernode.MaxCapacity {
				// Check if heartbeat is recent (within last 2 minutes)
				if time.Now().Unix()-supernode.LastHeartbeat < 120 {
					candidates = append(candidates, supernode)
				}
			}
		}
	}

	// Sort candidates by load (simple selection - choose least loaded)
	if len(candidates) > 1 {
		// Simple bubble sort by current load
		for i := 0; i < len(candidates); i++ {
			for j := i + 1; j < len(candidates); j++ {
				if candidates[i].CurrentLoad > candidates[j].CurrentLoad {
					candidates[i], candidates[j] = candidates[j], candidates[i]
				}
			}
		}
	}

	bn.logger.WithFields(logrus.Fields{
		"target_region":        req.TargetRegion,
		"requesting_supernode": req.RequestingSupernodeId,
		"candidates_found":     len(candidates),
	}).Info("Exit region request processed")

	return &proto.RequestExitRegionResponse{
		CandidateSupernodes: candidates,
	}, nil
}

// ListSuperNodes returns all registered SuperNodes
func (bn *BaseNode) ListSuperNodes(ctx context.Context, req *proto.ListSuperNodesRequest) (*proto.ListSuperNodesResponse, error) {
	bn.supernodesMux.RLock()
	defer bn.supernodesMux.RUnlock()

	var supernodes []*proto.SuperNodeInfo
	for _, supernode := range bn.supernodes {
		supernodes = append(supernodes, supernode)
	}

	bn.logger.WithField("total_supernodes", len(supernodes)).Info("Listed all SuperNodes")

	return &proto.ListSuperNodesResponse{
		Supernodes: supernodes,
	}, nil
}

// cleanupStaleSupernodes removes SuperNodes that haven't sent heartbeat recently
func (bn *BaseNode) cleanupStaleSupernodes() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		bn.supernodesMux.Lock()

		var staleSupernodes []string
		now := time.Now().Unix()

		for id, supernode := range bn.supernodes {
			// Remove SuperNodes that haven't sent heartbeat in 5 minutes
			if now-supernode.LastHeartbeat > 300 {
				staleSupernodes = append(staleSupernodes, id)
			}
		}

		for _, id := range staleSupernodes {
			supernode := bn.supernodes[id]
			delete(bn.supernodes, id)

			bn.logger.WithFields(logrus.Fields{
				"supernode_id":   id,
				"region":         supernode.Region,
				"last_heartbeat": supernode.LastHeartbeat,
			}).Warn("Removed stale SuperNode")
		}

		bn.supernodesMux.Unlock()
	}
}

// GetMetrics returns current metrics
func (bn *BaseNode) GetMetrics() map[string]interface{} {
	bn.supernodesMux.RLock()
	defer bn.supernodesMux.RUnlock()

	regionCount := make(map[string]int)
	totalLoad := int64(0)
	totalCapacity := int64(0)

	for _, supernode := range bn.supernodes {
		regionCount[supernode.Region]++
		totalLoad += int64(supernode.CurrentLoad)
		totalCapacity += int64(supernode.MaxCapacity)
	}

	return map[string]interface{}{
		"total_supernodes":   len(bn.supernodes),
		"regions":           regionCount,
		"total_load":        totalLoad,
		"total_capacity":    totalCapacity,
		"utilization_pct":   float64(totalLoad) / float64(totalCapacity) * 100,
	}
}
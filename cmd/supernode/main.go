package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"myDvpn/super/server"
	"github.com/sirupsen/logrus"
)

func main() {
	// Parse command line flags
	id := flag.String("id", "supernode-1", "SuperNode ID")
	region := flag.String("region", "us-east-1", "Region")
	listenAddr := flag.String("listen", "0.0.0.0:50052", "Address to listen on")
	baseNodeAddr := flag.String("basenode", "localhost:50051", "BaseNode address")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	// Setup logger
	logger := logrus.New()
	level, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		logger.Fatal("Invalid log level")
	}
	logger.SetLevel(level)

	// Create SuperNode
	superNode := server.NewSuperNode(*id, *region, *listenAddr, *baseNodeAddr, logger)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		logger.WithFields(logrus.Fields{
			"id":       *id,
			"region":   *region,
			"addr":     *listenAddr,
			"basenode": *baseNodeAddr,
		}).Info("Starting SuperNode")
		if err := superNode.Start(); err != nil {
			logger.WithError(err).Fatal("SuperNode failed")
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	logger.Info("Shutting down SuperNode")
	superNode.Stop()
}
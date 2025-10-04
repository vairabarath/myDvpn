package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"myDvpn/base/server"
	"github.com/sirupsen/logrus"
)

func main() {
	// Parse command line flags
	listenAddr := flag.String("listen", "0.0.0.0:50051", "Address to listen on")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	// Setup logger
	logger := logrus.New()
	level, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		logger.Fatal("Invalid log level")
	}
	logger.SetLevel(level)

	// Create BaseNode
	baseNode := server.NewBaseNode(*listenAddr, logger)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		logger.WithField("addr", *listenAddr).Info("Starting BaseNode")
		if err := baseNode.Start(); err != nil {
			logger.WithError(err).Fatal("BaseNode failed")
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	logger.Info("Shutting down BaseNode")
	baseNode.Stop()
}
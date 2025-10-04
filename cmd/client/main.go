package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"myDvpn/clientPeer/client"
	"github.com/sirupsen/logrus"
)

func main() {
	// Parse command line flags
	id := flag.String("id", "client-1", "Client peer ID")
	region := flag.String("region", "us-east-1", "Region")
	supernodeAddr := flag.String("supernode", "localhost:50052", "SuperNode address")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	// Setup logger
	logger := logrus.New()
	level, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		logger.Fatal("Invalid log level")
	}
	logger.SetLevel(level)

	// Create client peer
	peer, err := client.NewPeer(*id, *region, *supernodeAddr, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create client peer")
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start peer
	go func() {
		logger.WithFields(logrus.Fields{
			"id":        *id,
			"region":    *region,
			"supernode": *supernodeAddr,
		}).Info("Starting client peer")
		if err := peer.Start(); err != nil {
			logger.WithError(err).Fatal("Client peer failed")
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	logger.Info("Shutting down client peer")
	peer.Stop()
}
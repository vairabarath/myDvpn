package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"myDvpn/exitpeer"
	"github.com/sirupsen/logrus"
)

func main() {
	// Parse command line flags
	id := flag.String("id", "exit-1", "Exit peer ID")
	region := flag.String("region", "us-west-1", "Region")
	supernodeAddr := flag.String("supernode", "localhost:50053", "SuperNode address")
	listenPort := flag.Int("port", 51820, "WireGuard listen port")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	// Setup logger
	logger := logrus.New()
	level, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		logger.Fatal("Invalid log level")
	}
	logger.SetLevel(level)

	// Create exit peer
	exitPeer, err := exitpeer.NewExitPeer(*id, *region, *supernodeAddr, *listenPort, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create exit peer")
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start peer
	go func() {
		logger.WithFields(logrus.Fields{
			"id":         *id,
			"region":     *region,
			"supernode":  *supernodeAddr,
			"port":       *listenPort,
		}).Info("Starting exit peer")
		if err := exitPeer.Start(); err != nil {
			logger.WithError(err).Fatal("Exit peer failed")
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	logger.Info("Shutting down exit peer")
	exitPeer.Stop()
}
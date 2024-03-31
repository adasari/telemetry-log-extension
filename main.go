package main

import (
	"context"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"

	"github.com/adasari/telemetry-log-extension/extension"
	"github.com/adasari/telemetry-log-extension/telemetry"
	"github.com/adasari/telemetry-log-extension/utils"
	"github.com/adasari/telemetry-log-extension/writer"

	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetFormatter(&log.JSONFormatter{})
	// TODO: use env variable to control the log level
	log.SetLevel(log.InfoLevel)

	extensionEnabled := os.Getenv("TELEMETRY_EXTENSION_ENABLED")
	if strings.ToLower(extensionEnabled) != "true" {
		log.Info("telemetry lambda extension is not enabled")
		return
	}
	runtimeApiUrl := utils.GetEnvOrDefault("AWS_LAMBDA_RUNTIME_API", "127.0.0.1:9001")
	extensionName := path.Base(os.Args[0])
	ctx, cancel := context.WithCancel(context.Background())

	// exit on SIGTERM or SIGINT
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	var ws []writer.Writer
	fluentDEnabled := os.Getenv("TELEMETRY_EXTENSION_FLUENTD_ENABLED")
	if fluentDEnabled == "true" {
		fw, err := writer.NewFluentDWriter()
		if err != nil {
			log.Errorf("failed to create fluentd writer: %v", err)
			os.Exit(1)
		}
		ws = append(ws, fw)
	}

	if len(ws) == 0 {
		log.Error("no writers configured, exiting the extension")
		os.Exit(1)
	}

	/*
	 Telemetry Lambda extension steps:
	 1. Register extension
	 2. Run http listener to process the telemetry info
	 3. Subscribe to telemetry API
	*/

	// Register the extension
	extensionApiClient := extension.NewClient(runtimeApiUrl, extensionName)
	extensionId, err := extensionApiClient.Register(ctx)
	if err != nil {
		log.Errorf("failed to register extension %s : %v", extensionName, err)
		os.Exit(1)
	}

	log.Infof("registered extension '%s' and extensionId is '%s'", extensionName, extensionId)

	// Run http listener
	telemetryListener := telemetry.NewTelemetryApiListener(ws...)
	telemetryListenerUri, err := telemetryListener.Start()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	// Subscribe to telemetry API
	log.Infof("subscribing to telemetry API - %s", telemetryListenerUri)
	telemetryApiClient := telemetry.NewClient()
	_, err = telemetryApiClient.Subscribe(ctx, extensionId, telemetryListenerUri)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	flushTelemetryInfo := func() {
		for _, w := range ws {
			w.Flush(ctx)
		}
	}

	// Will block until invoke or shutdown event is received or sigterm is received.
	log.Info("starting infinite loop")
	for {
		select {
		case <-ctx.Done():
			log.Info("received context done signal")
			return
		case <-exit:
			log.Info("received sigterm/sigint/sigquit/interrupt signal")
			flushTelemetryInfo()
			cancel()
			return
		default:
			log.Debug("waiting for next event...")

			// This is a blocking action
			res, err := extensionApiClient.NextEvent(ctx, extensionId)
			if err != nil {
				log.Errorf("failed to get next event. Ignoring: %v", err)
				return
			}

			if res.EventType == extension.Shutdown {
				// Dispatch all remaining telemetry, handle shutdown
				flushTelemetryInfo()
				return
			}
		}
	}
}

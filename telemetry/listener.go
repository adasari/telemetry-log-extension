package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/adasari/telemetry-log-extension/writer"

	log "github.com/sirupsen/logrus"
)

const DefaultHttpListenerPort = "4323"

// Used to listen to the Telemetry API
type TelemetryApiListener struct {
	httpServer *http.Server
	writers    []writer.Writer
}

// TelemetryMessage is a message sent from the Logs API
type TelemetryMessage struct {
	Type   string      `json:"type"`
	Time   string      `json:"time"`
	Record interface{} `json:"record"`
}

func NewTelemetryApiListener(writers ...writer.Writer) *TelemetryApiListener {
	return &TelemetryApiListener{
		httpServer: nil,
		writers:    writers,
	}
}

func ListenOnAddress() string {
	env_aws_local, ok := os.LookupEnv("AWS_SAM_LOCAL")
	if ok && "true" == env_aws_local {
		return ":" + DefaultHttpListenerPort
	}

	return "sandbox:" + DefaultHttpListenerPort
}

// Starts the server in a goroutine where the log events will be sent
func (s *TelemetryApiListener) Start() (string, error) {
	address := ListenOnAddress()
	log.Infof("Running telemetry listener on address: %v", address)
	s.httpServer = &http.Server{Addr: address}
	http.HandleFunc("/", s.http_handler)
	go func() {
		err := s.httpServer.ListenAndServe()
		if err != http.ErrServerClosed {
			log.Error("unexpected stop on http server:", err)
			s.Shutdown()
		} else {
			log.Info("http server closed:", err)
		}
	}()
	return fmt.Sprintf("http://%s", address), nil
}

// http_handler handles the requests coming from the Telemetry API.
// Everytime Telemetry API sends log events, this function will read them from the response body
// and put into a synchronous queue to be dispatched later.
// Logging or printing besides the error cases below is not recommended if you have subscribed to
// receive extension logs. Otherwise, logging here will cause Telemetry API to send new logs for
// the printed lines which may create an infinite loop.
func (s *TelemetryApiListener) http_handler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Errorf("error reading body: %v", err)
		return
	}

	defer r.Body.Close()

	region := os.Getenv("AWS_REGION")
	functionName := os.Getenv("AWS_LAMBDA_FUNCTION_NAME")
	// Parse, format and put the log messages into the queue
	var records []TelemetryMessage
	err = json.Unmarshal(body, &records)
	if err != nil {
		log.Errorf("failed to unmarshal telemetry response: %v", err)
		return
	}

	for _, r := range records {
		formattedRecord := map[string]interface{}{
			"time":          r.Time,
			"record_type":   r.Type,
			"region":        region,
			"function_name": functionName,
		}

		switch record := r.Record.(type) {
		case string:
			// check if it is json string. if yes, extract
			var jsonRecord map[string]interface{}
			err := json.Unmarshal([]byte(record), &jsonRecord)
			if err != nil {
				formattedRecord["message"] = record
			} else {
				// valid json
				for k, v := range jsonRecord {
					formattedRecord[k] = v
				}
			}
		case map[string]interface{}:
			for k, v := range record {
				formattedRecord[k] = v
			}
		default:
			formattedRecord["raw"] = record
		}

		for _, w := range s.writers {
			w.Write(ctx, formattedRecord)
		}
	}
}

// Terminates the HTTP server listening for logs
func (s *TelemetryApiListener) Shutdown() {
	if s.httpServer != nil {
		ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
		err := s.httpServer.Shutdown(ctx)
		if err != nil {
			log.Error("failed to shutdown the http server:", err)
		} else {
			s.httpServer = nil
		}
	}
}

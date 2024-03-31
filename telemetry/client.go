package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/adasari/telemetry-log-extension/utils"
	log "github.com/sirupsen/logrus"
)

// The client used for subscribing to the Telemetry API
type Client struct {
	httpClient      *http.Client
	baseUrl         string
	subscribeEvents []EventType
}

func NewClient() *Client {
	baseUrl := fmt.Sprintf("http://%s/%s/telemetry", os.Getenv("AWS_LAMBDA_RUNTIME_API"), SchemaVersionLatest)

	return &Client{
		httpClient:      &http.Client{},
		baseUrl:         baseUrl,
		subscribeEvents: resolveSubscribeEvents(os.Getenv("TELEMETRY_EXTENSION_SUBSCRIBE_EVENTS")),
	}
}

func resolveSubscribeEvents(subscribeEvents string) []EventType {
	var events []EventType
	if subscribeEvents != "" {
		sEvents := strings.Split(subscribeEvents, ",")

		for _, e := range sEvents {
			switch strings.ToLower(e) {
			case string(Platform):
				events = append(events, Platform)
			case string(Function):
				events = append(events, Function)
			case string(Extension):
				events = append(events, Extension)
			default:
				log.Warnf("invalid subscribe type: '%s'", e)
			}
		}

	}

	if len(events) == 0 {
		events = append(events, Function)
	}

	return events
}

// Represents the type of log events in Lambda
type EventType string

const (
	// Used to receive log events emitted by the platform
	Platform EventType = "platform"
	// Used to receive log events emitted by the function
	Function EventType = "function"
	// Used is to receive log events emitted by the extension
	Extension EventType = "extension"
)

// Configuration for receiving telemetry from the Telemetry API.
// Telemetry will be sent to your listener when one of the conditions below is met.
type BufferingCfg struct {
	// Maximum number of log events to be buffered in memory. (default: 10000, minimum: 1000, maximum: 10000)
	MaxItems uint32 `json:"maxItems"`
	// Maximum size in bytes of the log events to be buffered in memory. (default: 262144, minimum: 262144, maximum: 1048576)
	MaxBytes uint32 `json:"maxBytes"`
	// Maximum time (in milliseconds) for a batch to be buffered. (default: 1000, minimum: 100, maximum: 30000)
	TimeoutMS uint32 `json:"timeoutMs"`
}

// URI is used to set the endpoint where the logs will be sent to
type URI string

// HttpMethod represents the HTTP method used to receive logs from Logs API
type HttpMethod string

const (
	// Receive log events via POST requests to the listener
	HttpPost HttpMethod = "POST"
	// Receive log events via PUT requests to the listener
	HttpPut HttpMethod = "PUT"
)

// Used to specify the protocol when subscribing to Telemetry API for HTTP
type HttpProtocol string

const (
	HttpProto HttpProtocol = "HTTP"
)

// Denotes what the content is encoded in
type HttpEncoding string

const (
	JSON HttpEncoding = "JSON"
)

// Configuration for listeners that would like to receive telemetry via HTTP
type Destination struct {
	Protocol   HttpProtocol `json:"protocol"`
	URI        URI          `json:"URI"`
	HttpMethod HttpMethod   `json:"method"`
	Encoding   HttpEncoding `json:"encoding"`
}

type SchemaVersion string

const (
	SchemaVersionLatest = "2022-07-01"
)

// Request body that is sent to the Telemetry API on subscribe
type SubscribeRequest struct {
	SchemaVersion SchemaVersion `json:"schemaVersion"`
	EventTypes    []EventType   `json:"types"`
	BufferingCfg  BufferingCfg  `json:"buffering"`
	Destination   Destination   `json:"destination"`
}

// Response body that is received from the Telemetry API on subscribe
type SubscribeResponse struct {
	body string
}

// Subscribes to the Telemetry API to start receiving the log events
func (c *Client) Subscribe(ctx context.Context, extensionId string, listenerUri string) (*SubscribeResponse, error) {
	data, err := json.Marshal(
		&SubscribeRequest{
			SchemaVersion: SchemaVersionLatest,
			EventTypes:    c.subscribeEvents,
			BufferingCfg: BufferingCfg{
				MaxItems:  1000, // using default lambda values for now.
				MaxBytes:  256 * 1024,
				TimeoutMS: 1000,
			},
			Destination: Destination{
				Protocol:   HttpProto,
				HttpMethod: HttpPost,
				Encoding:   JSON,
				URI:        URI(listenerUri),
			},
		})

	if err != nil {
		return nil, fmt.Errorf("failed to marshal subscribe request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", c.baseUrl, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(utils.ExtensionIdentifierHeader, extensionId)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusAccepted {
		log.Error("subscription failed. Telemetry API is not supported! Is this extension running in a local sandbox?")
	} else if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to subscribe to telemetry api '%s': %d[%s]", c.baseUrl, resp.StatusCode, resp.Status)
		}

		return nil, fmt.Errorf("failed to subscribe to telemetry api '%s': %d[%s] %s", c.baseUrl, resp.StatusCode, resp.Status, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &SubscribeResponse{string(body)}, nil
}

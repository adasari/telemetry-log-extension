package extension

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

// EventType represents the type of events received from /event/next
type EventType string

// ShutdownReason represents the reason for a shutdown event
type ShutdownReason string

const (
	// Invoke is the lambda invoke event
	Invoke EventType = "INVOKE"
	// Shutdown is a shutdown event for the environment
	Shutdown EventType = "SHUTDOWN"
	// ShutdownReasonSpindown is a normal end to a function
	ShutdownReasonSpindown ShutdownReason = "spindown"
	// ShutdownReasonTimeout means the handler ran out of time
	ShutdownReasonTimeout ShutdownReason = "timeout"
	// ShutdownReasonFailure is any other shutdown type, such as out-of-memory
	ShutdownReasonFailure ShutdownReason = "failure"
)

// Client is used to communicate with the Extensions API
type Client struct {
	extensionName string
	baseURL       string
	httpClient    *http.Client
}

// RegisterResponse is the body of the response for /register
type RegisterResponse struct {
	FunctionName    string `json:"functionName"`
	FunctionVersion string `json:"functionVersion"`
	Handler         string `json:"handler"`
}

// Tracing is part of the response for /event/next
type Tracing struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// NextEventResponse is the response for /event/next
type NextEventResponse struct {
	EventType          EventType      `json:"eventType"`
	DeadlineMS         int64          `json:"deadlineMs"`
	RequestID          string         `json:"requestId"`
	InvokedFunctionARN string         `json:"invokedFunctionArn"`
	Tracing            Tracing        `json:"tracing"`
	ShutdownReason     ShutdownReason `json:"shutdownReason,omitempty"`
}

// NewClient returns a new Lambda Extensions API client
func NewClient(baseURL string, extensionName string) *Client {
	if !strings.HasPrefix(baseURL, "http") {
		baseURL = fmt.Sprintf("http://%s", baseURL)
	}
	baseURL = fmt.Sprintf("%s/2020-01-01/extension", baseURL)
	return &Client{
		baseURL:       baseURL,
		extensionName: extensionName,
		httpClient:    &http.Client{},
	}
}

// Register registers the extension with the Lambda Extensions API. This happens
// during Extension Init. Each call must include the list of events in the body
// and the extension name in the headers.
func (c *Client) Register(ctx context.Context) (string, error) {
	reqBody, err := json.Marshal(map[string]interface{}{
		"events": []EventType{Invoke, Shutdown},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.url("/register"), bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set(utils.extensionNameHeader, c.extensionName)
	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to register extension, http status: %s", res.Status)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	resp := RegisterResponse{}
	if err = json.Unmarshal(body, &resp); err != nil {
		return "", err
	}

	return res.Header.Get(extensionIdentifierHeader), nil
}

// NextEvent blocks while long polling for the next lambda invoke or shutdown
// By default, the Go HTTP client has no timeout, and in this case this is actually
// the desired behavior to enable long polling of the Extensions API.
func (c *Client) NextEvent(ctx context.Context, extensionId string) (*NextEventResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.url("/event/next"), nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set(extensionIdentifierHeader, extensionId)
	httpRes, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpRes.Body.Close()
	if httpRes.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get next event, http status: %s", httpRes.Status)
	}
	body, err := io.ReadAll(httpRes.Body)
	if err != nil {
		return nil, err
	}
	res := NextEventResponse{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// url is a helper function to build urls out of relative paths
func (c *Client) url(requestPath string) string {
	newURL, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Sprintf("%s%s", c.baseURL, requestPath)
	}
	newURL.Path = path.Join(newURL.Path, requestPath)
	return newURL.String()
}

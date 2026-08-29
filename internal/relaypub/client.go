// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package relaypub

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client publishes farm/season events into Zyvor Relay, either via a Pub/Sub
// gateway (topic name = event type) or directly to POST /v1/events.
type Client struct {
	RelayBase    string
	RelayToken   string
	GatewayBase  string // e.g. http://127.0.0.1:18083
	GatewayToken string // GATEWAY_AUTH_TOKEN when gateway requires auth
	Project      string
	HTTP         *http.Client
	TLSInsecure  bool
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if c.TLSInsecure {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // lab self-signed
	}
	c.HTTP = &http.Client{Timeout: 15 * time.Second, Transport: tr}
	return c.HTTP
}

// HTTPClient exposes the shared HTTP client for probes.
func (c *Client) HTTPClient() *http.Client {
	return c.httpClient()
}

type PublishResult struct {
	Path    string `json:"path"`
	EventID string `json:"event_id,omitempty"`
	Raw     string `json:"raw,omitempty"`
}

// PublishEventType posts an event. Prefer gateway when GatewayBase is set.
func (c *Client) PublishEventType(eventType, severity, source, idempotencyKey string, data map[string]any) (PublishResult, error) {
	if data == nil {
		data = map[string]any{}
	}
	if c.GatewayBase != "" {
		return c.publishViaGateway(eventType, severity, source, idempotencyKey, data)
	}
	return c.publishDirect(eventType, severity, source, idempotencyKey, data)
}

func (c *Client) publishViaGateway(eventType, severity, source, idempotencyKey string, data map[string]any) (PublishResult, error) {
	payload, _ := json.Marshal(data)
	attrs := map[string]string{
		"severity":        severity,
		"source":          source,
		"idempotency_key": idempotencyKey,
	}
	body := map[string]any{
		"messages": []map[string]any{{
			"data":       base64.StdEncoding.EncodeToString(payload),
			"attributes": attrs,
		}},
	}
	raw, _ := json.Marshal(body)
	project := c.Project
	if project == "" {
		project = "fasal-onprem"
	}
	base := strings.TrimRight(c.GatewayBase, "/")
	url := base + "/v1/projects/" + project + "/topics/" + eventType + ":publish"
	res, err := c.doGatewayPublish(url, raw)
	if err == nil {
		return res, nil
	}
	// Memory backends return 404 when the topic was never created — ensure then retry once.
	if strings.Contains(err.Error(), "404") {
		if cerr := c.ensureGatewayTopic(project, eventType); cerr != nil {
			return PublishResult{}, fmt.Errorf("%w (ensure topic: %v)", err, cerr)
		}
		return c.doGatewayPublish(url, raw)
	}
	return PublishResult{}, err
}

func (c *Client) doGatewayPublish(url string, raw []byte) (PublishResult, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return PublishResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.GatewayToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.GatewayToken)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return PublishResult{}, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode/100 != 2 {
		return PublishResult{}, fmt.Errorf("gateway publish %s: %s", resp.Status, string(b))
	}
	return PublishResult{Path: "gateway", Raw: string(b)}, nil
}

func (c *Client) ensureGatewayTopic(project, eventType string) error {
	base := strings.TrimRight(c.GatewayBase, "/")
	url := base + "/v1/projects/" + project + "/topics/" + eventType
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.GatewayToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.GatewayToken)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	// 200 create / 409 already exists are both fine
	if resp.StatusCode/100 == 2 || resp.StatusCode == http.StatusConflict {
		return nil
	}
	return fmt.Errorf("create topic %s: %s", resp.Status, string(b))
}

func (c *Client) publishDirect(eventType, severity, source, idempotencyKey string, data map[string]any) (PublishResult, error) {
	body := map[string]any{
		"type":            eventType,
		"severity":        severity,
		"source":          source,
		"idempotency_key": idempotencyKey,
		"data":            data,
	}
	raw, _ := json.Marshal(body)
	url := strings.TrimRight(c.RelayBase, "/") + "/v1/events"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return PublishResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.RelayToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.RelayToken)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return PublishResult{}, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode/100 != 2 {
		return PublishResult{}, fmt.Errorf("relay events %s: %s", resp.Status, string(b))
	}
	var out struct {
		Event struct {
			ID string `json:"id"`
		} `json:"event"`
	}
	_ = json.Unmarshal(b, &out)
	return PublishResult{Path: "relay", EventID: out.Event.ID, Raw: string(b)}, nil
}

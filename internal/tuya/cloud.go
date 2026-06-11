package tuya

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yashiels/orro-cli/internal/config"
)

// Command is a single DP code + value pair used by both LAN and Cloud.
type Command struct {
	Code  string `json:"code"`
	Value any    `json:"value"`
}

// Cloud is a Tuya Cloud API client with HMAC-SHA256 request signing.
type Cloud struct {
	cfg         *config.Config
	endpoint    string
	clientID    string
	secret      []byte
	deviceID    string
	accessToken string
	http        *http.Client
}

// NewCloud creates a Cloud client. Connect() must be called before any device operations.
func NewCloud(cfg *config.Config) *Cloud {
	return &Cloud{
		cfg:      cfg,
		endpoint: strings.TrimRight(cfg.Endpoint, "/"),
		clientID: cfg.AccessID,
		secret:   []byte(cfg.AccessSecret),
		deviceID: cfg.DeviceID,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

// Connect obtains an access token from the Tuya token endpoint.
func (c *Cloud) Connect() error {
	c.cfg.Debug("cloud: obtaining access token")
	payload, err := c.request("GET", "/v1.0/token", url.Values{"grant_type": {"1"}}, nil, false)
	if err != nil {
		return fmt.Errorf("cloud connect: %w", err)
	}
	result, ok := payload["result"].(map[string]any)
	if !ok {
		return fmt.Errorf("cloud connect: unexpected response: %v", payload)
	}
	token, _ := result["access_token"].(string)
	if token == "" {
		return fmt.Errorf("cloud connect: no access_token in response: %v", payload)
	}
	c.accessToken = token
	c.cfg.Debug("cloud: token obtained")
	return nil
}

// Status fetches current device DP values.
func (c *Cloud) Status() (map[string]any, error) {
	if err := c.Connect(); err != nil {
		return nil, err
	}

	// Try iot-03 endpoint first, fall back to legacy.
	payload, err := c.request("GET",
		fmt.Sprintf("/v1.0/iot-03/devices/%s/status", c.deviceID),
		nil, nil, false)
	if err != nil || !truthy(payload["success"]) {
		payload, err = c.request("GET",
			fmt.Sprintf("/v1.0/devices/%s/status", c.deviceID),
			nil, nil, true)
		if err != nil {
			return nil, fmt.Errorf("cloud status: %w", err)
		}
	}

	rows, _ := payload["result"].([]any)
	out := make(map[string]any, len(rows))
	for _, row := range rows {
		if m, ok := row.(map[string]any); ok {
			code, _ := m["code"].(string)
			if code != "" {
				out[code] = m["value"]
			}
		}
	}
	return out, nil
}

// Properties fetches shadow properties (includes memory_height preset data).
func (c *Cloud) Properties() (map[string]map[string]any, error) {
	if err := c.Connect(); err != nil {
		return nil, err
	}

	payload, err := c.request("GET",
		fmt.Sprintf("/v2.0/cloud/thing/%s/shadow/properties", c.deviceID),
		nil, nil, true)
	if err != nil {
		return nil, fmt.Errorf("cloud properties: %w", err)
	}

	result, _ := payload["result"].(map[string]any)
	rows, _ := result["properties"].([]any)
	out := make(map[string]map[string]any, len(rows))
	for _, row := range rows {
		if m, ok := row.(map[string]any); ok {
			code, _ := m["code"].(string)
			if code != "" {
				out[code] = m
			}
		}
	}
	return out, nil
}

// Send sends device commands via the Cloud API.
func (c *Cloud) Send(commands []Command) (map[string]any, error) {
	if err := c.Connect(); err != nil {
		return nil, err
	}

	body := map[string]any{"commands": commands}

	// Try iot-03 first.
	payload, err := c.request("POST",
		fmt.Sprintf("/v1.0/iot-03/devices/%s/commands", c.deviceID),
		nil, body, false)
	if err == nil && truthy(payload["success"]) {
		return payload, nil
	}

	// Fall back to legacy endpoint.
	legacy, err2 := c.request("POST",
		fmt.Sprintf("/v1.0/devices/%s/commands", c.deviceID),
		nil, body, false)
	if err2 != nil {
		return nil, fmt.Errorf("cloud send failed: iot-03=%v; legacy=%v", err, err2)
	}
	if !truthy(legacy["success"]) {
		return nil, fmt.Errorf("cloud send: iot-03=%v; legacy=%v", payload, legacy)
	}
	return legacy, nil
}

// ──────────────────────────────────────────────
// HTTP + signing
// ──────────────────────────────────────────────

func (c *Cloud) request(
	method, path string,
	params url.Values,
	body any,
	failOnError bool,
) (map[string]any, error) {
	c.cfg.Debug(fmt.Sprintf("cloud: %s %s", method, path))

	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}

	// Build query string.
	query := canonicalQuery(params)
	fullPath := path
	if query != "" {
		fullPath = path + "?" + query
	}

	headers := c.sign(method, path, query, bodyBytes)

	reqURL := c.endpoint + fullPath
	var reqBody io.Reader
	if len(bodyBytes) > 0 {
		reqBody = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(context.Background(), method, reqURL, reqBody)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		if failOnError {
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, respBody)
		}
		return nil, nil
	}

	if failOnError && (resp.StatusCode < 200 || resp.StatusCode >= 300 || !truthy(result["success"])) {
		return nil, fmt.Errorf("API %s %s failed: HTTP %d: %v", method, path, resp.StatusCode, result)
	}

	return result, nil
}

func (c *Cloud) sign(method, path, query string, bodyBytes []byte) map[string]string {
	bodyHash := sha256Hex(bodyBytes)
	urlPath := path
	if query != "" {
		urlPath = path + "?" + query
	}

	stringToSign := strings.Join([]string{
		strings.ToUpper(method),
		bodyHash,
		"",
		urlPath,
	}, "\n")

	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	tokenPart := c.accessToken

	signSource := c.clientID + tokenPart + ts + stringToSign

	h := hmac.New(sha256.New, c.secret)
	h.Write([]byte(signSource))
	signature := strings.ToUpper(hex.EncodeToString(h.Sum(nil)))

	headers := map[string]string{
		"client_id":    c.clientID,
		"sign":         signature,
		"t":            ts,
		"sign_method":  "HMAC-SHA256",
		"Content-Type": "application/json",
	}
	if c.accessToken != "" {
		headers["access_token"] = c.accessToken
	}
	return headers
}

// canonicalQuery sorts and URL-encodes query parameters for Tuya signing.
func canonicalQuery(params url.Values) string {
	if len(params) == 0 {
		return ""
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pairs []string
	for _, k := range keys {
		for _, v := range params[k] {
			pairs = append(pairs, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}
	return strings.Join(pairs, "&")
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func truthy(v any) bool {
	if v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

package client

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// APIResponse represents a response from Instagram's API
type APIResponse struct {
	Status    string         `json:"status"`
	Message   string         `json:"message,omitempty"`
	ErrorType string         `json:"error_type,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	RawBody   []byte         `json:"-"`
}

// APIError represents an Instagram API error
type APIError struct {
	StatusCode int
	Message    string
	ErrorType  string
	Response   *APIResponse
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("Instagram API error: %s (code: %d, type: %s)", e.Message, e.StatusCode, e.ErrorType)
	}
	return fmt.Sprintf("Instagram API error: status code %d", e.StatusCode)
}

// Common Instagram API errors
var (
	ErrBadCredentials         = &APIError{Message: "Invalid username or password"}
	ErrTwoFactorRequired      = &APIError{Message: "Two factor authentication required", ErrorType: "two_factor_required"}
	ErrChallengeRequired      = &APIError{Message: "Challenge required", ErrorType: "challenge_required"}
	ErrCheckpointRequired     = &APIError{Message: "Checkpoint required", ErrorType: "checkpoint_challenge_required"}
	ErrRateLimited            = &APIError{Message: "Rate limited, please wait", ErrorType: "rate_limit"}
	ErrReloginAttemptExceeded = &APIError{Message: "Relogin attempt exceeded"}
)

// baseHeaders returns the base headers for API requests
func (c *Client) baseHeaders() map[string]string {
	headers := map[string]string{
		"User-Agent":                  c.UserAgent,
		"Content-Type":                "application/x-www-form-urlencoded; charset=UTF-8",
		"Accept-Language":             c.Locale,
		"Accept-Encoding":             "gzip, deflate",
		"X-IG-Capabilities":           "3brTvw==",
		"X-IG-Connection-Type":        "WIFI",
		"X-IG-Connection-Speed":       strconv.Itoa(rand.Intn(3000)+1000) + "kbps",
		"X-IG-Bandwidth-Speed-KBPS":   "-1.000",
		"X-IG-Bandwidth-TotalBytes-B": "0",
		"X-IG-Bandwidth-TotalTime-MS": "0",
		"X-IG-App-Locale":             c.Locale,
		"X-IG-Device-Locale":          c.Locale,
		"X-IG-Mapped-Locale":          c.Locale,
		"X-Pigeon-Session-Id":         c.ClientSessionID,
		"X-Pigeon-Rawclienttime":      strconv.FormatFloat(float64(time.Now().UnixNano())/1e9, 'f', 3, 64),
		"X-IG-App-ID":                 IGAppID,
		"X-Bloks-Version-Id":          c.BloksVersioningID,
		"X-Bloks-Is-Layout-RTL":       "false",
		"X-Bloks-Is-Panorama-Enabled": "true",
		"X-FB-HTTP-Engine":            "Liger",
		"X-FB-Client-IP":              "True",
		"X-FB-Server-Cluster":         "True",
		"IG-INTENDED-USER-ID":         strconv.FormatInt(c.UserID(), 10),
		"X-IG-Nav-Chain":              "",
		"X-IG-SALT-IDS":               strconv.FormatInt(rand.Int63(), 10),
		"X-MID":                       c.Mid,
	}

	if c.IgWwwClaim != "" {
		headers["X-IG-WWW-Claim"] = c.IgWwwClaim
	} else {
		headers["X-IG-WWW-Claim"] = "0"
	}

	if c.IgURur != "" {
		headers["IG-U-RUR"] = c.IgURur
	}

	return headers
}

// privateRequest makes an authenticated request to Instagram's private API
func (c *Client) privateRequest(endpoint string, data map[string]any, login bool) (*APIResponse, error) {
	urlStr := IGAPIBaseURL + endpoint

	// Prepare form data
	formData := url.Values{}
	for key, value := range data {
		switch v := value.(type) {
		case string:
			formData.Set(key, v)
		case int:
			formData.Set(key, strconv.Itoa(v))
		case int64:
			formData.Set(key, strconv.FormatInt(v, 10))
		case bool:
			if v {
				formData.Set(key, "1")
			} else {
				formData.Set(key, "0")
			}
		default:
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal data: %w", err)
			}
			formData.Set(key, string(jsonBytes))
		}
	}

	req, err := http.NewRequest("POST", urlStr, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	headers := c.baseHeaders()
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Add authorization header if not login request and we have auth data
	if !login && len(c.AuthorizationData) > 0 {
		req.Header.Set("Authorization", c.getAuthorizationHeader())
	}

	// Add CSRF token
	req.Header.Set("X-CSRFToken", c.CSRFToken())

	// Make request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	var bodyReader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		bodyReader = gzReader
	}

	body, err := io.ReadAll(bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Update cookies
	c.updateCookies(resp.Cookies())

	// Update headers from response
	c.updateFromResponseHeaders(resp.Header)

	// Store raw body for debugging
	apiResp := &APIResponse{
		RawBody: body,
	}

	// Parse response
	if err := json.Unmarshal(body, apiResp); err != nil {
		// Keep raw body if JSON parsing fails
		if c.Debug {
			fmt.Printf("[DEBUG] Failed to parse response: %s\n", string(body))
		}
	}

	if c.Debug {
		fmt.Printf("[DEBUG] Response Status: %d\n", resp.StatusCode)
		fmt.Printf("[DEBUG] Response Body: %s\n", string(body))
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return apiResp, c.handleAPIError(resp.StatusCode, apiResp)
	}

	if apiResp.Status == "fail" {
		return apiResp, c.handleAPIError(resp.StatusCode, apiResp)
	}

	return apiResp, nil
}

// privateRequestGET makes a GET request with query parameters to Instagram's private API
func (c *Client) privateRequestGET(endpoint string, params map[string]string) (*APIResponse, error) {
	urlStr := IGAPIBaseURL + endpoint

	// Build query string
	queryParams := url.Values{}
	for key, value := range params {
		queryParams.Set(key, value)
	}

	if len(queryParams) > 0 {
		urlStr += "?" + queryParams.Encode()
	}

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	headers := c.baseHeaders()
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Remove Content-Type for GET requests
	req.Header.Del("Content-Type")

	// Add authorization header
	if len(c.AuthorizationData) > 0 {
		req.Header.Set("Authorization", c.getAuthorizationHeader())
	}

	// Add CSRF token
	req.Header.Set("X-CSRFToken", c.CSRFToken())

	// Make request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	var bodyReader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		bodyReader = gzReader
	}

	body, err := io.ReadAll(bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Update cookies
	c.updateCookies(resp.Cookies())

	// Update headers from response
	c.updateFromResponseHeaders(resp.Header)

	// Store raw body for debugging
	apiResp := &APIResponse{
		RawBody: body,
	}

	// Parse response
	if err := json.Unmarshal(body, apiResp); err != nil {
		if c.Debug {
			fmt.Printf("[DEBUG] Failed to parse response: %s\n", string(body))
		}
	}

	if c.Debug {
		fmt.Printf("[DEBUG] GET Response Status: %d\n", resp.StatusCode)
		fmt.Printf("[DEBUG] GET Response Body: %s\n", string(body))
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return apiResp, c.handleAPIError(resp.StatusCode, apiResp)
	}

	if apiResp.Status == "fail" {
		return apiResp, c.handleAPIError(resp.StatusCode, apiResp)
	}

	return apiResp, nil
}

// privateRequestJSON makes a request with JSON body
func (c *Client) privateRequestJSON(endpoint string, data map[string]any, login bool) (*APIResponse, error) {
	urlStr := IGAPIBaseURL + endpoint

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	req, err := http.NewRequest("POST", urlStr, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	headers := c.baseHeaders()
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	if !login && len(c.AuthorizationData) > 0 {
		req.Header.Set("Authorization", c.getAuthorizationHeader())
	}

	req.Header.Set("X-CSRFToken", c.CSRFToken())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var bodyReader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		bodyReader = gzReader
	}

	body, err := io.ReadAll(bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	c.updateCookies(resp.Cookies())
	c.updateFromResponseHeaders(resp.Header)

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		apiResp.RawBody = body
	}

	if resp.StatusCode != http.StatusOK || apiResp.Status == "fail" {
		return &apiResp, c.handleAPIError(resp.StatusCode, &apiResp)
	}

	return &apiResp, nil
}

// getAuthorizationHeader builds the authorization header
func (c *Client) getAuthorizationHeader() string {
	if len(c.AuthorizationData) == 0 {
		return ""
	}

	jsonData, err := json.Marshal(c.AuthorizationData)
	if err != nil {
		return ""
	}

	encoded := base64Encode(jsonData)
	return fmt.Sprintf("Bearer IGT:2:%s", encoded)
}

// parseAuthorization parses the authorization header from response
func (c *Client) parseAuthorization(auth string) map[string]any {
	if auth == "" {
		return nil
	}

	// Extract base64 part after the last colon
	parts := strings.Split(auth, ":")
	if len(parts) < 2 {
		return nil
	}

	b64Part := parts[len(parts)-1]
	decoded, err := base64Decode(b64Part)
	if err != nil {
		return nil
	}

	var result map[string]any
	if err := json.Unmarshal(decoded, &result); err != nil {
		return nil
	}

	return result
}

// updateCookies updates stored cookies from response
func (c *Client) updateCookies(cookies []*http.Cookie) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, cookie := range cookies {
		c.Cookies[cookie.Name] = cookie.Value

		// Update specific fields
		switch cookie.Name {
		case "csrftoken":
			c.csrfToken = cookie.Value
		case "mid":
			c.Mid = cookie.Value
		case "sessionid":
			c.SessionID = cookie.Value
		}
	}
}

// updateFromResponseHeaders updates client state from response headers
func (c *Client) updateFromResponseHeaders(headers http.Header) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if auth := headers.Get("ig-set-authorization"); auth != "" {
		c.AuthorizationData = c.parseAuthorization(auth)
	}

	if rur := headers.Get("ig-set-ig-u-rur"); rur != "" {
		c.IgURur = rur
	}

	if claim := headers.Get("x-ig-set-www-claim"); claim != "" {
		c.IgWwwClaim = claim
	}
}

// handleAPIError converts API error responses to proper errors
func (c *Client) handleAPIError(statusCode int, resp *APIResponse) error {
	err := &APIError{
		StatusCode: statusCode,
		Message:    resp.Message,
		ErrorType:  resp.ErrorType,
		Response:   resp,
	}

	switch resp.ErrorType {
	case "two_factor_required":
		return ErrTwoFactorRequired
	case "challenge_required":
		return ErrChallengeRequired
	case "checkpoint_challenge_required":
		return ErrCheckpointRequired
	case "bad_password":
		return ErrBadCredentials
	case "invalid_user":
		return ErrBadCredentials
	}

	if statusCode == 429 {
		return ErrRateLimited
	}

	return err
}

// base64Encode encodes bytes to base64
func base64Encode(data []byte) string {
	const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

	var result strings.Builder
	for i := 0; i < len(data); i += 3 {
		var chunk int
		var padding int

		chunk = int(data[i]) << 16
		if i+1 < len(data) {
			chunk |= int(data[i+1]) << 8
			if i+2 < len(data) {
				chunk |= int(data[i+2])
			} else {
				padding = 1
			}
		} else {
			padding = 2
		}

		result.WriteByte(base64Chars[(chunk>>18)&0x3F])
		result.WriteByte(base64Chars[(chunk>>12)&0x3F])
		if padding < 2 {
			result.WriteByte(base64Chars[(chunk>>6)&0x3F])
		} else {
			result.WriteByte('=')
		}
		if padding < 1 {
			result.WriteByte(base64Chars[chunk&0x3F])
		} else {
			result.WriteByte('=')
		}
	}

	return result.String()
}

// base64Decode decodes base64 string to bytes
func base64Decode(s string) ([]byte, error) {
	const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

	// Build reverse lookup
	lookup := make(map[byte]int)
	for i, c := range []byte(base64Chars) {
		lookup[c] = i
	}

	// Remove padding
	s = strings.TrimRight(s, "=")

	var result []byte
	for i := 0; i < len(s); i += 4 {
		var chunk int
		for j := 0; j < 4 && i+j < len(s); j++ {
			chunk = chunk<<6 | lookup[s[i+j]]
		}

		// Determine how many bytes to extract
		remaining := len(s) - i
		if remaining >= 4 {
			result = append(result, byte(chunk>>16), byte(chunk>>8), byte(chunk))
		} else if remaining == 3 {
			result = append(result, byte(chunk>>10), byte(chunk>>2))
		} else if remaining == 2 {
			result = append(result, byte(chunk>>4))
		}
	}

	return result, nil
}

// withDefaultData adds default data to request
func (c *Client) withDefaultData(data map[string]any) map[string]any {
	result := map[string]any{
		"_uuid":     c.UUID,
		"device_id": c.AndroidDeviceID,
	}
	for k, v := range data {
		result[k] = v
	}
	return result
}

// withExtraData adds extra data including user ID
func (c *Client) withExtraData(data map[string]any) map[string]any {
	result := c.withDefaultData(map[string]any{
		"phone_id": c.PhoneID,
		"_uid":     strconv.FormatInt(c.UserID(), 10),
		"guid":     c.UUID,
	})
	for k, v := range data {
		result[k] = v
	}
	return result
}

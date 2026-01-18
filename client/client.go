package client

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Instagram API constants
const (
	IGAPIBaseURL     = "https://i.instagram.com/api/v1/"
	IGWebBaseURL     = "https://www.instagram.com/"
	IGBloksVersionID = "ce555e5500576acd8e84a66018f54a05720f2dce29f0bb5a1f97f0c10d6fac48"
	IGAppID          = "567067343352427"
)

// Client represents the Instagram API client
type Client struct {
	mu sync.RWMutex

	// Credentials
	Username string `json:"username"`
	Password string `json:"password"`

	// Session data
	SessionID         string            `json:"session_id,omitempty"`
	AuthorizationData map[string]any    `json:"authorization_data,omitempty"`
	LastLogin         int64             `json:"last_login,omitempty"`
	Cookies           map[string]string `json:"cookies,omitempty"`

	// Device settings
	DeviceSettings *DeviceSettings `json:"device_settings"`
	UserAgent      string          `json:"user_agent"`

	// UUIDs
	PhoneID           string `json:"phone_id"`
	UUID              string `json:"uuid"`
	ClientSessionID   string `json:"client_session_id"`
	AdvertisingID     string `json:"advertising_id"`
	AndroidDeviceID   string `json:"android_device_id"`
	RequestID         string `json:"request_id"`
	TraySessionID     string `json:"tray_session_id"`
	BloksVersioningID string `json:"bloks_versioning_id"`

	// Locale settings
	Country        string `json:"country"`
	CountryCode    int    `json:"country_code"`
	Locale         string `json:"locale"`
	TimezoneOffset int    `json:"timezone_offset"`

	// Headers
	Mid        string `json:"mid,omitempty"`
	IgURur     string `json:"ig_u_rur,omitempty"`
	IgWwwClaim string `json:"ig_www_claim,omitempty"`

	// HTTP client
	httpClient *http.Client
	csrfToken  string

	// Relogin tracking
	ReloginAttempt int `json:"-"`

	// Debug mode
	Debug bool `json:"-"`
}

// DeviceSettings represents device configuration
type DeviceSettings struct {
	AppVersion     string `json:"app_version"`
	AndroidVersion int    `json:"android_version"`
	AndroidRelease string `json:"android_release"`
	DPI            string `json:"dpi"`
	Resolution     string `json:"resolution"`
	Manufacturer   string `json:"manufacturer"`
	Device         string `json:"device"`
	Model          string `json:"model"`
	CPU            string `json:"cpu"`
	VersionCode    string `json:"version_code"`
}

// NewClient creates a new Instagram client with default settings
func NewClient() *Client {
	jar, _ := cookiejar.New(nil)

	c := &Client{
		DeviceSettings:    getDefaultDeviceSettings(),
		Country:           "US",
		CountryCode:       1,
		Locale:            "en_US",
		TimezoneOffset:    -14400, // GMT-4 (New York)
		BloksVersioningID: IGBloksVersionID,
		AuthorizationData: make(map[string]any),
		Cookies:           make(map[string]string),
		httpClient: &http.Client{
			Jar:     jar,
			Timeout: 30 * time.Second,
		},
	}

	c.initUUIDs()
	c.setUserAgent()

	return c
}

// NewClientWithCredentials creates a new client with username and password
func NewClientWithCredentials(username, password string) *Client {
	c := NewClient()
	c.Username = username
	c.Password = password
	return c
}

// initUUIDs generates all required UUIDs
func (c *Client) initUUIDs() {
	c.PhoneID = c.generateUUID()
	c.UUID = c.generateUUID()
	c.ClientSessionID = c.generateUUID()
	c.AdvertisingID = c.generateUUID()
	c.AndroidDeviceID = c.generateAndroidDeviceID()
	c.RequestID = c.generateUUID()
	c.TraySessionID = c.generateUUID()
}

// generateUUID generates a random UUID v4
func (c *Client) generateUUID() string {
	return uuid.New().String()
}

// generateAndroidDeviceID generates Android device ID format
func (c *Client) generateAndroidDeviceID() string {
	timestamp := strconv.FormatInt(time.Now().UnixNano(), 10)
	hash := sha256.Sum256([]byte(timestamp))
	return "android-" + hex.EncodeToString(hash[:])[:16]
}

// setUserAgent sets the user agent based on device settings
func (c *Client) setUserAgent() {
	c.UserAgent = fmt.Sprintf(
		"Instagram %s Android (%d/%s; %s; %s; %s; %s; %s; %s; %s)",
		c.DeviceSettings.AppVersion,
		c.DeviceSettings.AndroidVersion,
		c.DeviceSettings.AndroidRelease,
		c.DeviceSettings.DPI,
		c.DeviceSettings.Resolution,
		c.DeviceSettings.Manufacturer,
		c.DeviceSettings.Device,
		c.DeviceSettings.Model,
		c.DeviceSettings.CPU,
		c.Locale,
	)
}

// getDefaultDeviceSettings returns default device configuration
func getDefaultDeviceSettings() *DeviceSettings {
	return &DeviceSettings{
		AppVersion:     "269.0.0.18.75",
		AndroidVersion: 26,
		AndroidRelease: "8.0.0",
		DPI:            "480dpi",
		Resolution:     "1080x1920",
		Manufacturer:   "OnePlus",
		Device:         "devitron",
		Model:          "6T Dev",
		CPU:            "qcom",
		VersionCode:    "314665256",
	}
}

// UserID returns the user ID from cookies or authorization data
func (c *Client) UserID() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if userID, ok := c.Cookies["ds_user_id"]; ok {
		if id, err := strconv.ParseInt(userID, 10, 64); err == nil {
			return id
		}
	}

	if c.AuthorizationData != nil {
		if userID, ok := c.AuthorizationData["ds_user_id"]; ok {
			switch v := userID.(type) {
			case string:
				if id, err := strconv.ParseInt(v, 10, 64); err == nil {
					return id
				}
			case float64:
				return int64(v)
			case int64:
				return v
			}
		}
	}

	return 0
}

// GetSessionID returns the current session ID
func (c *Client) GetSessionID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.SessionID != "" {
		return c.SessionID
	}

	if sid, ok := c.Cookies["sessionid"]; ok {
		return sid
	}

	if c.AuthorizationData != nil {
		if sid, ok := c.AuthorizationData["sessionid"].(string); ok {
			return sid
		}
	}

	return ""
}

// CSRFToken returns or generates a CSRF token
func (c *Client) CSRFToken() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.csrfToken != "" {
		return c.csrfToken
	}

	if token, ok := c.Cookies["csrftoken"]; ok {
		c.csrfToken = token
		return token
	}

	// Generate a random token
	c.csrfToken = c.generateRandomToken(64)
	return c.csrfToken
}

// generateRandomToken generates a random hex token
func (c *Client) generateRandomToken(length int) string {
	bytes := make([]byte, length/2)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// RankToken returns the rank token for API requests
func (c *Client) RankToken() string {
	return fmt.Sprintf("%d_%s", c.UserID(), c.UUID)
}

// IsLoggedIn checks if the client has a valid session
func (c *Client) IsLoggedIn() bool {
	return c.UserID() != 0 && c.GetSessionID() != ""
}

// IsSessionValid checks if the current session is still valid
func (c *Client) IsSessionValid() bool {
	if !c.IsLoggedIn() {
		return false
	}

	// Check if last login was within 24 hours
	if c.LastLogin > 0 {
		elapsed := time.Now().Unix() - c.LastLogin
		if elapsed < 24*60*60 { // 24 hours
			return true
		}
	}

	return false
}

// GetSettings returns current session settings for storage
func (c *Client) GetSettings() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]any{
		"uuids": map[string]string{
			"phone_id":          c.PhoneID,
			"uuid":              c.UUID,
			"client_session_id": c.ClientSessionID,
			"advertising_id":    c.AdvertisingID,
			"android_device_id": c.AndroidDeviceID,
			"request_id":        c.RequestID,
			"tray_session_id":   c.TraySessionID,
		},
		"mid":                c.Mid,
		"ig_u_rur":           c.IgURur,
		"ig_www_claim":       c.IgWwwClaim,
		"authorization_data": c.AuthorizationData,
		"cookies":            c.Cookies,
		"last_login":         c.LastLogin,
		"device_settings":    c.DeviceSettings,
		"user_agent":         c.UserAgent,
		"country":            c.Country,
		"country_code":       c.CountryCode,
		"locale":             c.Locale,
		"timezone_offset":    c.TimezoneOffset,
		"username":           c.Username,
	}
}

// SetSettings restores session settings from storage
func (c *Client) SetSettings(settings map[string]any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if uuids, ok := settings["uuids"].(map[string]any); ok {
		if v, ok := uuids["phone_id"].(string); ok {
			c.PhoneID = v
		}
		if v, ok := uuids["uuid"].(string); ok {
			c.UUID = v
		}
		if v, ok := uuids["client_session_id"].(string); ok {
			c.ClientSessionID = v
		}
		if v, ok := uuids["advertising_id"].(string); ok {
			c.AdvertisingID = v
		}
		if v, ok := uuids["android_device_id"].(string); ok {
			c.AndroidDeviceID = v
		}
		if v, ok := uuids["request_id"].(string); ok {
			c.RequestID = v
		}
		if v, ok := uuids["tray_session_id"].(string); ok {
			c.TraySessionID = v
		}
	}

	if v, ok := settings["mid"].(string); ok {
		c.Mid = v
	}
	if v, ok := settings["ig_u_rur"].(string); ok {
		c.IgURur = v
	}
	if v, ok := settings["ig_www_claim"].(string); ok {
		c.IgWwwClaim = v
	}
	if v, ok := settings["authorization_data"].(map[string]any); ok {
		c.AuthorizationData = v
	}
	if v, ok := settings["cookies"].(map[string]any); ok {
		c.Cookies = make(map[string]string)
		for key, val := range v {
			if strVal, ok := val.(string); ok {
				c.Cookies[key] = strVal
			}
		}
	}
	if v, ok := settings["last_login"].(float64); ok {
		c.LastLogin = int64(v)
	}
	if v, ok := settings["user_agent"].(string); ok {
		c.UserAgent = v
	}
	if v, ok := settings["country"].(string); ok {
		c.Country = v
	}
	if v, ok := settings["country_code"].(float64); ok {
		c.CountryCode = int(v)
	}
	if v, ok := settings["locale"].(string); ok {
		c.Locale = v
	}
	if v, ok := settings["timezone_offset"].(float64); ok {
		c.TimezoneOffset = int(v)
	}
	if v, ok := settings["username"].(string); ok {
		c.Username = v
	}

	// Restore device settings
	if ds, ok := settings["device_settings"].(map[string]any); ok {
		c.DeviceSettings = &DeviceSettings{}
		if v, ok := ds["app_version"].(string); ok {
			c.DeviceSettings.AppVersion = v
		}
		if v, ok := ds["android_version"].(float64); ok {
			c.DeviceSettings.AndroidVersion = int(v)
		}
		if v, ok := ds["android_release"].(string); ok {
			c.DeviceSettings.AndroidRelease = v
		}
		if v, ok := ds["dpi"].(string); ok {
			c.DeviceSettings.DPI = v
		}
		if v, ok := ds["resolution"].(string); ok {
			c.DeviceSettings.Resolution = v
		}
		if v, ok := ds["manufacturer"].(string); ok {
			c.DeviceSettings.Manufacturer = v
		}
		if v, ok := ds["device"].(string); ok {
			c.DeviceSettings.Device = v
		}
		if v, ok := ds["model"].(string); ok {
			c.DeviceSettings.Model = v
		}
		if v, ok := ds["cpu"].(string); ok {
			c.DeviceSettings.CPU = v
		}
		if v, ok := ds["version_code"].(string); ok {
			c.DeviceSettings.VersionCode = v
		}
	}

	// Restore cookies to HTTP client
	if len(c.Cookies) > 0 {
		c.restoreCookies()
	}

	return nil
}

// restoreCookies restores cookies to the HTTP client
func (c *Client) restoreCookies() {
	u, _ := url.Parse(IGAPIBaseURL)
	var cookies []*http.Cookie

	for name, value := range c.Cookies {
		cookies = append(cookies, &http.Cookie{
			Name:   name,
			Value:  value,
			Domain: ".instagram.com",
			Path:   "/",
		})
	}

	c.httpClient.Jar.SetCookies(u, cookies)

	// Also set for web URL
	webURL, _ := url.Parse(IGWebBaseURL)
	c.httpClient.Jar.SetCookies(webURL, cookies)
}

// ToJSON serializes client settings to JSON
func (c *Client) ToJSON() ([]byte, error) {
	return json.Marshal(c.GetSettings())
}

// FromJSON deserializes client settings from JSON
func (c *Client) FromJSON(data []byte) error {
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return err
	}
	return c.SetSettings(settings)
}

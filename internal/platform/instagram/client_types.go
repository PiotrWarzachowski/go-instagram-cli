package instagram

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/PiotrWarzachowski/go-instagram-cli/internal/platform/instagram/session"
)

const (
	IGAPIBaseURL     = "https://i.instagram.com/api/v1/"
	IGWebBaseURL     = "https://www.instagram.com/"
	IGBloksVersionID = "ce555e5500576acd8e84a66018f54a05720f2dce29f0bb5a1f97f0c10d6fac48"
	IGAppID          = "567067343352427"
)

type Client struct {
	mu sync.RWMutex

	Username string `json:"username"`
	Password string `json:"password"`

	SessionID         string            `json:"session_id,omitempty"`
	AuthorizationData map[string]any    `json:"authorization_data,omitempty"`
	LastLogin         int64             `json:"last_login,omitempty"`
	Cookies           map[string]string `json:"cookies,omitempty"`

	DeviceSettings *session.DeviceSettings `json:"device_settings"`
	UserAgent      string                  `json:"user_agent"`

	PhoneID           string `json:"phone_id"`
	UUID              string `json:"uuid"`
	ClientSessionID   string `json:"client_session_id"`
	AdvertisingID     string `json:"advertising_id"`
	AndroidDeviceID   string `json:"android_device_id"`
	RequestID         string `json:"request_id"`
	TraySessionID     string `json:"tray_session_id"`
	BloksVersioningID string `json:"bloks_versioning_id"`

	Country        string `json:"country"`
	CountryCode    int    `json:"country_code"`
	Locale         string `json:"locale"`
	TimezoneOffset int    `json:"timezone_offset"`

	Mid        string `json:"mid,omitempty"`
	IgURur     string `json:"ig_u_rur,omitempty"`
	IgWwwClaim string `json:"ig_www_claim,omitempty"`

	httpClient *http.Client
	csrfToken  string

	ReloginAttempt int `json:"-"`

	Debug bool `json:"-"`
}

type APIResponse struct {
	Status    string         `json:"status"`
	Message   string         `json:"message,omitempty"`
	ErrorType string         `json:"error_type,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	RawBody   []byte         `json:"-"`
}

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

var (
	ErrBadCredentials         = &APIError{Message: "Invalid username or password"}
	ErrTwoFactorRequired      = &APIError{Message: "Two factor authentication required", ErrorType: "two_factor_required"}
	ErrChallengeRequired      = &APIError{Message: "Challenge required", ErrorType: "challenge_required"}
	ErrCheckpointRequired     = &APIError{Message: "Checkpoint required", ErrorType: "checkpoint_challenge_required"}
	ErrRateLimited            = &APIError{Message: "Rate limited, please wait", ErrorType: "rate_limit"}
	ErrReloginAttemptExceeded = &APIError{Message: "Relogin attempt exceeded"}
)

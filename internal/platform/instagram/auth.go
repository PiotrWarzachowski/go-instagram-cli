package instagram

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func (c *Client) Login(username, password string, verificationCode string) (*LoginResult, error) {
	c.Username = username
	c.Password = password

	// Check if already logged in
	if c.UserID() != 0 && c.GetSessionID() != "" {
		return &LoginResult{
			Success:  true,
			UserID:   c.UserID(),
			Username: c.Username,
		}, nil
	}

	if err := c.fetchInitialCookies(); err != nil {
		return nil, fmt.Errorf("failed to get initial cookies: %w", err)
	}

	result, err := c.webLogin(username, password)
	if err != nil {
		if result != nil && result.TwoFactorRequired {
			if verificationCode != "" {
				return c.webTwoFactorLogin(username, verificationCode, result.TwoFactorInfo)
			}
			return result, ErrTwoFactorRequired
		}
		return result, err
	}

	if result.Success {
		c.LastLogin = time.Now().Unix()
	}

	return result, nil
}

// fetchInitialCookies gets CSRF token and initial cookies from Instagram
func (c *Client) fetchInitialCookies() error {
	req, err := http.NewRequest("GET", "https://www.instagram.com/accounts/login/", nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", c.getWebUserAgent())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read body to complete the request
	io.Copy(io.Discard, resp.Body)

	// Extract CSRF token from cookies
	u, _ := url.Parse("https://www.instagram.com/")
	for _, cookie := range c.httpClient.Jar.Cookies(u) {
		if cookie.Name == "csrftoken" {
			c.csrfToken = cookie.Value
			c.Cookies["csrftoken"] = cookie.Value
		}
		if cookie.Name == "mid" {
			c.Mid = cookie.Value
			c.Cookies["mid"] = cookie.Value
		}
	}

	if c.csrfToken == "" {
		return errors.New("failed to get CSRF token")
	}

	if c.Debug {
		fmt.Printf("[DEBUG] Got CSRF token: %s\n", c.csrfToken[:20]+"...")
	}

	return nil
}

// webLogin performs the actual web login
func (c *Client) webLogin(username, password string) (*LoginResult, error) {
	// Build enc_password with version 0 (plaintext with timestamp)
	timestamp := time.Now().Unix()
	encPassword := fmt.Sprintf("#PWD_INSTAGRAM_BROWSER:0:%d:%s", timestamp, password)

	// Build form data
	formData := url.Values{}
	formData.Set("username", username)
	formData.Set("enc_password", encPassword)
	formData.Set("queryParams", "{}")
	formData.Set("optIntoOneTap", "false")

	req, err := http.NewRequest("POST", "https://www.instagram.com/accounts/login/ajax/", strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, err
	}

	// Set headers exactly like a browser
	req.Header.Set("User-Agent", c.getWebUserAgent())
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-CSRFToken", c.csrfToken)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("X-IG-App-ID", "936619743392459") // Web app ID
	req.Header.Set("X-ASBD-ID", "198387")
	req.Header.Set("X-IG-WWW-Claim", "0")
	req.Header.Set("Origin", "https://www.instagram.com")
	req.Header.Set("Referer", "https://www.instagram.com/accounts/login/")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")

	if c.Debug {
		fmt.Printf("[DEBUG] Login request to: %s\n", req.URL.String())
		fmt.Printf("[DEBUG] CSRF Token: %s\n", c.csrfToken[:20]+"...")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if c.Debug {
		fmt.Printf("[DEBUG] Response status: %d\n", resp.StatusCode)
		fmt.Printf("[DEBUG] Response body: %s\n", string(body))
	}

	// Update cookies from response
	u, _ := url.Parse("https://www.instagram.com/")
	for _, cookie := range c.httpClient.Jar.Cookies(u) {
		c.Cookies[cookie.Name] = cookie.Value
		if cookie.Name == "sessionid" {
			c.SessionID = cookie.Value
		}
		if cookie.Name == "ds_user_id" {
			c.Cookies["ds_user_id"] = cookie.Value
		}
	}

	// Parse response
	var loginResp WebLoginResponse
	if err := json.Unmarshal(body, &loginResp); err != nil {
		return nil, fmt.Errorf("failed to parse login response: %w (body: %s)", err, string(body))
	}

	// Check for 2FA
	if loginResp.TwoFactorRequired {
		return &LoginResult{
			TwoFactorRequired: true,
			TwoFactorInfo: map[string]any{
				"two_factor_identifier": loginResp.TwoFactorInfo.TwoFactorIdentifier,
				"username":              loginResp.TwoFactorInfo.Username,
			},
		}, ErrTwoFactorRequired
	}

	// Check for challenge/checkpoint
	if loginResp.Checkpoint.URL != "" || loginResp.ErrorType == "checkpoint_required" {
		return &LoginResult{
			ChallengeRequired: true,
			ChallengeInfo: map[string]any{
				"url": loginResp.Checkpoint.URL,
			},
		}, ErrChallengeRequired
	}

	if loginResp.Authenticated {
		userID, _ := strconv.ParseInt(loginResp.UserID, 10, 64)
		c.Cookies["ds_user_id"] = loginResp.UserID

		return &LoginResult{
			Success:  true,
			UserID:   userID,
			Username: username,
		}, nil
	}

	errMsg := loginResp.Message
	if errMsg == "" {
		errMsg = "Invalid username or password"
	}

	return &LoginResult{
		Error: &APIError{Message: errMsg, ErrorType: loginResp.ErrorType},
	}, &APIError{Message: errMsg, ErrorType: loginResp.ErrorType}
}

func (c *Client) webTwoFactorLogin(username, verificationCode string, twoFactorInfo map[string]any) (*LoginResult, error) {
	identifier := ""
	if twoFactorInfo != nil {
		if id, ok := twoFactorInfo["two_factor_identifier"].(string); ok {
			identifier = id
		}
	}

	formData := url.Values{}
	formData.Set("username", username)
	formData.Set("verificationCode", verificationCode)
	formData.Set("identifier", identifier)
	formData.Set("queryParams", "{}")

	req, err := http.NewRequest("POST", "https://www.instagram.com/accounts/login/ajax/two_factor/", strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", c.getWebUserAgent())
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-CSRFToken", c.csrfToken)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("X-IG-App-ID", "936619743392459")
	req.Header.Set("Origin", "https://www.instagram.com")
	req.Header.Set("Referer", "https://www.instagram.com/accounts/login/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if c.Debug {
		fmt.Printf("[DEBUG] 2FA Response: %s\n", string(body))
	}

	u, _ := url.Parse("https://www.instagram.com/")
	for _, cookie := range c.httpClient.Jar.Cookies(u) {
		c.Cookies[cookie.Name] = cookie.Value
		if cookie.Name == "sessionid" {
			c.SessionID = cookie.Value
		}
	}

	var loginResp WebLoginResponse
	if err := json.Unmarshal(body, &loginResp); err != nil {
		return nil, fmt.Errorf("failed to parse 2FA response: %w", err)
	}

	if loginResp.Authenticated {
		userID, _ := strconv.ParseInt(loginResp.UserID, 10, 64)
		c.Cookies["ds_user_id"] = loginResp.UserID
		c.LastLogin = time.Now().Unix()

		return &LoginResult{
			Success:  true,
			UserID:   userID,
			Username: username,
		}, nil
	}

	return nil, &APIError{Message: loginResp.Message, ErrorType: loginResp.ErrorType}
}

func (c *Client) LoginBySessionID(sessionID string) (*LoginResult, error) {
	if len(sessionID) < 30 {
		return nil, errors.New("invalid session ID")
	}

	c.Cookies["sessionid"] = sessionID
	c.SessionID = sessionID

	re := regexp.MustCompile(`^\d+`)
	match := re.FindString(sessionID)
	if match != "" {
		c.Cookies["ds_user_id"] = match
		c.AuthorizationData["ds_user_id"] = match
		c.AuthorizationData["sessionid"] = sessionID
		c.AuthorizationData["should_use_header_over_cookies"] = true
	}

	c.restoreCookies()
	c.LastLogin = time.Now().Unix()

	return &LoginResult{
		Success:  true,
		UserID:   c.UserID(),
		Username: c.Username,
	}, nil
}

func (c *Client) Relogin() (*LoginResult, error) {
	if c.ReloginAttempt > 1 {
		return nil, ErrReloginAttemptExceeded
	}
	c.ReloginAttempt++

	// Clear existing auth
	c.AuthorizationData = make(map[string]any)
	c.Cookies = make(map[string]string)
	c.SessionID = ""
	c.csrfToken = ""

	return c.Login(c.Username, c.Password, "")
}

func (c *Client) Logout() error {
	formData := url.Values{}
	formData.Set("one_tap_app_login", "true")

	req, err := http.NewRequest("POST", "https://www.instagram.com/accounts/logout/ajax/", strings.NewReader(formData.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", c.getWebUserAgent())
	req.Header.Set("X-CSRFToken", c.csrfToken)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Clear session data
	c.AuthorizationData = make(map[string]any)
	c.Cookies = make(map[string]string)
	c.SessionID = ""
	c.LastLogin = 0
	c.csrfToken = ""

	return nil
}

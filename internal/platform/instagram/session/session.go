package session

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

type Session struct {
	Username          string            `json:"username"`
	PasswordHash      string            `json:"password_hash"`
	SessionData       map[string]any    `json:"session_data"`
	AuthorizationData map[string]any    `json:"authorization_data"`
	Cookies           map[string]string `json:"cookies"`
	LastLogin         int64             `json:"last_login"`
	DeviceSettings    *DeviceSettings   `json:"device_settings"`
	UUIDs             map[string]string `json:"uuids"`
}

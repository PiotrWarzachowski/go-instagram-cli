package client

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strconv"
	"time"
)

// Common device manufacturers and models for fingerprinting
var deviceDatabase = []DeviceProfile{
	{
		Manufacturer:   "OnePlus",
		Device:         "devitron",
		Model:          "6T Dev",
		AndroidVersion: 26,
		AndroidRelease: "8.0.0",
		DPI:            "480dpi",
		Resolution:     "1080x1920",
		CPU:            "qcom",
	},
	{
		Manufacturer:   "samsung",
		Device:         "beyond1",
		Model:          "SM-G973F",
		AndroidVersion: 29,
		AndroidRelease: "10.0",
		DPI:            "560dpi",
		Resolution:     "1440x3040",
		CPU:            "exynos9820",
	},
	{
		Manufacturer:   "Google",
		Device:         "oriole",
		Model:          "Pixel 6",
		AndroidVersion: 31,
		AndroidRelease: "12.0",
		DPI:            "420dpi",
		Resolution:     "1080x2400",
		CPU:            "arm64-v8a",
	},
	{
		Manufacturer:   "Xiaomi",
		Device:         "cmi",
		Model:          "Mi 10 Pro",
		AndroidVersion: 30,
		AndroidRelease: "11.0",
		DPI:            "440dpi",
		Resolution:     "1080x2340",
		CPU:            "qcom",
	},
	{
		Manufacturer:   "HUAWEI",
		Device:         "ELS",
		Model:          "P40 Pro",
		AndroidVersion: 30,
		AndroidRelease: "11.0",
		DPI:            "480dpi",
		Resolution:     "1200x2640",
		CPU:            "kirin990",
	},
	{
		Manufacturer:   "OnePlus",
		Device:         "lemonadep",
		Model:          "LE2123",
		AndroidVersion: 31,
		AndroidRelease: "12.0",
		DPI:            "450dpi",
		Resolution:     "1440x3216",
		CPU:            "qcom",
	},
	{
		Manufacturer:   "samsung",
		Device:         "o1s",
		Model:          "SM-G991B",
		AndroidVersion: 31,
		AndroidRelease: "12.0",
		DPI:            "480dpi",
		Resolution:     "1080x2400",
		CPU:            "exynos2100",
	},
}

// DeviceProfile represents a device fingerprint profile
type DeviceProfile struct {
	Manufacturer   string
	Device         string
	Model          string
	AndroidVersion int
	AndroidRelease string
	DPI            string
	Resolution     string
	CPU            string
}

// GenerateDeviceFingerprint creates a realistic device fingerprint
func GenerateDeviceFingerprint() *DeviceSettings {
	// Select random device from database
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	profile := deviceDatabase[r.Intn(len(deviceDatabase))]

	return &DeviceSettings{
		AppVersion:     getLatestAppVersion(),
		AndroidVersion: profile.AndroidVersion,
		AndroidRelease: profile.AndroidRelease,
		DPI:            profile.DPI,
		Resolution:     profile.Resolution,
		Manufacturer:   profile.Manufacturer,
		Device:         profile.Device,
		Model:          profile.Model,
		CPU:            profile.CPU,
		VersionCode:    "314665256",
	}
}

// getLatestAppVersion returns a recent Instagram app version
func getLatestAppVersion() string {
	versions := []string{
		"269.0.0.18.75",
		"270.0.0.14.83",
		"271.0.0.21.84",
		"272.0.0.17.84",
		"273.0.0.16.70",
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return versions[r.Intn(len(versions))]
}

// GenerateJazoest generates the jazoest parameter from phone_id
// This is used in Instagram API requests
func GenerateJazoest(phoneID string) string {
	var sum int
	for _, c := range phoneID {
		sum += int(c)
	}
	return "2" + strconv.Itoa(sum)
}

// GenerateDeviceID generates a unique device ID based on timestamp
func GenerateDeviceID() string {
	timestamp := strconv.FormatInt(time.Now().UnixNano(), 10)
	hash := sha256.Sum256([]byte(timestamp))
	return "android-" + hex.EncodeToString(hash[:])[:16]
}

// GenerateMutationToken generates a token for DM sending and media upload
func GenerateMutationToken() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return strconv.FormatInt(6800011111111111111+r.Int63n(88888888888888888), 10)
}

// BuildUserAgent constructs a user agent string from device settings
func BuildUserAgent(ds *DeviceSettings, locale string) string {
	return fmt.Sprintf(
		"Instagram %s Android (%d/%s; %s; %s; %s; %s; %s; %s; %s)",
		ds.AppVersion,
		ds.AndroidVersion,
		ds.AndroidRelease,
		ds.DPI,
		ds.Resolution,
		ds.Manufacturer,
		ds.Device,
		ds.Model,
		ds.CPU,
		locale,
	)
}

// SupportedCapabilities returns the supported capabilities for API requests
var SupportedCapabilities = []map[string]any{
	{"name": "SUPPORTED_SDK_VERSIONS", "value": "131.0,132.0,133.0,134.0,135.0,136.0,137.0,138.0,139.0,140.0,141.0,142.0,143.0,144.0,145.0,146.0,147.0"},
	{"name": "FACE_TRACKER_VERSION", "value": "14"},
	{"name": "COMPRESSION", "value": "ETC2_COMPRESSION"},
	{"name": "world_tracker", "value": "world_tracker_enabled"},
	{"name": "gyroscope", "value": "gyroscope_enabled"},
}

// RandomizeTiming adds realistic timing variations
func RandomizeTiming() {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	delay := time.Duration(r.Intn(500)+200) * time.Millisecond
	time.Sleep(delay)
}

// GetTimezoneOffset returns timezone offset in seconds
func GetTimezoneOffset() int {
	_, offset := time.Now().Zone()
	return offset
}

// GetRandomBatteryLevel returns a realistic battery level
func GetRandomBatteryLevel() int {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return r.Intn(60) + 40 // 40-100%
}

// GetRandomLatency returns a realistic latency value
func GetRandomLatency() int {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return r.Intn(4) + 1 // 1-5ms
}

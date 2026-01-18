package client

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	SessionDir      = ".local/go-instagram-cli/db"
	SessionFile     = "session.enc"
	KeyFile         = ".key"
	CredentialsFile = "credentials.enc"
	CacheFile       = "cache.enc"
)

type SessionStorage struct {
	basePath string
	key      []byte
}

type StoredSession struct {
	Username          string            `json:"username"`
	PasswordHash      string            `json:"password_hash"`
	SessionData       map[string]any    `json:"session_data"`
	AuthorizationData map[string]any    `json:"authorization_data"`
	Cookies           map[string]string `json:"cookies"`
	LastLogin         int64             `json:"last_login"`
	DeviceSettings    *DeviceSettings   `json:"device_settings"`
	UUIDs             map[string]string `json:"uuids"`
}

// StoredCredentials holds encrypted username/password for quick re-login
type StoredCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"` // Encrypted
}

// CacheData holds cached API responses with TTL
type CacheData struct {
	Inbox   *CachedInbox             `json:"inbox,omitempty"`
	Threads map[string]*CachedThread `json:"threads,omitempty"`
}

// CachedInbox holds cached inbox data
type CachedInbox struct {
	Data      json.RawMessage `json:"data"`
	CachedAt  int64           `json:"cached_at"`
	ExpiresAt int64           `json:"expires_at"`
}

// CachedThread holds cached thread data
type CachedThread struct {
	Data      json.RawMessage `json:"data"`
	CachedAt  int64           `json:"cached_at"`
	ExpiresAt int64           `json:"expires_at"`
}

func NewSessionStorage() (*SessionStorage, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	basePath := filepath.Join(homeDir, SessionDir)

	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	ss := &SessionStorage{
		basePath: basePath,
	}

	if err := ss.loadOrGenerateKey(); err != nil {
		return nil, err
	}

	return ss, nil
}

func (ss *SessionStorage) loadOrGenerateKey() error {
	keyPath := filepath.Join(ss.basePath, KeyFile)

	keyData, err := os.ReadFile(keyPath)
	if err == nil && len(keyData) == 32 {
		ss.key = keyData
		return nil
	}

	ss.key = make([]byte, 32)
	if _, err := rand.Read(ss.key); err != nil {
		return fmt.Errorf("failed to generate encryption key: %w", err)
	}

	if err := os.WriteFile(keyPath, ss.key, 0600); err != nil {
		return fmt.Errorf("failed to save encryption key: %w", err)
	}

	return nil
}

func (ss *SessionStorage) encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(ss.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func (ss *SessionStorage) decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(ss.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func HashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return base64.StdEncoding.EncodeToString(hash[:])
}

func (ss *SessionStorage) SaveSession(client *Client, password string) error {
	stored := &StoredSession{
		Username:          client.Username,
		PasswordHash:      HashPassword(password),
		SessionData:       client.GetSettings(),
		AuthorizationData: client.AuthorizationData,
		Cookies:           client.Cookies,
		LastLogin:         client.LastLogin,
		DeviceSettings:    client.DeviceSettings,
		UUIDs: map[string]string{
			"phone_id":          client.PhoneID,
			"uuid":              client.UUID,
			"client_session_id": client.ClientSessionID,
			"advertising_id":    client.AdvertisingID,
			"android_device_id": client.AndroidDeviceID,
			"request_id":        client.RequestID,
			"tray_session_id":   client.TraySessionID,
		},
	}

	jsonData, err := json.Marshal(stored)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	encrypted, err := ss.encrypt(jsonData)
	if err != nil {
		return fmt.Errorf("failed to encrypt session: %w", err)
	}

	sessionPath := filepath.Join(ss.basePath, SessionFile)
	if err := os.WriteFile(sessionPath, encrypted, 0600); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

func (ss *SessionStorage) LoadSession() (*StoredSession, error) {
	sessionPath := filepath.Join(ss.basePath, SessionFile)

	encrypted, err := os.ReadFile(sessionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	decrypted, err := ss.decrypt(encrypted)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt session: %w", err)
	}

	var stored StoredSession
	if err := json.Unmarshal(decrypted, &stored); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &stored, nil
}

func (ss *SessionStorage) RestoreClient(stored *StoredSession) (*Client, error) {
	client := NewClient()
	client.Username = stored.Username

	if stored.UUIDs != nil {
		if v, ok := stored.UUIDs["phone_id"]; ok {
			client.PhoneID = v
		}
		if v, ok := stored.UUIDs["uuid"]; ok {
			client.UUID = v
		}
		if v, ok := stored.UUIDs["client_session_id"]; ok {
			client.ClientSessionID = v
		}
		if v, ok := stored.UUIDs["advertising_id"]; ok {
			client.AdvertisingID = v
		}
		if v, ok := stored.UUIDs["android_device_id"]; ok {
			client.AndroidDeviceID = v
		}
		if v, ok := stored.UUIDs["request_id"]; ok {
			client.RequestID = v
		}
		if v, ok := stored.UUIDs["tray_session_id"]; ok {
			client.TraySessionID = v
		}
	}

	if stored.DeviceSettings != nil {
		client.DeviceSettings = stored.DeviceSettings
	}

	if stored.AuthorizationData != nil {
		client.AuthorizationData = stored.AuthorizationData
	}

	if stored.Cookies != nil {
		client.Cookies = stored.Cookies
		client.restoreCookies()
	}

	client.LastLogin = stored.LastLogin

	if stored.SessionData != nil {
		if err := client.SetSettings(stored.SessionData); err != nil {
			return nil, fmt.Errorf("failed to restore session settings: %w", err)
		}
	}

	return client, nil
}

func (ss *SessionStorage) HasSession() bool {
	sessionPath := filepath.Join(ss.basePath, SessionFile)
	_, err := os.Stat(sessionPath)
	return err == nil
}

func (ss *SessionStorage) DeleteSession() error {
	sessionPath := filepath.Join(ss.basePath, SessionFile)
	err := os.Remove(sessionPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

func (ss *SessionStorage) VerifyPassword(stored *StoredSession, password string) bool {
	return stored.PasswordHash == HashPassword(password)
}

func (ss *SessionStorage) GetBasePath() string {
	return ss.basePath
}

// SaveCredentials saves encrypted username/password for auto-login
func (ss *SessionStorage) SaveCredentials(username, password string) error {
	creds := &StoredCredentials{
		Username: username,
		Password: password,
	}

	jsonData, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	encrypted, err := ss.encrypt(jsonData)
	if err != nil {
		return fmt.Errorf("failed to encrypt credentials: %w", err)
	}

	credsPath := filepath.Join(ss.basePath, CredentialsFile)
	if err := os.WriteFile(credsPath, encrypted, 0600); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}

	return nil
}

// LoadCredentials loads saved username/password
func (ss *SessionStorage) LoadCredentials() (*StoredCredentials, error) {
	credsPath := filepath.Join(ss.basePath, CredentialsFile)

	encrypted, err := os.ReadFile(credsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	decrypted, err := ss.decrypt(encrypted)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	var creds StoredCredentials
	if err := json.Unmarshal(decrypted, &creds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal credentials: %w", err)
	}

	return &creds, nil
}

// HasCredentials checks if saved credentials exist
func (ss *SessionStorage) HasCredentials() bool {
	credsPath := filepath.Join(ss.basePath, CredentialsFile)
	_, err := os.Stat(credsPath)
	return err == nil
}

// DeleteCredentials removes saved credentials
func (ss *SessionStorage) DeleteCredentials() error {
	credsPath := filepath.Join(ss.basePath, CredentialsFile)
	err := os.Remove(credsPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete credentials: %w", err)
	}
	return nil
}

// Cache Management

// LoadCache loads the cache data
func (ss *SessionStorage) LoadCache() (*CacheData, error) {
	cachePath := filepath.Join(ss.basePath, CacheFile)

	encrypted, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &CacheData{Threads: make(map[string]*CachedThread)}, nil
		}
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	decrypted, err := ss.decrypt(encrypted)
	if err != nil {
		// If decryption fails, return empty cache
		return &CacheData{Threads: make(map[string]*CachedThread)}, nil
	}

	var cache CacheData
	if err := json.Unmarshal(decrypted, &cache); err != nil {
		return &CacheData{Threads: make(map[string]*CachedThread)}, nil
	}

	if cache.Threads == nil {
		cache.Threads = make(map[string]*CachedThread)
	}

	return &cache, nil
}

// SaveCache saves the cache data
func (ss *SessionStorage) SaveCache(cache *CacheData) error {
	jsonData, err := json.Marshal(cache)
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	encrypted, err := ss.encrypt(jsonData)
	if err != nil {
		return fmt.Errorf("failed to encrypt cache: %w", err)
	}

	cachePath := filepath.Join(ss.basePath, CacheFile)
	if err := os.WriteFile(cachePath, encrypted, 0600); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// CacheInbox caches inbox data with TTL
func (ss *SessionStorage) CacheInbox(data []byte, ttlSeconds int64) error {
	cache, err := ss.LoadCache()
	if err != nil {
		cache = &CacheData{Threads: make(map[string]*CachedThread)}
	}

	now := getNow()
	cache.Inbox = &CachedInbox{
		Data:      data,
		CachedAt:  now,
		ExpiresAt: now + ttlSeconds,
	}

	return ss.SaveCache(cache)
}

// GetCachedInbox returns cached inbox if not expired
func (ss *SessionStorage) GetCachedInbox() ([]byte, bool) {
	cache, err := ss.LoadCache()
	if err != nil || cache.Inbox == nil {
		return nil, false
	}

	now := getNow()
	if now > cache.Inbox.ExpiresAt {
		return nil, false
	}

	return cache.Inbox.Data, true
}

// CacheThread caches thread data with TTL
func (ss *SessionStorage) CacheThread(threadID string, data []byte, ttlSeconds int64) error {
	cache, err := ss.LoadCache()
	if err != nil {
		cache = &CacheData{Threads: make(map[string]*CachedThread)}
	}

	now := getNow()
	cache.Threads[threadID] = &CachedThread{
		Data:      data,
		CachedAt:  now,
		ExpiresAt: now + ttlSeconds,
	}

	return ss.SaveCache(cache)
}

// GetCachedThread returns cached thread if not expired
func (ss *SessionStorage) GetCachedThread(threadID string) ([]byte, bool) {
	cache, err := ss.LoadCache()
	if err != nil || cache.Threads == nil {
		return nil, false
	}

	cached, ok := cache.Threads[threadID]
	if !ok {
		return nil, false
	}

	now := getNow()
	if now > cached.ExpiresAt {
		return nil, false
	}

	return cached.Data, true
}

// ClearCache clears all cached data
func (ss *SessionStorage) ClearCache() error {
	cachePath := filepath.Join(ss.basePath, CacheFile)
	err := os.Remove(cachePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear cache: %w", err)
	}
	return nil
}

// getNow returns the current Unix timestamp
func getNow() int64 {
	return time.Now().Unix()
}

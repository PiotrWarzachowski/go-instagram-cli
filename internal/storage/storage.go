package storage

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

	"github.com/PiotrWarzachowski/go-instagram-cli/internal/platform/instagram/session"
)

const (
	SessionDir      = ".local/go-instagram-cli/db"
	SessionFile     = "session.enc"
	KeyFile         = ".key"
	CredentialsFile = "credentials.enc"
	CacheFile       = "cache.enc"
)

func NewSessionStorage() (*Storage, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	basePath := filepath.Join(homeDir, SessionDir)

	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	s := &Storage{
		basePath: basePath,
	}

	if err := s.loadOrGenerateKey(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Storage) loadOrGenerateKey() error {
	keyPath := filepath.Join(s.basePath, KeyFile)

	keyData, err := os.ReadFile(keyPath)
	if err == nil && len(keyData) == 32 {
		s.key = keyData
		return nil
	}

	s.key = make([]byte, 32)
	if _, err := rand.Read(s.key); err != nil {
		return fmt.Errorf("failed to generate encryption key: %w", err)
	}

	if err := os.WriteFile(keyPath, s.key, 0600); err != nil {
		return fmt.Errorf("failed to save encryption key: %w", err)
	}

	return nil
}

func (s *Storage) encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
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

func (s *Storage) decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
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

func (s *Storage) SaveSession(sessionToStore *session.Session, password string) error {

	storedSession := &session.Session{
		Username:          sessionToStore.Username,
		PasswordHash:      HashPassword(password),
		SessionData:       sessionToStore.SessionData,
		AuthorizationData: sessionToStore.AuthorizationData,
		Cookies:           sessionToStore.Cookies,
		LastLogin:         time.Now().Unix(),
		DeviceSettings:    sessionToStore.DeviceSettings,
		UUIDs:             sessionToStore.UUIDs,
	}

	jsonData, err := json.Marshal(storedSession)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	encrypted, err := s.encrypt(jsonData)
	if err != nil {
		return fmt.Errorf("failed to encrypt session: %w", err)
	}

	sessionPath := filepath.Join(s.basePath, SessionFile)
	if err := os.WriteFile(sessionPath, encrypted, 0600); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

func (s *Storage) LoadSession() (*session.Session, error) {
	sessionPath := filepath.Join(s.basePath, SessionFile)

	encrypted, err := os.ReadFile(sessionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	decrypted, err := s.decrypt(encrypted)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt session: %w", err)
	}

	var stored session.Session
	if err := json.Unmarshal(decrypted, &stored); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &stored, nil
}

func (s *Storage) HasSession() bool {
	sessionPath := filepath.Join(s.basePath, SessionFile)
	_, err := os.Stat(sessionPath)
	return err == nil
}

func (s *Storage) DeleteSession() error {
	sessionPath := filepath.Join(s.basePath, SessionFile)
	err := os.Remove(sessionPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

func (s *Storage) VerifyPassword(stored *session.Session, password string) bool {
	return stored.PasswordHash == HashPassword(password)
}

func (s *Storage) GetBasePath() string {
	return s.basePath
}

func (s *Storage) SaveCredentials(username, password string) error {
	creds := &StoredCredentials{
		Username: username,
		Password: password,
	}

	jsonData, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	encrypted, err := s.encrypt(jsonData)
	if err != nil {
		return fmt.Errorf("failed to encrypt credentials: %w", err)
	}

	credsPath := filepath.Join(s.basePath, CredentialsFile)
	if err := os.WriteFile(credsPath, encrypted, 0600); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}

	return nil
}

func (s *Storage) LoadCredentials() (*StoredCredentials, error) {
	credsPath := filepath.Join(s.basePath, CredentialsFile)

	encrypted, err := os.ReadFile(credsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	decrypted, err := s.decrypt(encrypted)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	var creds StoredCredentials
	if err := json.Unmarshal(decrypted, &creds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal credentials: %w", err)
	}

	return &creds, nil
}

func (s *Storage) HasCredentials() bool {
	credsPath := filepath.Join(s.basePath, CredentialsFile)
	_, err := os.Stat(credsPath)
	return err == nil
}

func (s *Storage) DeleteCredentials() error {
	credsPath := filepath.Join(s.basePath, CredentialsFile)
	err := os.Remove(credsPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete credentials: %w", err)
	}
	return nil
}

func (s *Storage) LoadCache() (*CacheData, error) {
	cachePath := filepath.Join(s.basePath, CacheFile)

	encrypted, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &CacheData{Threads: make(map[string]*CachedThread)}, nil
		}
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	decrypted, err := s.decrypt(encrypted)
	if err != nil {
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

func (s *Storage) SaveCache(cache *CacheData) error {
	jsonData, err := json.Marshal(cache)
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	encrypted, err := s.encrypt(jsonData)
	if err != nil {
		return fmt.Errorf("failed to encrypt cache: %w", err)
	}

	cachePath := filepath.Join(s.basePath, CacheFile)
	if err := os.WriteFile(cachePath, encrypted, 0600); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

func (s *Storage) CacheInbox(data []byte, ttlSeconds int64) error {
	cache, err := s.LoadCache()
	if err != nil {
		cache = &CacheData{Threads: make(map[string]*CachedThread)}
	}

	now := time.Now().Unix()
	cache.Inbox = &CachedInbox{
		Data:      data,
		CachedAt:  now,
		ExpiresAt: now + ttlSeconds,
	}

	return s.SaveCache(cache)
}

func (s *Storage) GetCachedInbox() ([]byte, bool) {
	cache, err := s.LoadCache()
	if err != nil || cache.Inbox == nil {
		return nil, false
	}

	now := time.Now().Unix()
	if now > cache.Inbox.ExpiresAt {
		return nil, false
	}

	return cache.Inbox.Data, true
}

func (s *Storage) CacheThread(threadID string, data []byte, ttlSeconds int64) error {
	cache, err := s.LoadCache()
	if err != nil {
		cache = &CacheData{Threads: make(map[string]*CachedThread)}
	}

	now := time.Now().Unix()
	cache.Threads[threadID] = &CachedThread{
		Data:      data,
		CachedAt:  now,
		ExpiresAt: now + ttlSeconds,
	}

	return s.SaveCache(cache)
}

func (s *Storage) GetCachedThread(threadID string) ([]byte, bool) {
	cache, err := s.LoadCache()
	if err != nil || cache.Threads == nil {
		return nil, false
	}

	cached, ok := cache.Threads[threadID]
	if !ok {
		return nil, false
	}

	now := time.Now().Unix()
	if now > cached.ExpiresAt {
		return nil, false
	}

	return cached.Data, true
}

func (s *Storage) ClearCache() error {
	cachePath := filepath.Join(s.basePath, CacheFile)
	err := os.Remove(cachePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear cache: %w", err)
	}
	return nil
}

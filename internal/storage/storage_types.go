package storage

import (
	"encoding/json"
)

type Storage struct {
	basePath string
	key      []byte
}

type StoredCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type CacheData struct {
	Inbox   *CachedInbox             `json:"inbox,omitempty"`
	Threads map[string]*CachedThread `json:"threads,omitempty"`
}

type CachedInbox struct {
	Data      json.RawMessage `json:"data"`
	CachedAt  int64           `json:"cached_at"`
	ExpiresAt int64           `json:"expires_at"`
}

type CachedThread struct {
	Data      json.RawMessage `json:"data"`
	CachedAt  int64           `json:"cached_at"`
	ExpiresAt int64           `json:"expires_at"`
}

package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/brandonthis-that/taskOS-server/internal/store"
)

const apiKeyPrefix = "tos_"

type contextKey string

const userIDKey contextKey = "userID"
const usernameKey contextKey = "username"

// Service validates API keys and issues new credentials.
type Service struct {
	store *store.Store
}

func New(s *store.Store) *Service {
	return &Service{store: s}
}

// GenerateAPIKey returns a new opaque key, its bcrypt hash, and a lookup digest for storage.
func GenerateAPIKey() (plain, hash, lookup string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", "", err
	}
	plain = apiKeyPrefix + base64.RawURLEncoding.EncodeToString(buf)
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", "", "", err
	}
	lookup = KeyLookup(plain)
	return plain, string(h), lookup, nil
}

// KeyLookup derives a stable index for API key lookup (not secret on its own).
func KeyLookup(apiKey string) string {
	sum := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(sum[:])
}

// ValidateAPIKey resolves a bearer token to a user ID.
func (s *Service) ValidateAPIKey(ctx context.Context, apiKey string) (userID, username string, err error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" || !strings.HasPrefix(apiKey, apiKeyPrefix) {
		return "", "", errors.New("invalid api key")
	}
	u, hash, err := s.store.UserByAPIKeyLookup(ctx, KeyLookup(apiKey))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return "", "", errors.New("invalid api key")
		}
		return "", "", err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(apiKey)) != nil {
		return "", "", errors.New("invalid api key")
	}
	return u.ID, u.Username, nil
}

// WithUser attaches authenticated identity to a context.
func WithUser(ctx context.Context, userID, username string) context.Context {
	ctx = context.WithValue(ctx, userIDKey, userID)
	return context.WithValue(ctx, usernameKey, username)
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userIDKey).(string)
	return v, ok && v != ""
}

func UsernameFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(usernameKey).(string)
	return v, ok && v != ""
}

// ConstantTimeEqual compares two strings in constant time.
func ConstantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// ParseBearer extracts the token from an Authorization header.
func ParseBearer(header string) (string, error) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", fmt.Errorf("expected bearer token")
	}
	token := strings.TrimSpace(header[len(prefix):])
	if token == "" {
		return "", fmt.Errorf("empty bearer token")
	}
	return token, nil
}

package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const CookieName = "hookgram_session"

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

type Session struct {
	Token     string
	AdminID   uint
	ExpiresAt time.Time
}

// SessionManager 保存服务端 Session，重启后需要重新登录。
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]Session
	ttl      time.Duration
	secret   []byte
}

func NewSessionManager(ttl time.Duration, secret string) *SessionManager {
	return &SessionManager{
		sessions: make(map[string]Session),
		ttl:      ttl,
		secret:   []byte(secret),
	}
}

func (m *SessionManager) Create(adminID uint) (Session, error) {
	nonce, err := randomToken(32)
	if err != nil {
		return Session{}, err
	}
	token := m.sign(nonce)
	session := Session{
		Token:     token,
		AdminID:   adminID,
		ExpiresAt: time.Now().Add(m.ttl),
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[token] = session
	return session, nil
}

func (m *SessionManager) Get(token string) (Session, bool) {
	if !m.valid(token) {
		return Session{}, false
	}
	m.mu.RLock()
	session, ok := m.sessions[token]
	m.mu.RUnlock()
	if !ok {
		return Session{}, false
	}
	if time.Now().After(session.ExpiresAt) {
		m.Delete(token)
		return Session{}, false
	}
	return session, true
}

func (m *SessionManager) Delete(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, token)
}

func (m *SessionManager) DeleteByAdmin(adminID uint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for token, session := range m.sessions {
		if session.AdminID == adminID {
			delete(m.sessions, token)
		}
	}
}

func randomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (m *SessionManager) sign(nonce string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(nonce))
	return nonce + "." + hex.EncodeToString(mac.Sum(nil))
}

func (m *SessionManager) valid(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	expected := m.sign(parts[0])
	return hmac.Equal([]byte(expected), []byte(token))
}

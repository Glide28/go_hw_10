package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type authService struct {
	secret []byte
	mu     sync.RWMutex
	users  map[string][]byte
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func newAuthService(secret string) *authService {
	return &authService{secret: []byte(secret), users: make(map[string][]byte)}
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var request credentials
	if err := decodeJSON(r, &request); err != nil || !validCredentials(request) {
		writeError(w, http.StatusBadRequest, "valid email and password of at least 8 characters are required")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	email := strings.ToLower(strings.TrimSpace(request.Email))
	s.auth.mu.Lock()
	defer s.auth.mu.Unlock()
	if _, exists := s.auth.users[email]; exists {
		writeError(w, http.StatusConflict, "user already exists")
		return
	}
	s.auth.users[email] = hash
	writeJSON(w, http.StatusCreated, map[string]string{"email": email})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var request credentials
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid credentials")
		return
	}
	email := strings.ToLower(strings.TrimSpace(request.Email))
	s.auth.mu.RLock()
	hash, exists := s.auth.users[email]
	s.auth.mu.RUnlock()
	if !exists || bcrypt.CompareHashAndPassword(hash, []byte(request.Password)) != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	token, err := s.auth.token(email, time.Now().Add(24*time.Hour))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func validCredentials(value credentials) bool {
	return strings.Contains(value.Email, "@") && len(value.Password) >= 8
}

func (a *authService) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(a.secret) == 0 {
			next.ServeHTTP(w, r)
			return
		}
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") || a.verify(strings.TrimPrefix(header, "Bearer ")) != nil {
			writeError(w, http.StatusUnauthorized, "valid bearer token is required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *authService) token(subject string, expiresAt time.Time) (string, error) {
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, err := json.Marshal(map[string]any{"sub": subject, "exp": expiresAt.Unix()})
	if err != nil {
		return "", err
	}
	unsigned := encodePart(header) + "." + encodePart(payload)
	return unsigned + "." + a.sign(unsigned), nil
}

func (a *authService) verify(token string) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || !hmac.Equal([]byte(parts[2]), []byte(a.sign(parts[0]+"."+parts[1]))) {
		return errors.New("invalid token signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return err
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.Exp <= time.Now().Unix() {
		return errors.New("token expired")
	}
	return nil
}

func (a *authService) sign(value string) string {
	mac := hmac.New(sha256.New, a.secret)
	_, _ = mac.Write([]byte(value))
	return encodePart(mac.Sum(nil))
}

func encodePart(value []byte) string { return base64.RawURLEncoding.EncodeToString(value) }

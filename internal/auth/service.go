package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const (
	sessionTTL = 14 * 24 * time.Hour
	cookieName = "retrove_session"
)

var (
	ErrValidation   = errors.New("validation")
	ErrUserExists   = errors.New("user exists")
	ErrCredentials  = errors.New("invalid credentials")
	ErrUnauthorized = errors.New("unauthorized")
)

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type Service struct {
	db *sql.DB
}

func New(db *sql.DB) *Service {
	return &Service{db: db}
}

func CookieName() string { return cookieName }

func (s *Service) HasUser() (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n > 0, err
}

type RegisterInput struct {
	Username        string
	Email           string
	Password        string
	PasswordConfirm string
}

func (s *Service) Register(in RegisterInput) (User, string, error) {
	username := strings.TrimSpace(in.Username)
	email := strings.TrimSpace(in.Email)
	if err := validateRegister(username, email, in.Password, in.PasswordConfirm); err != nil {
		return User{}, "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, "", err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return User{}, "", err
	}
	defer func() { _ = tx.Rollback() }()

	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return User{}, "", err
	}
	if n > 0 {
		return User{}, "", ErrUserExists
	}
	res, err := tx.Exec(
		`INSERT INTO users (username, email, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		username, email, string(hash), time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return User{}, "", err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return User{}, "", err
	}
	token, err := newToken()
	if err != nil {
		return User{}, "", err
	}
	expires := time.Now().UTC().Add(sessionTTL)
	if _, err := tx.Exec(`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`, token, id, expires.Format(time.RFC3339)); err != nil {
		return User{}, "", err
	}
	if err := tx.Commit(); err != nil {
		return User{}, "", err
	}
	return User{ID: id, Username: username, Email: email}, token, nil
}

func (s *Service) Login(username, password string) (User, string, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return User{}, "", ErrCredentials
	}
	var user User
	var hash string
	err := s.db.QueryRow(`SELECT id, username, email, password_hash FROM users WHERE username = ?`, username).
		Scan(&user.ID, &user.Username, &user.Email, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, "", ErrCredentials
	}
	if err != nil {
		return User{}, "", err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return User{}, "", ErrCredentials
	}
	token, err := newToken()
	if err != nil {
		return User{}, "", err
	}
	expires := time.Now().UTC().Add(sessionTTL)
	if _, err := s.db.Exec(`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`, token, user.ID, expires.Format(time.RFC3339)); err != nil {
		return User{}, "", err
	}
	return user, token, nil
}

func (s *Service) Logout(token string) error {
	if token == "" {
		return nil
	}
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

func (s *Service) UserByToken(token string) (User, error) {
	if token == "" {
		return User{}, ErrUnauthorized
	}
	var user User
	var expires string
	err := s.db.QueryRow(`
		SELECT u.id, u.username, u.email, s.expires_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token = ?`, token).Scan(&user.ID, &user.Username, &user.Email, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, err
	}
	exp, err := time.Parse(time.RFC3339, expires)
	if err != nil || time.Now().After(exp) {
		_, _ = s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
		return User{}, ErrUnauthorized
	}
	return user, nil
}

func validateRegister(username, email, password, confirm string) error {
	if username == "" || strings.ContainsAny(username, " \t\r\n") || utf8.RuneCountInString(username) > 32 {
		return ErrValidation
	}
	if _, err := mail.ParseAddress(email); err != nil || !strings.Contains(email, "@") {
		return ErrValidation
	}
	if utf8.RuneCountInString(password) < 8 || password != confirm {
		return ErrValidation
	}
	return nil
}

func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

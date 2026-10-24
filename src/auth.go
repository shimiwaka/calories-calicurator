package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	sessionCookie   = "calories_sid"
	sessionLifetime = 30 * 24 * time.Hour
)

type user struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (c *ctx) currentUser() (*user, error) {
	ck, err := c.r.Cookie(sessionCookie)
	if err != nil || ck.Value == "" {
		return nil, nil
	}
	var u user
	err = c.db.QueryRow(
		"SELECT user_id, username FROM sessions WHERE id = ? AND expires_at > ?",
		hashToken(ck.Value), time.Now().Unix(),
	).Scan(&u.ID, &u.Username)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}
	return &u, nil
}

func (c *ctx) requireAuth() (*user, error) {
	u, err := c.currentUser()
	if err != nil {
		return nil, err
	}
	if u == nil {
		c.json(http.StatusUnauthorized, errBody("Unauthorized"))
	}
	return u, nil
}

func (c *ctx) setSessionCookie(value string, maxAge int) {
	http.SetCookie(c.w, &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   c.r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
}

func (c *ctx) destroySession() error {
	if ck, err := c.r.Cookie(sessionCookie); err == nil && ck.Value != "" {
		if _, err := c.db.Exec("DELETE FROM sessions WHERE id = ?", hashToken(ck.Value)); err != nil {
			return fmt.Errorf("delete session: %w", err)
		}
	}
	return nil
}

func (c *ctx) startSession(u user) error {
	if err := c.destroySession(); err != nil {
		return err
	}
	if _, err := c.db.Exec("DELETE FROM sessions WHERE expires_at <= ?", time.Now().Unix()); err != nil {
		return fmt.Errorf("purge sessions: %w", err)
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Errorf("generate session id: %w", err)
	}
	token := hex.EncodeToString(buf)
	_, err := c.db.Exec(
		"INSERT INTO sessions (id, user_id, username, expires_at) VALUES (?, ?, ?, ?)",
		hashToken(token), u.ID, u.Username, time.Now().Add(sessionLifetime).Unix(),
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	c.setSessionCookie(token, int(sessionLifetime/time.Second))
	return nil
}

func (c *ctx) auth() error {
	action, _ := c.query("action")
	method := c.r.Method

	switch {
	case action == "register" && method == http.MethodPost:
		return c.register()
	case action == "login" && method == http.MethodPost:
		return c.login()
	case action == "logout" && method == http.MethodPost:
		if err := c.destroySession(); err != nil {
			return err
		}
		c.setSessionCookie("", -1)
		c.json(http.StatusOK, map[string]bool{"ok": true})
		return nil
	case action == "me" && method == http.MethodGet:
		u, err := c.requireAuth()
		if u == nil {
			return err
		}
		c.json(http.StatusOK, u)
		return nil
	}
	c.json(http.StatusNotFound, errBody("Not Found"))
	return nil
}

func (c *ctx) register() error {
	b := c.body()
	username := phpTrim(b.toString("username"))
	password := b.toString("password")

	if username == "" || len(password) < 4 {
		c.json(http.StatusBadRequest, errBody("ユーザー名とパスワード（4文字以上）を入力してください"))
		return nil
	}

	var id int64
	err := c.db.QueryRow("SELECT id FROM users WHERE username = ?", username).Scan(&id)
	if err == nil {
		c.json(http.StatusConflict, errBody("そのユーザー名は既に使われています"))
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("find user: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	res, err := c.db.Exec(
		"INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)",
		username, string(hash), now().Format(datetimeLayout),
	)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	if id, err = res.LastInsertId(); err != nil {
		return fmt.Errorf("insert user: %w", err)
	}

	u := user{ID: id, Username: username}
	if err := c.startSession(u); err != nil {
		return err
	}
	c.json(http.StatusOK, u)
	return nil
}

func (c *ctx) login() error {
	b := c.body()
	username := phpTrim(b.toString("username"))
	password := b.toString("password")

	var u user
	var hash string
	err := c.db.QueryRow("SELECT id, password_hash FROM users WHERE username = ?", username).Scan(&u.ID, &hash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("find user: %w", err)
	}
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		c.json(http.StatusUnauthorized, errBody("ユーザー名またはパスワードが正しくありません"))
		return nil
	}

	u.Username = username
	if err := c.startSession(u); err != nil {
		return err
	}
	c.json(http.StatusOK, u)
	return nil
}

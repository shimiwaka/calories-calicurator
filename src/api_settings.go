package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
)

type apiSetting struct {
	APIURL   string `json:"api_url"`
	APIToken string `json:"api_token"`
}

func (c *ctx) apiSettings() error {
	u, err := c.requireAuth()
	if u == nil {
		return err
	}

	switch c.r.Method {
	case http.MethodGet:
		var s apiSetting
		err := c.db.QueryRow("SELECT api_url, api_token FROM user_api_settings WHERE user_id = ?", u.ID).
			Scan(&s.APIURL, &s.APIToken)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("get api settings: %w", err)
		}
		c.json(http.StatusOK, s)
		return nil

	case http.MethodPost:
		b := c.body()
		s := apiSetting{
			APIURL:   phpTrim(b.toString("api_url")),
			APIToken: phpTrim(b.toString("api_token")),
		}
		if _, err := c.db.Exec(
			`INSERT INTO user_api_settings (user_id, api_url, api_token)
			 VALUES (?, ?, ?)
			 ON CONFLICT(user_id) DO UPDATE SET api_url = excluded.api_url, api_token = excluded.api_token`,
			u.ID, s.APIURL, s.APIToken,
		); err != nil {
			return fmt.Errorf("save api settings: %w", err)
		}
		c.json(http.StatusOK, map[string]bool{"ok": true})
		return nil
	}
	c.methodNotAllowed()
	return nil
}

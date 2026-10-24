package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
)

type dailyEvent struct {
	ID         int64  `json:"id"`
	RecordedAt string `json:"recorded_at"`
	EventType  string `json:"event_type"`
}

func (c *ctx) events() error {
	u, err := c.requireAuth()
	if u == nil {
		return err
	}
	switch c.r.Method {
	case http.MethodGet:
		return c.listEvents(u.ID)
	case http.MethodPost:
		return c.addEvent(u.ID)
	case http.MethodPut:
		return c.updateEventTime(u.ID)
	case http.MethodDelete:
		return c.deleteEvent(u.ID)
	}
	c.methodNotAllowed()
	return nil
}

func (c *ctx) listEvents(userID int64) error {
	date, ok := c.query("date")
	if !ok {
		date = todayDate()
	}
	if !dateRe.MatchString(date) {
		c.json(http.StatusBadRequest, errBody("日付の形式が正しくありません"))
		return nil
	}

	rows, err := c.db.Query(
		`SELECT id, recorded_at, event_type FROM daily_events
		 WHERE user_id = ? AND date = ? ORDER BY recorded_at ASC`,
		userID, date,
	)
	if err != nil {
		return fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	list := []dailyEvent{}
	for rows.Next() {
		var e dailyEvent
		if err := rows.Scan(&e.ID, &e.RecordedAt, &e.EventType); err != nil {
			return fmt.Errorf("scan event: %w", err)
		}
		list = append(list, e)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("list events: %w", err)
	}
	c.json(http.StatusOK, list)
	return nil
}

func (c *ctx) addEvent(userID int64) error {
	b := c.body()
	eventType, _ := b.str("event_type")
	if eventType != "excretion" && eventType != "weigh_in" {
		c.json(http.StatusBadRequest, errBody("event_typeはexcretionまたはweigh_inを指定してください"))
		return nil
	}

	recordedAt := now().Format(datetimeLayout)
	date, ok := b.str("date")
	if !ok || !dateRe.MatchString(date) {
		date = todayDate()
	}

	res, err := c.db.Exec(
		"INSERT INTO daily_events (user_id, recorded_at, event_type, date) VALUES (?, ?, ?, ?)",
		userID, recordedAt, eventType, date,
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	c.json(http.StatusOK, map[string]interface{}{
		"id":          id,
		"recorded_at": recordedAt,
		"event_type":  eventType,
		"date":        date,
	})
	return nil
}

func (c *ctx) updateEventTime(userID int64) error {
	b := c.body()
	id := b.toInt("id")
	hhmm, _ := b.str("time")
	if id <= 0 || !hhmmRe.MatchString(hhmm) {
		c.json(http.StatusBadRequest, errBody("idとtime（HH:MM形式）が必要です"))
		return nil
	}

	var date string
	err := c.db.QueryRow("SELECT date FROM daily_events WHERE id = ? AND user_id = ?", id, userID).Scan(&date)
	if errors.Is(err, sql.ErrNoRows) {
		c.json(http.StatusNotFound, errBody("イベントが見つかりません"))
		return nil
	}
	if err != nil {
		return fmt.Errorf("find event: %w", err)
	}

	recordedAt := date + " " + hhmm + ":00"
	if _, err := c.db.Exec(
		"UPDATE daily_events SET recorded_at = ? WHERE id = ? AND user_id = ?",
		recordedAt, id, userID,
	); err != nil {
		return fmt.Errorf("update event: %w", err)
	}
	c.json(http.StatusOK, map[string]interface{}{"ok": true, "recorded_at": recordedAt})
	return nil
}

func (c *ctx) deleteEvent(userID int64) error {
	id := c.queryInt("id", 0)
	if id <= 0 {
		c.json(http.StatusBadRequest, errBody("idが必要です"))
		return nil
	}
	if _, err := c.db.Exec("DELETE FROM daily_events WHERE id = ? AND user_id = ?", id, userID); err != nil {
		return fmt.Errorf("delete event: %w", err)
	}
	c.json(http.StatusOK, map[string]bool{"ok": true})
	return nil
}

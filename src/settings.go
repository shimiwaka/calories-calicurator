package main

import (
	"fmt"
	"net/http"
)

type calorieSetting struct {
	ID               int64   `json:"id"`
	StartDate        string  `json:"start_date"`
	EndDate          *string `json:"end_date"`
	BaseIntakeKcal   int64   `json:"base_intake_kcal"`
	BaseExerciseKcal int64   `json:"base_exercise_kcal"`
}

func (c *ctx) settings() error {
	u, err := c.requireAuth()
	if u == nil {
		return err
	}
	switch c.r.Method {
	case http.MethodGet:
		return c.listSettings(u.ID)
	case http.MethodPost:
		return c.saveSetting(u.ID)
	case http.MethodDelete:
		return c.deleteSetting(u.ID)
	}
	c.methodNotAllowed()
	return nil
}

func (c *ctx) listSettings(userID int64) error {
	rows, err := c.db.Query(
		`SELECT id, start_date, end_date, base_intake_kcal, base_exercise_kcal
		 FROM calorie_settings WHERE user_id = ? ORDER BY start_date DESC`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("list settings: %w", err)
	}
	defer rows.Close()

	list := []calorieSetting{}
	for rows.Next() {
		var s calorieSetting
		if err := rows.Scan(&s.ID, &s.StartDate, &s.EndDate, &s.BaseIntakeKcal, &s.BaseExerciseKcal); err != nil {
			return fmt.Errorf("scan setting: %w", err)
		}
		list = append(list, s)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("list settings: %w", err)
	}
	c.json(http.StatusOK, list)
	return nil
}

func (c *ctx) hasOverlap(userID int64, start string, end *string, excludeID *int64) (bool, error) {
	q := `SELECT COUNT(*) FROM calorie_settings
	      WHERE user_id = ?
	        AND (? IS NULL OR start_date <= ?)
	        AND (end_date IS NULL OR end_date >= ?)`
	args := []interface{}{userID, end, end, start}
	if excludeID != nil {
		q += " AND id != ?"
		args = append(args, *excludeID)
	}
	var n int
	if err := c.db.QueryRow(q, args...).Scan(&n); err != nil {
		return false, fmt.Errorf("check overlap: %w", err)
	}
	return n > 0, nil
}

func (c *ctx) saveSetting(userID int64) error {
	b := c.body()
	var id *int64
	if b.has("id") {
		v := b.toInt("id")
		id = &v
	}
	startDate, _ := b.str("start_date")
	var endDate *string
	if b.has("end_date") {
		s, _ := b.str("end_date")
		endDate = &s
	}
	baseIntake, intakeOK := b.strictInt("base_intake_kcal")
	baseExercise, exerciseOK := b.strictInt("base_exercise_kcal")

	if !dateRe.MatchString(startDate) {
		c.json(http.StatusBadRequest, errBody("start_dateの形式が正しくありません"))
		return nil
	}
	if endDate != nil && !dateRe.MatchString(*endDate) {
		c.json(http.StatusBadRequest, errBody("end_dateの形式が正しくありません"))
		return nil
	}
	if !intakeOK || baseIntake < 0 || !exerciseOK || baseExercise < 0 {
		c.json(http.StatusBadRequest, errBody("基準カロリーは0以上の整数で入力してください"))
		return nil
	}

	overlap, err := c.hasOverlap(userID, startDate, endDate, id)
	if err != nil {
		return err
	}
	if overlap {
		c.json(http.StatusConflict, errBody("期間が重複しています"))
		return nil
	}

	if id == nil {
		res, err := c.db.Exec(
			`INSERT INTO calorie_settings (user_id, start_date, end_date, base_intake_kcal, base_exercise_kcal)
			 VALUES (?, ?, ?, ?, ?)`,
			userID, startDate, endDate, baseIntake, baseExercise,
		)
		if err != nil {
			return fmt.Errorf("insert setting: %w", err)
		}
		newID, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("insert setting: %w", err)
		}
		c.json(http.StatusOK, map[string]interface{}{"id": newID, "ok": true})
		return nil
	}

	if _, err := c.db.Exec(
		`UPDATE calorie_settings
		 SET start_date = ?, end_date = ?, base_intake_kcal = ?, base_exercise_kcal = ?
		 WHERE id = ? AND user_id = ?`,
		startDate, endDate, baseIntake, baseExercise, *id, userID,
	); err != nil {
		return fmt.Errorf("update setting: %w", err)
	}
	c.json(http.StatusOK, map[string]bool{"ok": true})
	return nil
}

func (c *ctx) deleteSetting(userID int64) error {
	id := c.queryInt("id", 0)
	if id <= 0 {
		c.json(http.StatusBadRequest, errBody("idが必要です"))
		return nil
	}
	if _, err := c.db.Exec("DELETE FROM calorie_settings WHERE id = ? AND user_id = ?", id, userID); err != nil {
		return fmt.Errorf("delete setting: %w", err)
	}
	c.json(http.StatusOK, map[string]bool{"ok": true})
	return nil
}

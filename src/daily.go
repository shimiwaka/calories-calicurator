package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
)

type dailyRecord struct {
	Date         string  `json:"date"`
	IntakeKcal   *int64  `json:"intake_kcal"`
	ExerciseKcal *int64  `json:"exercise_kcal"`
	SnackKcal    *int64  `json:"snack_kcal"`
	Memo         *string `json:"memo"`
}

type baseSetting struct {
	BaseIntakeKcal   int64 `json:"base_intake_kcal"`
	BaseExerciseKcal int64 `json:"base_exercise_kcal"`
}

func (c *ctx) daily() error {
	u, err := c.requireAuth()
	if u == nil {
		return err
	}
	switch c.r.Method {
	case http.MethodGet:
		return c.getDaily(u.ID)
	case http.MethodPost:
		return c.saveDaily(u.ID)
	}
	c.methodNotAllowed()
	return nil
}

func (c *ctx) getDaily(userID int64) error {
	date, ok := c.query("date")
	if !ok {
		date = todayDate()
	}
	if !dateRe.MatchString(date) {
		c.json(http.StatusBadRequest, errBody("日付の形式が正しくありません"))
		return nil
	}

	record := &dailyRecord{}
	err := c.db.QueryRow(
		`SELECT date, intake_kcal, exercise_kcal, snack_kcal, memo
		 FROM daily_records WHERE user_id = ? AND date = ?`,
		userID, date,
	).Scan(&record.Date, &record.IntakeKcal, &record.ExerciseKcal, &record.SnackKcal, &record.Memo)
	if errors.Is(err, sql.ErrNoRows) {
		record = nil
	} else if err != nil {
		return fmt.Errorf("get daily record: %w", err)
	}

	setting := &baseSetting{}
	err = c.db.QueryRow(
		`SELECT base_intake_kcal, base_exercise_kcal FROM calorie_settings
		 WHERE user_id = ? AND start_date <= ? AND (end_date IS NULL OR end_date >= ?)
		 LIMIT 1`,
		userID, date, date,
	).Scan(&setting.BaseIntakeKcal, &setting.BaseExerciseKcal)
	if errors.Is(err, sql.ErrNoRows) {
		setting = nil
	} else if err != nil {
		return fmt.Errorf("get calorie setting: %w", err)
	}

	c.json(http.StatusOK, map[string]interface{}{
		"date":    date,
		"today":   todayDate(),
		"record":  record,
		"setting": setting,
	})
	return nil
}

func (c *ctx) saveDaily(userID int64) error {
	b := c.body()
	date := todayDate()
	if b.has("date") {
		date, _ = b.str("date")
	}
	if !dateRe.MatchString(date) {
		c.json(http.StatusBadRequest, errBody("日付の形式が正しくありません"))
		return nil
	}

	kcal := map[string]*int64{}
	for _, f := range []string{"intake_kcal", "exercise_kcal", "snack_kcal"} {
		if !b.has(f) {
			kcal[f] = nil
			continue
		}
		v, ok := b.strictInt(f)
		if !ok || v < 0 {
			c.json(http.StatusBadRequest, errBody(f+"は0以上の整数で入力してください"))
			return nil
		}
		kcal[f] = &v
	}
	var memo *string
	if b.has("memo") {
		s := b.toString("memo")
		memo = &s
	}

	_, err := c.db.Exec(
		`INSERT INTO daily_records (user_id, date, intake_kcal, exercise_kcal, snack_kcal, memo)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(user_id, date) DO UPDATE SET
		   intake_kcal   = excluded.intake_kcal,
		   exercise_kcal = excluded.exercise_kcal,
		   snack_kcal    = excluded.snack_kcal,
		   memo          = excluded.memo`,
		userID, date, kcal["intake_kcal"], kcal["exercise_kcal"], kcal["snack_kcal"], memo,
	)
	if err != nil {
		return fmt.Errorf("save daily record: %w", err)
	}
	c.json(http.StatusOK, map[string]interface{}{"ok": true, "date": date})
	return nil
}

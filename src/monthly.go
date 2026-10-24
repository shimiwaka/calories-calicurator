package main

import (
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"time"
)

func (c *ctx) monthly() error {
	u, err := c.requireAuth()
	if u == nil {
		return err
	}

	t := now()
	year := c.queryInt("year", int64(t.Year()))
	month := c.queryInt("month", int64(t.Month()))
	if year < 2000 || year > 2100 || month < 1 || month > 12 {
		c.json(http.StatusBadRequest, errBody("年月の値が正しくありません"))
		return nil
	}

	first := time.Date(int(year), time.Month(month), 1, 0, 0, 0, 0, jst)
	from := first.Format("2006-01-02")
	to := first.AddDate(0, 1, -1).Format("2006-01-02")

	rows, err := c.db.Query(
		`SELECT dr.intake_kcal, dr.exercise_kcal, dr.snack_kcal,
		        cs.base_intake_kcal, cs.base_exercise_kcal
		 FROM daily_records dr
		 LEFT JOIN calorie_settings cs
		   ON cs.user_id = dr.user_id
		  AND cs.start_date <= dr.date
		  AND (cs.end_date IS NULL OR cs.end_date >= dr.date)
		 WHERE dr.user_id = ? AND dr.date BETWEEN ? AND ?`,
		u.ID, from, to,
	)
	if err != nil {
		return fmt.Errorf("monthly records: %w", err)
	}
	defer rows.Close()

	var totalDiff, totalSnack, totalIntake, totalExercise, days int64
	for rows.Next() {
		var intake, exercise, snack, baseIn, baseEx sql.NullInt64
		if err := rows.Scan(&intake, &exercise, &snack, &baseIn, &baseEx); err != nil {
			return fmt.Errorf("scan monthly record: %w", err)
		}
		totalDiff += intake.Int64 - baseIn.Int64 - exercise.Int64 + baseEx.Int64
		totalSnack += snack.Int64
		totalIntake += intake.Int64
		totalExercise += exercise.Int64
		days++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("monthly records: %w", err)
	}

	avg := func(total int64) int64 {
		if days == 0 {
			return 0
		}
		return int64(math.Round(float64(total) / float64(days)))
	}
	c.json(http.StatusOK, map[string]interface{}{
		"year":              year,
		"month":             month,
		"total_diff_kcal":   totalDiff,
		"avg_snack_kcal":    avg(totalSnack),
		"avg_intake_kcal":   avg(totalIntake),
		"avg_exercise_kcal": avg(totalExercise),
		"avg_diff_kcal":     avg(totalDiff),
		"days_recorded":     days,
	})
	return nil
}

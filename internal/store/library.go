package store

import (
	"context"
	"encoding/json"
)

// ---- presets: a person's own breathing patterns and timers ----

const presetCols = `id, user_id, kind, name, config, favorite, sort, created_at, updated_at`

func scanPreset(row interface{ Scan(...any) error }) (*Preset, error) {
	var p Preset
	var cfg string
	var fav int
	if err := row.Scan(&p.ID, &p.UserID, &p.Kind, &p.Name, &cfg, &fav, &p.Sort, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, notFound(err)
	}
	p.Config = json.RawMessage(cfg)
	p.Favorite = fav == 1
	return &p, nil
}

// ListPresets lists a person's presets of a kind ("" = all).
func (s *Store) ListPresets(ctx context.Context, uid int64, kind string) ([]Preset, error) {
	q := `SELECT ` + presetCols + ` FROM presets WHERE user_id = ?`
	args := []any{uid}
	if kind != "" {
		q += ` AND kind = ?`
		args = append(args, kind)
	}
	rows, err := s.DB.QueryContext(ctx, q+` ORDER BY favorite DESC, sort, name COLLATE NOCASE`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Preset{}
	for rows.Next() {
		p, err := scanPreset(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// GetPreset loads one of a person's presets.
func (s *Store) GetPreset(ctx context.Context, uid, id int64) (*Preset, error) {
	return scanPreset(s.DB.QueryRowContext(ctx, `SELECT `+presetCols+` FROM presets WHERE user_id = ? AND id = ?`, uid, id))
}

// CreatePreset saves a new preset.
func (s *Store) CreatePreset(ctx context.Context, uid int64, p *Preset) error {
	now := Now()
	res, err := s.DB.ExecContext(ctx, `INSERT INTO presets (user_id, kind, name, config, favorite, sort, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		uid, p.Kind, p.Name, string(p.Config), boolInt(p.Favorite), p.Sort, now, now)
	if err != nil {
		return err
	}
	p.ID, _ = res.LastInsertId()
	p.CreatedAt, p.UpdatedAt = now, now
	return nil
}

// UpdatePreset saves a preset.
func (s *Store) UpdatePreset(ctx context.Context, uid int64, p *Preset) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE presets SET name = ?, config = ?, favorite = ?, sort = ?, updated_at = ? WHERE user_id = ? AND id = ?`,
		p.Name, string(p.Config), boolInt(p.Favorite), p.Sort, Now(), uid, p.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeletePreset removes a preset.
func (s *Store) DeletePreset(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM presets WHERE user_id = ? AND id = ?`, uid, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- favourites among the built-in techniques ----

// Favorites lists the built-in keys a person starred.
func (s *Store) Favorites(ctx context.Context, uid int64) (map[string]bool, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT key FROM favorites WHERE user_id = ?`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		out[k] = true
	}
	return out, rows.Err()
}

// SetFavorite stars or unstars a built-in technique.
func (s *Store) SetFavorite(ctx context.Context, uid int64, key string, on bool) error {
	if on {
		_, err := s.DB.ExecContext(ctx, `INSERT OR IGNORE INTO favorites (user_id, key) VALUES (?, ?)`, uid, key)
		return err
	}
	_, err := s.DB.ExecContext(ctx, `DELETE FROM favorites WHERE user_id = ? AND key = ?`, uid, key)
	return err
}

// ---- programs ----

// ProgramStarts maps each program a person started to when.
func (s *Store) ProgramStarts(ctx context.Context, uid int64) (map[string]string, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT program, started_at FROM program_starts WHERE user_id = ?`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var p, at string
		if err := rows.Scan(&p, &at); err != nil {
			return nil, err
		}
		out[p] = at
	}
	return out, rows.Err()
}

// StartProgram records that a person began a program (once).
func (s *Store) StartProgram(ctx context.Context, uid int64, program string) error {
	_, err := s.DB.ExecContext(ctx, `INSERT OR IGNORE INTO program_starts (user_id, program, started_at) VALUES (?, ?, ?)`, uid, program, Now())
	return err
}

// LessonDone is a finished lesson.
type LessonDone struct {
	Program    string `json:"program"`
	Lesson     string `json:"lesson"`
	PracticeID *int64 `json:"practice_id"`
	DoneAt     string `json:"done_at"`
}

// LessonsDone lists the lessons a person finished, per program ("" = all programs).
func (s *Store) LessonsDone(ctx context.Context, uid int64, program string) ([]LessonDone, error) {
	q := `SELECT program, lesson, practice_id, done_at FROM lessons_done WHERE user_id = ?`
	args := []any{uid}
	if program != "" {
		q += ` AND program = ?`
		args = append(args, program)
	}
	rows, err := s.DB.QueryContext(ctx, q+` ORDER BY done_at`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LessonDone{}
	for rows.Next() {
		var l LessonDone
		if err := rows.Scan(&l.Program, &l.Lesson, &l.PracticeID, &l.DoneAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// CompleteLesson marks a lesson done; it reports whether it was new.
func (s *Store) CompleteLesson(ctx context.Context, uid int64, program, lesson string, practiceID int64) (bool, error) {
	var pid any
	if practiceID > 0 {
		pid = practiceID
	}
	res, err := s.DB.ExecContext(ctx, `INSERT OR IGNORE INTO lessons_done (user_id, program, lesson, practice_id, done_at) VALUES (?, ?, ?, ?, ?)`, uid, program, lesson, pid, Now())
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		_ = s.StartProgram(ctx, uid, program)
	}
	return n > 0, nil
}

// ResetProgram forgets a person's progress in a program (their practices stay).
func (s *Store) ResetProgram(ctx context.Context, uid int64, program string) error {
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM lessons_done WHERE user_id = ? AND program = ?`, uid, program); err != nil {
		return err
	}
	_, err := s.DB.ExecContext(ctx, `DELETE FROM program_starts WHERE user_id = ? AND program = ?`, uid, program)
	return err
}

// ---- milestones ----

// Unlocked is a reached milestone.
type Unlocked struct {
	Key        string `json:"key"`
	UnlockedAt string `json:"unlocked_at"`
	Notified   bool   `json:"-"`
}

// Achievements lists what a person has unlocked.
func (s *Store) Achievements(ctx context.Context, uid int64) (map[string]Unlocked, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT key, unlocked_at, notified FROM achievements WHERE user_id = ?`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Unlocked{}
	for rows.Next() {
		var u Unlocked
		var n int
		if err := rows.Scan(&u.Key, &u.UnlockedAt, &n); err != nil {
			return nil, err
		}
		u.Notified = n == 1
		out[u.Key] = u
	}
	return out, rows.Err()
}

// Unlock records a milestone; it reports whether it was new.
func (s *Store) Unlock(ctx context.Context, uid int64, key, at string) (bool, error) {
	if at == "" {
		at = Now()
	}
	res, err := s.DB.ExecContext(ctx, `INSERT OR IGNORE INTO achievements (user_id, key, unlocked_at) VALUES (?, ?, ?)`, uid, key, at)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// MarkAchievementsNotified marks milestones as pushed.
func (s *Store) MarkAchievementsNotified(ctx context.Context, uid int64, keys []string) error {
	for _, k := range keys {
		if _, err := s.DB.ExecContext(ctx, `UPDATE achievements SET notified = 1 WHERE user_id = ? AND key = ?`, uid, k); err != nil {
			return err
		}
	}
	return nil
}

// ---- habit rules ----

const ruleCols = `id, user_id, kind, min_minutes, habit_id, habit_name, enabled, created_at`

func scanRule(row interface{ Scan(...any) error }) (*HabitRule, error) {
	var r HabitRule
	var en int
	if err := row.Scan(&r.ID, &r.UserID, &r.Kind, &r.MinMinutes, &r.HabitID, &r.HabitName, &en, &r.CreatedAt); err != nil {
		return nil, notFound(err)
	}
	r.Enabled = en == 1
	return &r, nil
}

// ListRules lists a person's habit rules.
func (s *Store) ListRules(ctx context.Context, uid int64) ([]HabitRule, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+ruleCols+` FROM habit_rules WHERE user_id = ? ORDER BY id`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []HabitRule{}
	for rows.Next() {
		r, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// GetRule loads one rule.
func (s *Store) GetRule(ctx context.Context, uid, id int64) (*HabitRule, error) {
	return scanRule(s.DB.QueryRowContext(ctx, `SELECT `+ruleCols+` FROM habit_rules WHERE user_id = ? AND id = ?`, uid, id))
}

// CreateRule saves a rule.
func (s *Store) CreateRule(ctx context.Context, uid int64, r *HabitRule) error {
	res, err := s.DB.ExecContext(ctx, `INSERT INTO habit_rules (user_id, kind, min_minutes, habit_id, habit_name, enabled) VALUES (?, ?, ?, ?, ?, ?)`,
		uid, r.Kind, r.MinMinutes, r.HabitID, r.HabitName, boolInt(r.Enabled))
	if err != nil {
		return err
	}
	r.ID, _ = res.LastInsertId()
	return nil
}

// UpdateRule saves a rule.
func (s *Store) UpdateRule(ctx context.Context, uid int64, r *HabitRule) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE habit_rules SET kind = ?, min_minutes = ?, habit_id = ?, habit_name = ?, enabled = ? WHERE user_id = ? AND id = ?`,
		r.Kind, r.MinMinutes, r.HabitID, r.HabitName, boolInt(r.Enabled), uid, r.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteRule removes a rule.
func (s *Store) DeleteRule(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM habit_rules WHERE user_id = ? AND id = ?`, uid, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// HabitMarked reports whether a rule already ticked its habit on a day.
func (s *Store) HabitMarked(ctx context.Context, uid, ruleID int64, date string) bool {
	var n int
	_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM habit_marks WHERE user_id = ? AND rule_id = ? AND date = ?`, uid, ruleID, date).Scan(&n)
	return n > 0
}

// MarkHabitDay records that a rule ticked its habit on a day.
func (s *Store) MarkHabitDay(ctx context.Context, uid, ruleID int64, date string) error {
	_, err := s.DB.ExecContext(ctx, `INSERT OR IGNORE INTO habit_marks (user_id, rule_id, date, marked_at) VALUES (?, ?, ?, ?)`, uid, ruleID, date, Now())
	return err
}

// KindMinutes is the minutes of one kind ("any" = all) a person practised on a day.
func (s *Store) KindMinutes(ctx context.Context, uid int64, kind, date string) float64 {
	q := `SELECT COALESCE(SUM(duration_s), 0) FROM practices WHERE user_id = ? AND date = ?`
	args := []any{uid, date}
	if kind != "" && kind != "any" {
		q += ` AND kind = ?`
		args = append(args, kind)
	}
	var secs int
	_ = s.DB.QueryRowContext(ctx, q, args...).Scan(&secs)
	return float64(secs) / 60
}

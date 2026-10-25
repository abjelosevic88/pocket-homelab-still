package store

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

const practiceCols = `id, user_id, kind, technique, title, started_at, ended_at, date, duration_s, planned_s, completed, cycles,
	calm_before, calm_after, hr_before, hr_after, note, tags, program, lesson, source, external_id,
	health_synced_at, health_error, habit_marked_at, habit_error, created_at, updated_at`

func scanPractice(row interface{ Scan(...any) error }) (*Practice, error) {
	var p Practice
	var cb, ca sql.NullInt64
	var hb, ha sql.NullFloat64
	var done int
	if err := row.Scan(&p.ID, &p.UserID, &p.Kind, &p.Technique, &p.Title, &p.StartedAt, &p.EndedAt, &p.Date, &p.DurationS, &p.PlannedS, &done, &p.Cycles,
		&cb, &ca, &hb, &ha, &p.Note, &p.Tags, &p.Program, &p.Lesson, &p.Source, &p.ExternalID,
		&p.HealthSyncedAt, &p.HealthError, &p.HabitMarkedAt, &p.HabitError, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, notFound(err)
	}
	p.Completed = done == 1
	if cb.Valid {
		v := int(cb.Int64)
		p.CalmBefore = &v
	}
	if ca.Valid {
		v := int(ca.Int64)
		p.CalmAfter = &v
	}
	if hb.Valid {
		p.HRBefore = &hb.Float64
	}
	if ha.Valid {
		p.HRAfter = &ha.Float64
	}
	return &p, nil
}

func intPtr(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func floatPtr(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

// ParseTime reads RFC3339 (with or without fraction) or a local "2006-01-02T15:04".
func ParseTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, f := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02 15:04"} {
		var t time.Time
		var err error
		if strings.Contains(f, "Z07") {
			t, err = time.Parse(f, s)
		} else {
			t, err = time.ParseInLocation(f, s, time.Local)
		}
		if err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// normalize fills the derived fields: date from the start, end from the length.
func (p *Practice) normalize() {
	if t, ok := ParseTime(p.StartedAt); ok {
		p.StartedAt = t.Format(time.RFC3339)
		p.Date = t.Format("2006-01-02")
		if p.EndedAt == "" && p.DurationS > 0 {
			p.EndedAt = t.Add(time.Duration(p.DurationS) * time.Second).Format(time.RFC3339)
		}
	}
	p.Tags = JoinTags(p.Tags)
	if p.Source == "" {
		p.Source = "app"
	}
}

// CreatePractice saves a practice. With an external id the same practice sent
// twice (a retry from the player, a linked app) updates instead of doubling.
func (s *Store) CreatePractice(ctx context.Context, uid int64, p *Practice) error {
	p.normalize()
	now := Now()
	res, err := s.DB.ExecContext(ctx, `INSERT INTO practices (user_id, kind, technique, title, started_at, ended_at, date, duration_s, planned_s, completed, cycles,
		calm_before, calm_after, hr_before, hr_after, note, tags, program, lesson, source, external_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, source, external_id) WHERE external_id <> '' DO UPDATE SET kind = excluded.kind, technique = excluded.technique,
		title = excluded.title, started_at = excluded.started_at, ended_at = excluded.ended_at, date = excluded.date, duration_s = excluded.duration_s,
		planned_s = excluded.planned_s, completed = excluded.completed, cycles = excluded.cycles, calm_before = excluded.calm_before,
		calm_after = excluded.calm_after, hr_before = excluded.hr_before, hr_after = excluded.hr_after, note = excluded.note, tags = excluded.tags,
		updated_at = excluded.updated_at, health_synced_at = ''`,
		uid, p.Kind, p.Technique, p.Title, p.StartedAt, p.EndedAt, p.Date, p.DurationS, p.PlannedS, boolInt(p.Completed), p.Cycles,
		intPtr(p.CalmBefore), intPtr(p.CalmAfter), floatPtr(p.HRBefore), floatPtr(p.HRAfter), p.Note, p.Tags, p.Program, p.Lesson, p.Source, p.ExternalID, now, now)
	if err != nil {
		return err
	}
	if p.ExternalID != "" {
		return s.DB.QueryRowContext(ctx, `SELECT id FROM practices WHERE user_id = ? AND source = ? AND external_id = ?`, uid, p.Source, p.ExternalID).Scan(&p.ID)
	}
	p.ID, _ = res.LastInsertId()
	return nil
}

// GetPractice loads one of a person's practices.
func (s *Store) GetPractice(ctx context.Context, uid, id int64) (*Practice, error) {
	return scanPractice(s.DB.QueryRowContext(ctx, `SELECT `+practiceCols+` FROM practices WHERE user_id = ? AND id = ?`, uid, id))
}

// UpdatePractice saves the editable fields; Pocket Health learns the new length.
func (s *Store) UpdatePractice(ctx context.Context, uid int64, p *Practice) error {
	p.normalize()
	res, err := s.DB.ExecContext(ctx, `UPDATE practices SET kind = ?, technique = ?, title = ?, started_at = ?, ended_at = ?, date = ?, duration_s = ?, planned_s = ?,
		completed = ?, cycles = ?, calm_before = ?, calm_after = ?, hr_before = ?, hr_after = ?, note = ?, tags = ?, health_synced_at = '', health_error = '', updated_at = ?
		WHERE user_id = ? AND id = ?`,
		p.Kind, p.Technique, p.Title, p.StartedAt, p.EndedAt, p.Date, p.DurationS, p.PlannedS, boolInt(p.Completed), p.Cycles,
		intPtr(p.CalmBefore), intPtr(p.CalmAfter), floatPtr(p.HRBefore), floatPtr(p.HRAfter), p.Note, p.Tags, Now(), uid, p.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeletePractice removes a practice.
func (s *Store) DeletePractice(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM practices WHERE user_id = ? AND id = ?`, uid, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// PracticeFilter narrows a list.
type PracticeFilter struct {
	From, To  string // dates, inclusive
	Kind      string
	Technique string
	Program   string
	Source    string
	Q         string
	Notes     bool // only practices with a note
	Limit     int
	Offset    int
	Asc       bool
}

func (f PracticeFilter) where(uid int64) (string, []any) {
	w := []string{"user_id = ?"}
	args := []any{uid}
	add := func(cond string, v any) {
		w = append(w, cond)
		args = append(args, v)
	}
	if f.From != "" {
		add("date >= ?", f.From)
	}
	if f.To != "" {
		add("date <= ?", f.To)
	}
	if f.Kind != "" {
		add("kind = ?", f.Kind)
	}
	if f.Technique != "" {
		add("technique = ?", f.Technique)
	}
	if f.Program != "" {
		add("program = ?", f.Program)
	}
	if f.Source != "" {
		add("source = ?", f.Source)
	}
	if f.Notes {
		w = append(w, "note <> ''")
	}
	if q := strings.TrimSpace(f.Q); q != "" {
		like := "%" + strings.ToLower(q) + "%"
		w = append(w, "(lower(note) LIKE ? OR lower(title) LIKE ? OR lower(tags) LIKE ?)")
		args = append(args, like, like, like)
	}
	return strings.Join(w, " AND "), args
}

// ListPractices lists a person's practices, newest first, and how many match.
func (s *Store) ListPractices(ctx context.Context, uid int64, f PracticeFilter) ([]Practice, int, error) {
	where, args := f.where(uid)
	var total int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM practices WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := "DESC"
	if f.Asc {
		order = "ASC"
	}
	q := `SELECT ` + practiceCols + ` FROM practices WHERE ` + where + ` ORDER BY started_at ` + order + `, id ` + order
	if f.Limit > 0 {
		q += ` LIMIT ? OFFSET ?`
		args = append(args, f.Limit, f.Offset)
	}
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Practice{}
	for rows.Next() {
		p, err := scanPractice(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *p)
	}
	return out, total, rows.Err()
}

// FirstDate is the day of a person's first practice ("" when none).
func (s *Store) FirstDate(ctx context.Context, uid int64) string {
	var d sql.NullString
	_ = s.DB.QueryRowContext(ctx, `SELECT MIN(date) FROM practices WHERE user_id = ?`, uid).Scan(&d)
	return d.String
}

// DayMinutes is the minutes practised per day in a range (only days with practice).
func (s *Store) DayMinutes(ctx context.Context, uid int64, from, to string) (map[string]float64, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT date, SUM(duration_s) FROM practices WHERE user_id = ? AND date >= ? AND date <= ? GROUP BY date`, uid, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]float64{}
	for rows.Next() {
		var d string
		var secs int
		if err := rows.Scan(&d, &secs); err != nil {
			return nil, err
		}
		out[d] = float64(secs) / 60
	}
	return out, rows.Err()
}

// PracticeDates lists every day with practice, oldest first (for streaks).
func (s *Store) PracticeDates(ctx context.Context, uid int64) ([]string, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT DISTINCT date FROM practices WHERE user_id = ? ORDER BY date`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Totals is a person's lifetime numbers.
type Totals struct {
	Sessions int     `json:"sessions"`
	Minutes  float64 `json:"minutes"`
	Days     int     `json:"days"`
	Notes    int     `json:"notes"`
	LongestS int     `json:"longest_s"`
}

// LifetimeTotals sums everything a person has practised.
func (s *Store) LifetimeTotals(ctx context.Context, uid int64) (Totals, error) {
	var t Totals
	var secs, notes, longest sql.NullInt64
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(1), SUM(duration_s), COUNT(DISTINCT date), SUM(note <> ''), MAX(duration_s) FROM practices WHERE user_id = ?`, uid).
		Scan(&t.Sessions, &secs, &t.Days, &notes, &longest)
	if err != nil {
		return t, err
	}
	t.Notes = int(notes.Int64)
	t.Minutes = float64(secs.Int64) / 60
	t.LongestS = int(longest.Int64)
	return t, nil
}

// TechniquesUsed lists the distinct techniques a person has finished.
func (s *Store) TechniquesUsed(ctx context.Context, uid int64) (map[string]bool, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT DISTINCT technique FROM practices WHERE user_id = ? AND completed = 1`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out[t] = true
	}
	return out, rows.Err()
}

// SetHealthSync records how writing a practice to Pocket Health went.
func (s *Store) SetHealthSync(ctx context.Context, uid, id int64, syncedAt, errText string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE practices SET health_synced_at = ?, health_error = ? WHERE user_id = ? AND id = ?`, syncedAt, errText, uid, id)
	return err
}

// SetHabitMark records that the habit rules looked at a practice.
func (s *Store) SetHabitMark(ctx context.Context, uid, id int64, markedAt, errText string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE practices SET habit_marked_at = ?, habit_error = ? WHERE user_id = ? AND id = ?`, markedAt, errText, uid, id)
	return err
}

// PendingHealth lists recent practices not yet written to Pocket Health.
func (s *Store) PendingHealth(ctx context.Context, uid int64, since string) ([]Practice, error) {
	ps, _, err := s.listWhere(ctx, uid, `health_synced_at = '' AND date >= ?`, since)
	return ps, err
}

// PendingHabits lists recent practices the habit rules have not looked at.
func (s *Store) PendingHabits(ctx context.Context, uid int64, since string) ([]Practice, error) {
	ps, _, err := s.listWhere(ctx, uid, `habit_marked_at = '' AND date >= ?`, since)
	return ps, err
}

func (s *Store) listWhere(ctx context.Context, uid int64, cond string, args ...any) ([]Practice, int, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+practiceCols+` FROM practices WHERE user_id = ? AND `+cond+` ORDER BY started_at`, append([]any{uid}, args...)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Practice{}
	for rows.Next() {
		p, err := scanPractice(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *p)
	}
	return out, len(out), rows.Err()
}

// Tags lists a person's tags with how often each is used.
func (s *Store) Tags(ctx context.Context, uid int64) (map[string]int, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT tags FROM practices WHERE user_id = ? AND tags <> ''`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		for _, x := range SplitTags(t) {
			out[x]++
		}
	}
	return out, rows.Err()
}

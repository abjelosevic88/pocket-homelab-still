package server

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-still/internal/content"
	"github.com/abjelosevic88/pocket-homelab-still/internal/store"
)

// practiceInput is what the player, the "add a practice" form and linked apps send.
type practiceInput struct {
	Kind       string   `json:"kind"`
	Technique  string   `json:"technique"`
	Title      string   `json:"title"`
	StartedAt  string   `json:"started_at"`
	DurationS  int      `json:"duration_s"`
	PlannedS   int      `json:"planned_s"`
	Completed  *bool    `json:"completed"`
	Cycles     int      `json:"cycles"`
	CalmBefore *int     `json:"calm_before"`
	CalmAfter  *int     `json:"calm_after"`
	HRBefore   *float64 `json:"hr_before"`
	HRAfter    *float64 `json:"hr_after"`
	Note       string   `json:"note"`
	Tags       string   `json:"tags"`
	Program    string   `json:"program"`
	Lesson     string   `json:"lesson"`
	Source     string   `json:"source"`
	ExternalID string   `json:"external_id"`
}

func validCalm(v *int) bool { return v == nil || (*v >= 1 && *v <= 5) }

func validHR(v *float64) bool { return v == nil || (*v >= 25 && *v <= 250) }

// apply checks the input and copies it onto p.
func (in *practiceInput) apply(p *store.Practice) error {
	in.Kind = strings.TrimSpace(in.Kind)
	if in.Kind == "" {
		in.Kind = "meditation"
		if _, ok := content.PatternByKey(in.Technique); ok {
			in.Kind = "breath"
		} else if st, ok := content.StyleByKey(in.Technique); ok {
			in.Kind = st.Kind
		}
	}
	if !content.ValidKind(in.Kind) {
		return fmt.Errorf("unknown kind %q", in.Kind)
	}
	if in.DurationS < 0 || in.DurationS > 6*3600 || in.PlannedS < 0 || in.PlannedS > 6*3600 {
		return fmt.Errorf("a practice lasts up to six hours")
	}
	if !validCalm(in.CalmBefore) || !validCalm(in.CalmAfter) {
		return fmt.Errorf("how you feel is 1 (tense) to 5 (calm)")
	}
	if !validHR(in.HRBefore) || !validHR(in.HRAfter) {
		return fmt.Errorf("heart rate is 25 to 250 bpm")
	}
	start := time.Now().Add(-time.Duration(in.DurationS) * time.Second)
	if in.StartedAt != "" {
		t, ok := store.ParseTime(in.StartedAt)
		if !ok {
			return fmt.Errorf("started_at must be a date and time")
		}
		start = t
	}
	if start.After(time.Now().Add(5 * time.Minute)) {
		return fmt.Errorf("a practice cannot start in the future")
	}
	if len(in.Note) > 10000 || len(in.Title) > 200 || len(in.Tags) > 500 {
		return fmt.Errorf("the note, title or tags are too long")
	}
	p.Kind, p.Technique, p.Title = in.Kind, strings.TrimSpace(in.Technique), strings.TrimSpace(in.Title)
	p.StartedAt, p.EndedAt = start.Format(time.RFC3339), ""
	p.DurationS, p.PlannedS, p.Cycles = in.DurationS, in.PlannedS, in.Cycles
	p.Completed = in.Completed == nil || *in.Completed
	p.CalmBefore, p.CalmAfter, p.HRBefore, p.HRAfter = in.CalmBefore, in.CalmAfter, in.HRBefore, in.HRAfter
	p.Note, p.Tags = strings.TrimSpace(in.Note), in.Tags
	if p.Title == "" {
		p.Title = content.TechniqueName(p.Technique)
		if p.Title == "" {
			p.Title = content.KindName(p.Kind)
		}
	}
	return nil
}

// forLink hides what a linked app never sees: the person's notes.
func forLink(ctx context.Context, ps []store.Practice) []store.Practice {
	if linkApp(ctx) == "" {
		return ps
	}
	for i := range ps {
		ps[i].Note = ""
	}
	return ps
}

func (s *Server) handleListPractices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	f := store.PracticeFilter{From: q.Get("from"), To: q.Get("to"), Kind: q.Get("kind"), Technique: q.Get("technique"), Program: q.Get("program"),
		Source: q.Get("source"), Q: q.Get("q"), Notes: queryBool(r, "notes"), Limit: queryInt(r, "limit"), Offset: queryInt(r, "offset")}
	if !validDate(f.From) || !validDate(f.To) {
		writeErr(w, http.StatusBadRequest, "from and to are dates (YYYY-MM-DD)")
		return
	}
	if f.Limit <= 0 || f.Limit > 1000 {
		f.Limit = 100
	}
	if linkApp(ctx) != "" {
		f.Q, f.Notes = "", false
	}
	ps, total, err := s.store.ListPractices(ctx, actorID(ctx), f)
	if err != nil {
		s.fail(w, err, "practices")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"practices": forLink(ctx, ps), "total": total})
}

func (s *Server) handleGetPractice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p, err := s.store.GetPractice(ctx, actorID(ctx), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "practice")
		return
	}
	writeJSON(w, http.StatusOK, forLink(ctx, []store.Practice{*p})[0])
}

func (s *Server) handleCreatePractice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in practiceInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	p := store.Practice{}
	if err := in.apply(&p); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	p.Source = "app"
	if in.Source == "manual" {
		p.Source = "manual"
	}
	if app := linkApp(ctx); app != "" {
		p.Source = app
	}
	p.ExternalID = strings.TrimSpace(in.ExternalID)
	if len(p.ExternalID) > 120 {
		writeErr(w, http.StatusBadRequest, "external_id is at most 120 characters")
		return
	}
	// a lesson's practice: only a real lesson of a real program counts
	if in.Program != "" || in.Lesson != "" {
		prog, ok := content.ProgramByKey(in.Program)
		if !ok {
			writeErr(w, http.StatusBadRequest, "unknown program")
			return
		}
		if _, ok := prog.FindLesson(in.Lesson); !ok {
			writeErr(w, http.StatusBadRequest, "unknown lesson")
			return
		}
		p.Program, p.Lesson = in.Program, in.Lesson
	}
	if err := s.store.CreatePractice(ctx, uid, &p); err != nil {
		s.fail(w, err, "save practice")
		return
	}
	s.saved.Add(1)
	out := s.afterPractice(ctx, uid, &p)
	saved, _ := s.store.GetPractice(ctx, uid, p.ID)
	if saved != nil {
		out["practice"] = saved
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleUpdatePractice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	cur, err := s.store.GetPractice(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "practice")
		return
	}
	if app := linkApp(ctx); app != "" && cur.Source != app {
		writeErr(w, http.StatusForbidden, "a linked app may only change the practices it logged")
		return
	}
	in := practiceInput{Kind: cur.Kind, Technique: cur.Technique, Title: cur.Title, StartedAt: cur.StartedAt, DurationS: cur.DurationS, PlannedS: cur.PlannedS,
		Completed: &cur.Completed, Cycles: cur.Cycles, CalmBefore: cur.CalmBefore, CalmAfter: cur.CalmAfter, HRBefore: cur.HRBefore, HRAfter: cur.HRAfter, Note: cur.Note, Tags: cur.Tags}
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if keys["title"] && strings.TrimSpace(in.Title) == "" {
		in.Title = ""
	}
	oldStart, oldMin := cur.StartedAt, cur.Minutes()
	if err := in.apply(cur); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.UpdatePractice(ctx, uid, cur); err != nil {
		s.fail(w, err, "save practice")
		return
	}
	// a moved start leaves the old sample in Pocket Health: zero it
	if oldStart != cur.StartedAt && oldMin > 0 {
		s.zeroHealthSoon(uid, oldStart)
	}
	out := s.afterPractice(ctx, uid, cur)
	saved, _ := s.store.GetPractice(ctx, uid, cur.ID)
	out["practice"] = saved
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeletePractice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	cur, err := s.store.GetPractice(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "practice")
		return
	}
	if app := linkApp(ctx); app != "" && cur.Source != app {
		writeErr(w, http.StatusForbidden, "a linked app may only delete the practices it logged")
		return
	}
	if err := s.store.DeletePractice(ctx, uid, cur.ID); err != nil {
		s.fail(w, err, "delete practice")
		return
	}
	if cur.HealthSyncedAt != "" {
		s.zeroHealthSoon(uid, cur.StartedAt)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTags(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tags, err := s.store.Tags(ctx, actorID(ctx))
	if err != nil {
		s.fail(w, err, "tags")
		return
	}
	type tc struct {
		Tag   string `json:"tag"`
		Count int    `json:"count"`
	}
	out := make([]tc, 0, len(tags))
	for t, n := range tags {
		out = append(out, tc{t, n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Tag < out[j].Tag
	})
	writeJSON(w, http.StatusOK, out)
}

// afterPractice does everything a saved practice sets off: the lesson it was
// for, milestones, the Core events, Pocket Health and Pocket Habits (the
// last two in the background; the janitor retries them). It returns what the
// player shows on its "well done" screen.
func (s *Server) afterPractice(ctx context.Context, uid int64, p *store.Practice) map[string]any {
	out := map[string]any{}
	if p.Program != "" && p.Lesson != "" && p.Completed {
		if fresh, err := s.store.CompleteLesson(ctx, uid, p.Program, p.Lesson, p.ID); err == nil && fresh {
			out["lesson_completed"] = true
			prog, _ := content.ProgramByKey(p.Program)
			l, _ := prog.FindLesson(p.Lesson)
			s.publish(ctx, uid, "still.lesson_completed", p.Program+"/"+p.Lesson, map[string]any{"program": p.Program, "program_name": prog.Name, "lesson": p.Lesson, "title": l.Lesson.Title, "stage": l.Stage})
			if pr, err := s.programProgress(ctx, uid, prog); err == nil {
				out["program"] = pr
				if pr.Done == pr.Total {
					s.publish(ctx, uid, "still.program_completed", p.Program, map[string]any{"program": p.Program, "name": prog.Name})
				}
			}
		}
	}
	unlocked := s.checkAchievements(ctx, uid, p)
	out["unlocked"] = unlocked
	if p.Completed && p.Source != "import" {
		s.publish(ctx, uid, "still.practice_completed", fmt.Sprint(p.ID), map[string]any{"id": p.ID, "kind": p.Kind, "technique": p.Technique, "title": p.Title,
			"minutes": round1(p.Minutes()), "date": p.Date, "started_at": p.StartedAt, "calm_before": p.CalmBefore, "calm_after": p.CalmAfter})
	}
	today, _ := s.store.DayMinutes(ctx, uid, p.Date, p.Date)
	st, _ := s.store.GetSettings(ctx, uid)
	cur, _ := s.streaks(ctx, uid)
	out["day_minutes"] = round1(today[p.Date])
	out["goal_min"] = st.DailyGoalMin
	out["streak"] = cur
	go func(uid int64, date string) {
		bg, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		s.syncHealth(bg, uid, date)
		s.tickPendingHabits(bg, uid, date)
	}(uid, p.Date)
	return out
}

func round1(v float64) float64 { return float64(int(v*10+0.5)) / 10 }

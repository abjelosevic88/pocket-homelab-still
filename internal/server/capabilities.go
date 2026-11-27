package server

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-still/internal/content"
	"github.com/abjelosevic88/pocket-homelab-still/internal/store"
)

// What Pocket Search, Pocket Day and Pocket Hub ask Still (the capabilities
// value in links.go): /api/v1/pocket/search, /day/{date} and /widget, in the
// shape every Pocket app answers in. All three only read, as the request's
// person. The day is the one GET /day/{date} gives; through a link the
// person's notes stay out of it, as they do there.

// Pages that open things in the web app (web/src/main.tsx). Meditation styles
// and timers have no page of their own: they open on Meditate.
func patternURL(key string) string { return "/breathe/" + key }
func presetURL(p store.Preset) string {
	if p.Kind == "breath" {
		return "/breathe/preset:" + strconv.FormatInt(p.ID, 10)
	}
	return "/meditate"
}
func programURL(key string) string         { return "/path/" + key }
func lessonURL(program, key string) string { return "/path/" + program + "/" + key }
func sessionURL(id int64) string           { return "/journal/" + strconv.FormatInt(id, 10) }

const meditateURL = "/meditate"

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

// lengthWords says how long a practice runs: "5 min", "4 rounds".
func lengthWords(minutes float64, cycles int) string {
	switch {
	case cycles > 0:
		return plural(cycles, "round", "rounds")
	case minutes > 0:
		return fmtMinutes(minutes)
	}
	return "Open-ended"
}

// goalWords is the whole minutes of a day, of the goal when there is one:
// "8 of 10", "8". A day short of its goal never reads as reaching it.
func goalWords(minutes float64, goal int) string {
	m := int(math.Round(minutes))
	if m == 0 && minutes > 0 {
		m = 1
	}
	if goal <= 0 {
		return strconv.Itoa(m)
	}
	if m >= goal && minutes+0.01 < float64(goal) {
		m = goal - 1
	}
	return fmt.Sprintf("%d of %d", m, goal)
}

// ---------- search ----------

// searchWords are the words of a query, lower case; words without a letter
// or digit are left out, so "??" finds nothing.
func searchWords(q string) []string {
	var out []string
	for _, w := range strings.Fields(strings.ToLower(q)) {
		if strings.IndexFunc(w, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0 {
			out = append(out, w)
		}
	}
	return out
}

// nameScore is how well a name has every word: 3 for the name itself, 2 when
// it starts with the words, 1 when it only has them, 0 when a word is missing.
func nameScore(name string, words []string) float64 {
	low := strings.ToLower(name)
	for _, w := range words {
		if !strings.Contains(low, w) {
			return 0
		}
	}
	q := strings.Join(words, " ")
	switch {
	case low == q:
		return 3
	case strings.HasPrefix(low, q):
		return 2
	}
	return 1
}

// The person's own things come before what Still ships with.
const (
	scorePreset  = 30
	scoreSession = 20
	scoreBuiltIn = 10
)

// pocketSearch answers Pocket Search: the person's own breathing patterns and
// timers by name, their sessions by the note they wrote (newest first; the
// note itself is never sent back), then the built-in breathing patterns,
// meditation styles, courses and lessons by name.
func (s *Server) pocketSearch(r *http.Request, q string, limit int) ([]core.SearchItem, error) {
	words := searchWords(q)
	if len(words) == 0 {
		return nil, nil
	}
	ctx := r.Context()
	uid := actorID(ctx)
	var out []core.SearchItem

	presets, err := s.store.ListPresets(ctx, uid, "")
	if err != nil {
		return nil, err
	}
	for _, p := range presets {
		sc := nameScore(p.Name, words)
		if sc == 0 {
			continue
		}
		snippet := "Your meditation timer"
		if p.Kind == "breath" {
			var c content.BreathConfig
			_ = json.Unmarshal(p.Config, &c)
			snippet = lengthWords(c.Minutes, c.Cycles) + " · Your breathing pattern"
		} else {
			var c content.TimerConfig
			_ = json.Unmarshal(p.Config, &c)
			snippet = lengthWords(c.Minutes, 0) + " · " + snippet
		}
		out = append(out, core.SearchItem{ID: "practice:preset:" + strconv.FormatInt(p.ID, 10), Title: p.Name, Snippet: snippet, Kind: "practice",
			Date: p.UpdatedAt, URL: presetURL(p), Score: scorePreset + sc})
	}

	sessions, err := s.store.SearchNotes(ctx, uid, words, limit)
	if err != nil {
		return nil, err
	}
	for _, p := range sessions {
		title := p.Title
		if d, err := time.Parse("2006-01-02", p.Date); err == nil {
			title += ", " + d.Format("2 Jan 2006")
		}
		out = append(out, core.SearchItem{ID: "session:" + strconv.FormatInt(p.ID, 10), Title: title, Snippet: fmtMinutes(p.Minutes()) + " · " + content.KindName(p.Kind),
			Kind: "session", Date: p.StartedAt, URL: sessionURL(p.ID), Score: scoreSession})
	}

	for _, p := range content.Patterns {
		if sc := nameScore(p.Name, words); sc > 0 {
			out = append(out, core.SearchItem{ID: "practice:" + p.Key, Title: p.Name, Snippet: lengthWords(p.Config.Minutes, p.Config.Cycles) + " · Breathing pattern",
				Kind: "practice", URL: patternURL(p.Key), Score: scoreBuiltIn + sc})
		}
	}
	for _, st := range content.Styles {
		if sc := nameScore(st.Name, words); sc > 0 {
			out = append(out, core.SearchItem{ID: "practice:" + st.Key, Title: st.Name, Snippet: lengthWords(st.Minutes, 0) + " · " + content.KindName(st.Kind),
				Kind: "practice", URL: meditateURL, Score: scoreBuiltIn + sc})
		}
	}
	for _, p := range content.Programs {
		lessons := p.Lessons()
		if sc := nameScore(p.Name, words); sc > 0 {
			kind := "Course"
			if p.Kind == "path" {
				kind = "Path in " + plural(len(p.Stages), "stage", "stages")
			}
			out = append(out, core.SearchItem{ID: "course:" + p.Key, Title: p.Name, Snippet: plural(len(lessons), "lesson", "lessons") + " · " + kind,
				Kind: "course", URL: programURL(p.Key), Score: scoreBuiltIn + sc})
		}
		for _, l := range lessons {
			if sc := nameScore(l.Lesson.Title, words); sc > 0 {
				out = append(out, core.SearchItem{ID: "lesson:" + p.Key + "/" + l.Lesson.Key, Title: l.Lesson.Title,
					Snippet: lengthWords(l.Lesson.Practice.Minutes, 0) + " · Lesson " + strconv.Itoa(l.Index+1) + " of " + p.Name,
					Kind:    "lesson", URL: lessonURL(p.Key, l.Lesson.Key), Score: scoreBuiltIn + sc})
			}
		}
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ---------- day ----------

// clock is a moment's wall-clock time, in tz when the caller gave one, else
// where it was recorded.
func clock(at string, tz *time.Location) string {
	t, ok := store.ParseTime(at)
	if !ok {
		return ""
	}
	if tz != nil {
		t = t.In(tz)
	}
	return t.Format("15:04")
}

// pocketDay answers Pocket Day with the day GET /day/{date} gives: one item
// for each session, then the lessons finished and the milestones reached.
// The caller's time zone decides the clock times and on which day a lesson or
// a milestone falls; sessions keep the day they were recorded on.
func (s *Server) pocketDay(r *http.Request, date string, tz *time.Location) (*core.Day, error) {
	ctx := r.Context()
	loc := time.Local
	if tz != nil {
		loc = tz
	}
	v, err := s.dayOf(ctx, actorID(ctx), date, loc)
	if err != nil {
		return nil, err
	}
	day := &core.Day{Date: date, Summary: v.Summary}
	for _, p := range v.Practices {
		sub := p.KindName
		if p.Lesson != "" {
			sub += " · " + p.Lesson
		}
		day.Items = append(day.Items, core.DayItem{ID: "session:" + strconv.FormatInt(p.ID, 10), Time: clock(p.StartedAt, tz), End: clock(p.EndedAt, tz),
			Title: p.Title, Sub: sub, Kind: "session", Minutes: p.Minutes, URL: sessionURL(p.ID), Note: p.Note})
	}
	for _, l := range v.Lessons {
		sub := l.Program
		if l.programKey == "path" {
			sub += " · " + l.Stage
		}
		day.Items = append(day.Items, core.DayItem{ID: "lesson:" + l.programKey + "/" + l.lessonKey, Time: l.at.In(loc).Format("15:04"),
			Title: l.Title, Sub: sub, Kind: "lesson", Done: true, URL: lessonURL(l.programKey, l.lessonKey)})
	}
	for _, m := range v.Milestones {
		day.Items = append(day.Items, core.DayItem{ID: "milestone:" + m.key, Time: m.at.In(loc).Format("15:04"),
			Title: m.name, Sub: "Milestone reached", Kind: "milestone", Done: true, URL: "/milestones"})
	}
	if len(day.Items) == 0 {
		return nil, nil
	}
	if len(v.Practices) > 0 {
		day.Metrics = append(day.Metrics, core.Metric{Label: "Minutes", Value: goalWords(v.Minutes, v.GoalMin)})
	}
	if v.Streak > 0 {
		day.Metrics = append(day.Metrics, core.Metric{Label: "Streak", Value: plural(v.Streak, "day", "days")})
	}
	return day, nil
}

// ---------- widget ----------

// pocketWidget answers Pocket Hub with today as the dashboard has it: the
// minutes against the daily goal, the streak, and one thing to practise next
// (the next lesson of the course worked on last, else the practice used most
// in the last 90 days, else the next lesson of the Path).
func (s *Server) pocketWidget(r *http.Request) (*core.WidgetData, error) {
	ctx := r.Context()
	uid := actorID(ctx)
	st, err := s.store.GetSettings(ctx, uid)
	if err != nil {
		return nil, err
	}
	today := store.Today()
	mins, err := s.store.DayMinutes(ctx, uid, today, today)
	if err != nil {
		return nil, err
	}
	cur, _ := s.streaks(ctx, uid)
	w := &core.WidgetData{URL: "/"}

	var parts []string
	if mins[today] > 0 {
		words := goalWords(mins[today], st.DailyGoalMin)
		minutes := "minutes"
		if words == "1" {
			minutes = "minute"
		}
		parts = append(parts, words+" "+minutes+" today")
	} else {
		parts = append(parts, "Nothing yet today")
	}
	if cur > 1 {
		parts = append(parts, plural(cur, "day", "days")+" in a row")
	}
	w.Line = strings.Join(parts, ", ")
	w.Metrics = append(w.Metrics, core.Metric{Label: "Today", Value: fmtMinutes(mins[today])})
	if st.DailyGoalMin > 0 {
		w.Metrics = append(w.Metrics, core.Metric{Label: "Goal", Value: fmtMinutes(float64(st.DailyGoalMin))})
	}
	w.Metrics = append(w.Metrics, core.Metric{Label: "Streak", Value: plural(cur, "day", "days")})

	if it := s.nextUp(r); it != nil {
		w.Items = append(w.Items, *it)
	}
	return w, nil
}

// nextUp is the widget's one suggestion.
func (s *Server) nextUp(r *http.Request) *core.WidgetItem {
	ctx := r.Context()
	uid := actorID(ctx)
	starts, _ := s.store.ProgramStarts(ctx, uid)
	var next, path *lessonView
	var nextProg, pathProg string
	lastAt := ""
	for _, p := range content.Programs {
		v, err := s.programProgress(ctx, uid, p)
		if err != nil || v.Next == nil {
			continue
		}
		if p.Kind == "path" && path == nil {
			path, pathProg = v.Next, p.Name
		}
		if active := firstNonEmpty(v.LastAt, starts[p.Key]); active != "" && active > lastAt {
			next, nextProg, lastAt = v.Next, p.Name, active
		}
	}
	lesson := func(l *lessonView, prog string) *core.WidgetItem {
		return &core.WidgetItem{ID: "lesson:" + l.Program + "/" + l.Key, Title: l.Title, Detail: "Next lesson · " + prog, URL: lessonURL(l.Program, l.Key)}
	}
	if next != nil {
		return lesson(next, nextProg)
	}
	if key := s.store.TopTechnique(ctx, uid, store.AddDays(store.Today(), -90)); key != "" {
		const detail = "Your most used practice"
		if p, ok := content.PatternByKey(key); ok {
			return &core.WidgetItem{ID: "practice:" + key, Title: p.Name, Detail: detail, URL: patternURL(key)}
		}
		if st, ok := content.StyleByKey(key); ok {
			return &core.WidgetItem{ID: "practice:" + key, Title: st.Name, Detail: detail, URL: meditateURL}
		}
		if id, err := strconv.ParseInt(strings.TrimPrefix(key, "preset:"), 10, 64); err == nil && strings.HasPrefix(key, "preset:") {
			if p, err := s.store.GetPreset(ctx, uid, id); err == nil {
				return &core.WidgetItem{ID: "practice:" + key, Title: p.Name, Detail: detail, URL: presetURL(*p)}
			}
		}
	}
	if path != nil {
		return lesson(path, pathProg)
	}
	return nil
}

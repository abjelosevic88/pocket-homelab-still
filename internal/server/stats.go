package server

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-still/internal/content"
	"github.com/abjelosevic88/pocket-homelab-still/internal/store"
)

// ---- streaks ----

// streakOf counts consecutive days ending today, or yesterday when today
// has nothing yet (the day isn't over), and the longest run ever.
func streakOf(dates []string, today string) (cur, best int) {
	if len(dates) == 0 {
		return 0, 0
	}
	have := make(map[string]bool, len(dates))
	for _, d := range dates {
		have[d] = true
	}
	run, prev := 0, ""
	for _, d := range dates { // sorted, distinct
		if prev != "" && store.AddDays(prev, 1) == d {
			run++
		} else {
			run = 1
		}
		if run > best {
			best = run
		}
		prev = d
	}
	day := today
	if !have[day] {
		day = store.AddDays(today, -1)
	}
	for have[day] {
		cur++
		day = store.AddDays(day, -1)
	}
	return cur, best
}

// streaks is a person's current and longest streak of days with practice.
func (s *Server) streaks(ctx context.Context, uid int64) (int, int) {
	dates, err := s.store.PracticeDates(ctx, uid)
	if err != nil {
		return 0, 0
	}
	return streakOf(dates, store.Today())
}

// goalStreaks counts days that reached the daily goal.
func (s *Server) goalStreaks(ctx context.Context, uid int64, goal int) (int, int, []string) {
	first := s.store.FirstDate(ctx, uid)
	if first == "" || goal <= 0 {
		return 0, 0, nil
	}
	mins, err := s.store.DayMinutes(ctx, uid, first, store.Today())
	if err != nil {
		return 0, 0, nil
	}
	var met []string
	for d, m := range mins {
		if m+0.01 >= float64(goal) {
			met = append(met, d)
		}
	}
	sort.Strings(met)
	cur, best := streakOf(met, store.Today())
	return cur, best, met
}

// ---- ranges ----

func rangeDays(r string) int {
	switch r {
	case "w", "7", "week":
		return 7
	case "2w", "14":
		return 14
	case "m", "30", "month", "":
		return 30
	case "3m", "90":
		return 90
	case "6m", "180":
		return 180
	case "y", "365", "year":
		return 365
	case "all":
		return 100000
	}
	return 30
}

func weekStartOf(date string, weekStart int) string {
	t, err := store.ParseDate(date)
	if err != nil {
		return date
	}
	wd := int(t.Weekday()) // 0 = Sunday
	if wd == 0 {
		wd = 7
	}
	if weekStart < 1 || weekStart > 7 {
		weekStart = 1
	}
	back := (wd - weekStart + 7) % 7
	return t.AddDate(0, 0, -back).Format("2006-01-02")
}

func isoWeekday(date string) int {
	t, err := store.ParseDate(date)
	if err != nil {
		return 1
	}
	wd := int(t.Weekday())
	if wd == 0 {
		return 7
	}
	return wd
}

func mean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	t := 0.0
	for _, x := range v {
		t += x
	}
	return t / float64(len(v))
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func pct(a, b int) int {
	if b == 0 {
		return 0
	}
	return int(math.Round(float64(a) * 100 / float64(b)))
}

func partOfDay(h int) string {
	switch {
	case h >= 5 && h < 12:
		return "morning"
	case h >= 12 && h < 17:
		return "afternoon"
	case h >= 17 && h < 22:
		return "evening"
	}
	return "night"
}

func startHour(p store.Practice) int {
	if t, err := time.Parse(time.RFC3339, p.StartedAt); err == nil {
		return t.Hour()
	}
	return 0
}

// ---- the statistics page ----

type techStat struct {
	Key        string  `json:"key"`
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	Sessions   int     `json:"sessions"`
	Minutes    float64 `json:"minutes"`
	Rated      int     `json:"rated"`
	AvgBefore  float64 `json:"avg_before"`
	AvgAfter   float64 `json:"avg_after"`
	AvgShift   float64 `json:"avg_shift"`
	LastDate   string  `json:"last_date"`
	before     []float64
	after      []float64
	shift      []float64
	Completion int `json:"completion_pct"`
	completed  int
}

// buildStats computes everything the statistics page shows for a range.
func (s *Server) buildStats(ctx context.Context, uid int64, days int) (map[string]any, error) {
	st, _ := s.store.GetSettings(ctx, uid)
	today := store.Today()
	first := s.store.FirstDate(ctx, uid)
	to := today
	from := store.AddDays(to, -(days - 1))
	if days >= 100000 || (first != "" && first > from && days > 365) {
		from = first
		if from == "" {
			from = to
		}
	}
	if from > to {
		from = to
	}
	t0, _ := store.ParseDate(from)
	t1, _ := store.ParseDate(to)
	nDays := int(t1.Sub(t0).Hours()/24) + 1

	ps, _, err := s.store.ListPractices(ctx, uid, store.PracticeFilter{From: from, To: to, Asc: true})
	if err != nil {
		return nil, err
	}
	prevFrom, prevTo := store.AddDays(from, -nDays), store.AddDays(from, -1)
	prev, _, _ := s.store.ListPractices(ctx, uid, store.PracticeFilter{From: prevFrom, To: prevTo})

	// totals
	var minutes, longest float64
	completed := 0
	dayMin := map[string]float64{}
	daySessions := map[string]int{}
	for _, p := range ps {
		m := p.Minutes()
		minutes += m
		longest = math.Max(longest, m)
		dayMin[p.Date] += m
		daySessions[p.Date]++
		if p.Completed {
			completed++
		}
	}
	prevMin := 0.0
	prevDays := map[string]bool{}
	for _, p := range prev {
		prevMin += p.Minutes()
		prevDays[p.Date] = true
	}
	avgSession := 0.0
	if len(ps) > 0 {
		avgSession = minutes / float64(len(ps))
	}
	totals := map[string]any{
		"minutes": round1(minutes), "sessions": len(ps), "days_practiced": len(dayMin), "avg_session_min": round1(avgSession),
		"longest_session_min": round1(longest), "consistency_pct": pct(len(dayMin), nDays), "avg_daily_min": round1(minutes / float64(nDays)),
		"completion_pct": pct(completed, len(ps)),
	}
	previous := map[string]any{"from": prevFrom, "to": prevTo, "minutes": round1(prevMin), "sessions": len(prev), "days_practiced": len(prevDays)}
	if prevMin > 0 {
		previous["change_pct"] = math.Round((minutes-prevMin)/prevMin*1000) / 10
	}

	// series
	goal := float64(st.DailyGoalMin)
	type dayPoint struct {
		Date     string  `json:"date"`
		Minutes  float64 `json:"minutes"`
		Sessions int     `json:"sessions"`
		GoalMet  bool    `json:"goal_met"`
	}
	daily := make([]dayPoint, 0, nDays)
	goalDays := 0
	for d := from; d <= to; d = store.AddDays(d, 1) {
		m := dayMin[d]
		met := goal > 0 && m+0.01 >= goal
		if met {
			goalDays++
		}
		daily = append(daily, dayPoint{d, round1(m), daySessions[d], met})
	}
	type weekPoint struct {
		Week    string  `json:"week"`
		Minutes float64 `json:"minutes"`
		Days    int     `json:"days"`
	}
	weekly := []weekPoint{}
	wIdx := map[string]int{}
	for _, dp := range daily {
		w := weekStartOf(dp.Date, st.WeekStart)
		i, ok := wIdx[w]
		if !ok {
			i = len(weekly)
			wIdx[w] = i
			weekly = append(weekly, weekPoint{Week: w})
		}
		weekly[i].Minutes = round1(weekly[i].Minutes + dp.Minutes)
		if dp.Sessions > 0 {
			weekly[i].Days++
		}
	}
	type monthPoint struct {
		Month    string  `json:"month"`
		Minutes  float64 `json:"minutes"`
		Sessions int     `json:"sessions"`
		Days     int     `json:"days"`
	}
	monthly := []monthPoint{}
	mIdx := map[string]int{}
	for _, dp := range daily {
		mo := dp.Date[:7]
		i, ok := mIdx[mo]
		if !ok {
			i = len(monthly)
			mIdx[mo] = i
			monthly = append(monthly, monthPoint{Month: mo})
		}
		monthly[i].Minutes = round1(monthly[i].Minutes + dp.Minutes)
		monthly[i].Sessions += dp.Sessions
		if dp.Sessions > 0 {
			monthly[i].Days++
		}
	}

	// the year heatmap, whatever the range
	hFrom := store.AddDays(today, -370)
	hm, _ := s.store.DayMinutes(ctx, uid, hFrom, today)
	type heat struct {
		Date    string  `json:"date"`
		Minutes float64 `json:"minutes"`
	}
	heatmap := make([]heat, 0, 371)
	for d := hFrom; d <= today; d = store.AddDays(d, 1) {
		heatmap = append(heatmap, heat{d, round1(hm[d])})
	}

	// by kind, technique, hour, weekday, length
	type kindStat struct {
		Kind     string  `json:"kind"`
		Name     string  `json:"name"`
		Sessions int     `json:"sessions"`
		Minutes  float64 `json:"minutes"`
	}
	kinds := map[string]*kindStat{}
	techs := map[string]*techStat{}
	presetNames := map[string]string{}
	if pr, err := s.store.ListPresets(ctx, uid, ""); err == nil {
		for _, p := range pr {
			presetNames[fmt.Sprintf("preset:%d", p.ID)] = p.Name
		}
	}
	type hourStat struct {
		Hour     int     `json:"hour"`
		Sessions int     `json:"sessions"`
		Minutes  float64 `json:"minutes"`
	}
	hours := make([]hourStat, 24)
	for i := range hours {
		hours[i].Hour = i
	}
	parts := map[string]*partStat{}
	for _, k := range []string{"morning", "afternoon", "evening", "night"} {
		parts[k] = &partStat{}
	}
	lengthEdges := []struct {
		label string
		max   float64
	}{{"under 3 min", 3}, {"3–5 min", 5}, {"5–10 min", 10}, {"10–15 min", 15}, {"15–20 min", 20}, {"20–30 min", 30}, {"30+ min", math.Inf(1)}}
	lengths := make([]map[string]any, len(lengthEdges))
	for i, e := range lengthEdges {
		lengths[i] = map[string]any{"label": e.label, "sessions": 0}
	}
	var before, after, shift []float64
	improved := 0
	type moodWeek struct {
		Week     string  `json:"week"`
		AvgAfter float64 `json:"avg_after"`
		AvgShift float64 `json:"avg_shift"`
		Rated    int     `json:"rated"`
		after    []float64
		shift    []float64
	}
	moodWeeks := []*moodWeek{}
	mwIdx := map[string]*moodWeek{}
	for _, p := range ps {
		m := p.Minutes()
		k := kinds[p.Kind]
		if k == nil {
			k = &kindStat{Kind: p.Kind, Name: content.KindName(p.Kind)}
			kinds[p.Kind] = k
		}
		k.Sessions++
		k.Minutes += m
		key := p.Technique
		if key == "" {
			key = p.Kind
		}
		t := techs[key]
		if t == nil {
			name := content.TechniqueName(key)
			if name == "" {
				name = presetNames[key]
			}
			if name == "" {
				name = firstNonEmpty(p.Title, content.KindName(p.Kind))
			}
			t = &techStat{Key: key, Name: name, Kind: p.Kind}
			techs[key] = t
		}
		t.Sessions++
		t.Minutes += m
		if p.Completed {
			t.completed++
		}
		if p.Date > t.LastDate {
			t.LastDate = p.Date
		}
		h := startHour(p)
		hours[h].Sessions++
		hours[h].Minutes = round1(hours[h].Minutes + m)
		pd := parts[partOfDay(h)]
		pd.Sessions++
		pd.Minutes = round1(pd.Minutes + m)
		for i, e := range lengthEdges {
			if m < e.max {
				lengths[i]["sessions"] = lengths[i]["sessions"].(int) + 1
				break
			}
		}
		if p.CalmAfter != nil {
			a := float64(*p.CalmAfter)
			after = append(after, a)
			t.after = append(t.after, a)
			w := weekStartOf(p.Date, st.WeekStart)
			mw := mwIdx[w]
			if mw == nil {
				mw = &moodWeek{Week: w}
				mwIdx[w] = mw
				moodWeeks = append(moodWeeks, mw)
			}
			mw.after = append(mw.after, a)
			if p.CalmBefore != nil {
				b := float64(*p.CalmBefore)
				before = append(before, b)
				t.before = append(t.before, b)
				sh := a - b
				shift = append(shift, sh)
				t.shift = append(t.shift, sh)
				pd.shift = append(pd.shift, sh)
				mw.shift = append(mw.shift, sh)
				if sh > 0 {
					improved++
				}
			}
		}
	}
	byKind := []kindStat{}
	for _, k := range kinds {
		k.Minutes = round1(k.Minutes)
		byKind = append(byKind, *k)
	}
	sort.Slice(byKind, func(i, j int) bool { return byKind[i].Minutes > byKind[j].Minutes })
	byTech := []techStat{}
	for _, t := range techs {
		t.Minutes = round1(t.Minutes)
		t.Rated = len(t.after)
		t.AvgBefore, t.AvgAfter, t.AvgShift = round2(mean(t.before)), round2(mean(t.after)), round2(mean(t.shift))
		t.Completion = pct(t.completed, t.Sessions)
		byTech = append(byTech, *t)
	}
	sort.Slice(byTech, func(i, j int) bool {
		if byTech[i].Sessions != byTech[j].Sessions {
			return byTech[i].Sessions > byTech[j].Sessions
		}
		return byTech[i].Name < byTech[j].Name
	})
	for _, pd := range parts {
		pd.AvgShift = round2(mean(pd.shift))
	}

	// weekdays: average minutes and how often practised
	wds := make([]wdStat, 7)
	for i := range wds {
		wds[i].Weekday = i + 1
	}
	for _, dp := range daily {
		w := &wds[isoWeekday(dp.Date)-1]
		w.Days++
		w.Sessions += dp.Sessions
		w.minutes += dp.Minutes
		if dp.Sessions > 0 {
			w.Practiced++
		}
	}
	for i := range wds {
		if wds[i].Days > 0 {
			wds[i].AvgMinutes = round1(wds[i].minutes / float64(wds[i].Days))
			wds[i].PracticePct = pct(wds[i].Practiced, wds[i].Days)
		}
	}

	mood := map[string]any{"rated": len(after), "with_before": len(shift), "avg_before": round2(mean(before)), "avg_after": round2(mean(after)),
		"avg_shift": round2(mean(shift)), "improved_pct": pct(improved, len(shift))}
	mw := make([]moodWeek, 0, len(moodWeeks))
	for _, w := range moodWeeks {
		w.AvgAfter, w.AvgShift, w.Rated = round2(mean(w.after)), round2(mean(w.shift)), len(w.after)
		mw = append(mw, *w)
	}
	mood["weekly"] = mw

	// lifetime, level, streaks
	life, _ := s.store.LifetimeTotals(ctx, uid)
	lvl, next := content.LevelFor(life.Minutes)
	lifetime := map[string]any{"minutes": round1(life.Minutes), "sessions": life.Sessions, "days": life.Days, "first_date": first, "level": lvl, "next_level": next}
	if next != nil {
		span := float64(next.Minutes - lvl.Minutes)
		lifetime["level_pct"] = int(math.Min(100, (life.Minutes-float64(lvl.Minutes))*100/span))
		lifetime["minutes_to_next"] = math.Ceil(float64(next.Minutes) - life.Minutes)
	} else {
		lifetime["level_pct"] = 100
	}
	cur, best := s.streaks(ctx, uid)
	gcur, gbest, _ := s.goalStreaks(ctx, uid, st.DailyGoalMin)

	// this week against the weekly goal
	ws := weekStartOf(today, st.WeekStart)
	wm, _ := s.store.DayMinutes(ctx, uid, ws, today)
	weekMin := 0.0
	for _, m := range wm {
		weekMin += m
	}
	goals := map[string]any{"daily_goal_min": st.DailyGoalMin, "weekly_goal_days": st.WeeklyGoalDays, "days_met": goalDays,
		"days_met_pct": pct(goalDays, nDays), "this_week_days": len(wm), "this_week_minutes": round1(weekMin), "week_start": ws}

	out := map[string]any{
		"range":  map[string]any{"from": from, "to": to, "days": nDays},
		"totals": totals, "previous": previous, "lifetime": lifetime,
		"streak": map[string]any{"current": cur, "longest": best, "goal_current": gcur, "goal_longest": gbest},
		"daily":  daily, "weekly": weekly, "monthly": monthly, "heatmap": heatmap,
		"by_kind": byKind, "by_technique": byTech, "by_hour": hours, "parts_of_day": parts, "weekdays": wds, "lengths": lengths,
		"mood": mood, "goals": goals,
	}
	out["insights"] = statInsights(out, byTech, wds, parts, life.Minutes, next)
	return out, nil
}

var weekdayNames = []string{"", "Mondays", "Tuesdays", "Wednesdays", "Thursdays", "Fridays", "Saturdays", "Sundays"}

// partStat is one part of the day.
type partStat struct {
	Sessions int     `json:"sessions"`
	Minutes  float64 `json:"minutes"`
	AvgShift float64 `json:"avg_shift"`
	shift    []float64
}

// wdStat is one weekday: average minutes and how often practised.
type wdStat struct {
	Weekday     int     `json:"weekday"`
	Days        int     `json:"days"` // how many of that weekday were in the range
	Practiced   int     `json:"practiced"`
	Sessions    int     `json:"sessions"`
	AvgMinutes  float64 `json:"avg_minutes"`
	PracticePct int     `json:"practice_pct"`
	minutes     float64
}

// statInsights turns the numbers into a few plain sentences.
func statInsights(st map[string]any, techs []techStat, wds []wdStat, parts map[string]*partStat, lifeMin float64, next *content.Level) []string {
	var out []string
	totals := st["totals"].(map[string]any)
	if totals["sessions"].(int) == 0 {
		return []string{"No practice in this period yet. A three-minute breath is a good start."}
	}
	prev := st["previous"].(map[string]any)
	if c, ok := prev["change_pct"].(float64); ok {
		if c >= 10 {
			out = append(out, fmt.Sprintf("You practised %.0f%% more than in the period before.", c))
		} else if c <= -10 {
			out = append(out, fmt.Sprintf("You practised %.0f%% less than in the period before.", -c))
		}
	}
	// the technique that calms you most (at least three rated sessions)
	best := techStat{AvgShift: -99}
	for _, t := range techs {
		if len(t.shift) >= 3 && t.AvgShift > best.AvgShift {
			best = t
		}
	}
	if best.Name != "" && best.AvgShift > 0 {
		out = append(out, fmt.Sprintf("%s helps you most: on average you feel %.1f steps calmer afterwards (%d rated).", best.Name, best.AvgShift, len(best.shift)))
	}
	// the most reliable weekday
	bw, bp := 0, 0
	for _, w := range wds {
		if w.Days >= 2 && w.PracticePct > bp {
			bw, bp = w.Weekday, w.PracticePct
		}
	}
	if bw > 0 && bp >= 50 && totals["days_practiced"].(int) >= 5 {
		out = append(out, fmt.Sprintf("%s are your most reliable day: you practised on %d%% of them.", weekdayNames[bw], bp))
	}
	// favourite part of the day
	top, topN, total := "", 0, 0
	for _, k := range []string{"morning", "afternoon", "evening", "night"} {
		v := parts[k]
		total += v.Sessions
		if v.Sessions > topN {
			top, topN = k, v.Sessions
		}
	}
	if total >= 5 && topN*2 >= total {
		out = append(out, fmt.Sprintf("The %s is your time: %d%% of your practices start then.", top, pct(topN, total)))
	}
	if c := totals["consistency_pct"].(int); c >= 80 {
		out = append(out, fmt.Sprintf("You practised on %d%% of days. That consistency is what changes things.", c))
	}
	if next != nil {
		left := math.Ceil(float64(next.Minutes) - lifeMin)
		if left <= 120 {
			out = append(out, fmt.Sprintf("%.0f more minutes to reach %s.", left, next.Name))
		}
	}
	mood := st["mood"].(map[string]any)
	if n := mood["with_before"].(int); n >= 5 {
		out = append(out, fmt.Sprintf("You felt calmer after %d%% of the practices you rated.", mood["improved_pct"].(int)))
	}
	return out
}

// bestWeekday names the weekday practised most (for the weekly report).
func bestWeekday(dates map[string]float64) string {
	cnt := make([]int, 8)
	for d := range dates {
		cnt[isoWeekday(d)]++
	}
	best, bi := 0, 0
	for i := 1; i <= 7; i++ {
		if cnt[i] > best {
			best, bi = cnt[i], i
		}
	}
	if bi == 0 {
		return ""
	}
	return weekdayNames[bi]
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	out, err := s.buildStats(ctx, actorID(ctx), rangeDays(r.URL.Query().Get("range")))
	if err != nil {
		s.fail(w, err, "stats")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// ---- home, dashboard, Pocket Day, Pocket Calendar ----

// suggestion picks something to practise now: the time of day first, then
// what has calmed this person most at this time.
func (s *Server) suggestion(ctx context.Context, uid int64, now time.Time) map[string]any {
	h := now.Hour()
	key, why := "coherent", "Ten minutes of resonance breathing, the classic daily dose."
	switch {
	case h >= 5 && h < 10:
		key, why = "energize", "A short energizing breath to start the day awake."
	case h >= 10 && h < 14:
		key, why = "box", "Box breathing to steady attention for the rest of the day."
	case h >= 14 && h < 18:
		key, why = "sigh", "A few physiological sighs to reset the afternoon."
	case h >= 18 && h < 22:
		key, why = "downshift", "Long exhales to come down from the day."
	default:
		key, why = "relax478", "4-7-8 breathing to help you fall asleep."
	}
	from := store.AddDays(store.Today(), -90)
	ps, _, _ := s.store.ListPractices(ctx, uid, store.PracticeFilter{From: from})
	shift := map[string][]float64{}
	part := partOfDay(h)
	for _, p := range ps {
		if p.CalmBefore == nil || p.CalmAfter == nil || partOfDay(startHour(p)) != part || content.TechniqueName(p.Technique) == "" {
			continue
		}
		shift[p.Technique] = append(shift[p.Technique], float64(*p.CalmAfter-*p.CalmBefore))
	}
	bestKey, bestShift := "", 0.5
	for k, v := range shift {
		if len(v) >= 3 && mean(v) > bestShift {
			bestKey, bestShift = k, mean(v)
		}
	}
	if bestKey != "" {
		key = bestKey
		why = fmt.Sprintf("In the %s this has calmed you most (%+.1f on average).", part, bestShift)
	}
	out := map[string]any{"key": key, "name": content.TechniqueName(key), "reason": why, "part_of_day": part}
	if p, ok := content.PatternByKey(key); ok {
		out["type"], out["color"], out["minutes"] = "pattern", p.Color, p.Config.Minutes
	} else if st, ok := content.StyleByKey(key); ok {
		out["type"], out["color"], out["minutes"] = "style", st.Color, st.Minutes
	}
	return out
}

// intentions are short lines for the top of the home page, one a day.
var intentions = []string{
	"Breathe in, and know you are breathing in.",
	"Nothing to fix. Just notice.",
	"Every return to the breath is a repetition.",
	"Slow is smooth, smooth is calm.",
	"You can't stop the waves, but you can learn to surf.",
	"Wherever you go, there you are.",
	"The present moment is the only one you can practise in.",
	"Feelings come and go like clouds in a windy sky.",
	"Be kind to the mind that wanders.",
	"A few breaths are enough to begin.",
	"Let the out-breath be a little longer.",
	"Rest is also practice.",
	"Notice one thing you haven't noticed before.",
	"Small and daily beats big and rare.",
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	st, _ := s.store.GetSettings(ctx, uid)
	now := time.Now()
	today := store.Today()
	weekFrom := store.AddDays(today, -6)
	mins, err := s.store.DayMinutes(ctx, uid, weekFrom, today)
	if err != nil {
		s.fail(w, err, "home")
		return
	}
	type day struct {
		Date    string  `json:"date"`
		Minutes float64 `json:"minutes"`
		Met     bool    `json:"met"`
	}
	week := make([]day, 0, 7)
	for d := weekFrom; d <= today; d = store.AddDays(d, 1) {
		week = append(week, day{d, round1(mins[d]), st.DailyGoalMin > 0 && mins[d]+0.01 >= float64(st.DailyGoalMin)})
	}
	todayPs, _, _ := s.store.ListPractices(ctx, uid, store.PracticeFilter{From: today, To: today})
	recent, _, _ := s.store.ListPractices(ctx, uid, store.PracticeFilter{Limit: 6})
	cur, best := s.streaks(ctx, uid)
	life, _ := s.store.LifetimeTotals(ctx, uid)
	lvl, next := content.LevelFor(life.Minutes)
	levelPct := 100
	if next != nil {
		levelPct = int(math.Min(100, (life.Minutes-float64(lvl.Minutes))*100/float64(next.Minutes-lvl.Minutes)))
	}
	// the next lesson of the program worked on most recently (the Path when nothing was started)
	var nextLesson *lessonView
	var nextProgram *programView
	var inProgress []programView
	starts, _ := s.store.ProgramStarts(ctx, uid)
	lastAt := ""
	for _, p := range content.Programs {
		v, err := s.programProgress(ctx, uid, p)
		if err != nil {
			continue
		}
		v.Stages = nil
		active := firstNonEmpty(v.LastAt, starts[p.Key])
		if starts[p.Key] != "" && v.Done < v.Total {
			inProgress = append(inProgress, *v)
		}
		if v.Next == nil || (p.Kind != "path" && active == "") {
			continue
		}
		if nextLesson == nil || active > lastAt {
			cp := *v
			nextLesson, nextProgram, lastAt = v.Next, &cp, active
		}
	}
	favs, _ := s.store.Favorites(ctx, uid)
	presets, _ := s.store.ListPresets(ctx, uid, "")
	quick := []map[string]any{}
	for _, p := range content.Patterns {
		if favs[p.Key] {
			quick = append(quick, map[string]any{"type": "pattern", "key": p.Key, "name": p.Name, "color": p.Color, "minutes": p.Config.Minutes, "cycles": p.Config.Cycles})
		}
	}
	for _, sty := range content.Styles {
		if favs[sty.Key] {
			quick = append(quick, map[string]any{"type": "style", "key": sty.Key, "name": sty.Name, "color": sty.Color, "minutes": sty.Minutes})
		}
	}
	for _, p := range presets {
		if p.Favorite {
			quick = append(quick, map[string]any{"type": "preset", "key": fmt.Sprintf("preset:%d", p.ID), "id": p.ID, "kind": p.Kind, "name": p.Name, "config": p.Config})
		}
	}
	ach, _ := s.store.Achievements(ctx, uid)
	type un struct {
		content.Achievement
		UnlockedAt string `json:"unlocked_at"`
	}
	var latest []un
	for k, u := range ach {
		if a, ok := content.AchievementByKey(k); ok {
			latest = append(latest, un{a, u.UnlockedAt})
		}
	}
	sort.Slice(latest, func(i, j int) bool { return latest[i].UnlockedAt > latest[j].UnlockedAt })
	if len(latest) > 3 {
		latest = latest[:3]
	}
	dayN := now.YearDay() + now.Year()
	writeJSON(w, http.StatusOK, map[string]any{
		"part_of_day": partOfDay(now.Hour()), "intention": intentions[dayN%len(intentions)],
		"today": map[string]any{"date": today, "minutes": round1(mins[today]), "sessions": len(todayPs), "goal_min": st.DailyGoalMin,
			"goal_pct": int(math.Min(100, float64(pct(int(mins[today]*10), st.DailyGoalMin*10))))},
		"week": week, "streak": cur, "longest_streak": best,
		"lifetime":    map[string]any{"minutes": round1(life.Minutes), "sessions": life.Sessions, "level": lvl, "next_level": next, "level_pct": levelPct},
		"next_lesson": nextLesson, "next_program": nextProgram, "in_progress": inProgress,
		"recent": recent, "today_practices": todayPs, "suggestion": s.suggestion(ctx, uid, now), "quick": quick, "latest_milestones": latest,
	})
}

// handleDashboard is the small summary for Homepage widgets and the sidebar.
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	st, _ := s.store.GetSettings(ctx, uid)
	today := store.Today()
	ws := weekStartOf(today, st.WeekStart)
	mins, _ := s.store.DayMinutes(ctx, uid, ws, today)
	week := 0.0
	for _, m := range mins {
		week += m
	}
	cur, best := s.streaks(ctx, uid)
	life, _ := s.store.LifetimeTotals(ctx, uid)
	lvl, _ := content.LevelFor(life.Minutes)
	out := map[string]any{"today_min": round1(mins[today]), "goal_min": st.DailyGoalMin, "goal_met": st.DailyGoalMin > 0 && mins[today]+0.01 >= float64(st.DailyGoalMin),
		"week_min": round1(week), "week_days": len(mins), "weekly_goal_days": st.WeeklyGoalDays, "streak": cur, "longest_streak": best,
		"lifetime_min": round1(life.Minutes), "sessions": life.Sessions, "level": lvl.Name}
	if p, ok := content.ProgramByKey("path"); ok {
		if v, err := s.programProgress(ctx, uid, p); err == nil {
			out["path_done"], out["path_total"], out["path_stage"] = v.Done, v.Total, v.StageName
			if v.Next != nil {
				out["next_lesson"] = v.Next.Title
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// handleDay is one day for Pocket Day's journal (and anyone with mindfulness:read).
func (s *Server) handleDay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	date := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	if !validDate(date) || date == "" {
		writeErr(w, http.StatusBadRequest, "a date (YYYY-MM-DD) is required")
		return
	}
	ps, _, err := s.store.ListPractices(ctx, uid, store.PracticeFilter{From: date, To: date, Asc: true})
	if err != nil {
		s.fail(w, err, "day")
		return
	}
	ps = forLink(ctx, ps)
	total := 0.0
	type item struct {
		ID         int64   `json:"id"`
		Kind       string  `json:"kind"`
		KindName   string  `json:"kind_name"`
		Technique  string  `json:"technique"`
		Title      string  `json:"title"`
		StartedAt  string  `json:"started_at"`
		EndedAt    string  `json:"ended_at"`
		Minutes    float64 `json:"minutes"`
		CalmBefore *int    `json:"calm_before"`
		CalmAfter  *int    `json:"calm_after"`
		Lesson     string  `json:"lesson,omitempty"`
		Note       string  `json:"note,omitempty"`
	}
	items := make([]item, 0, len(ps))
	for _, p := range ps {
		total += p.Minutes()
		it := item{p.ID, p.Kind, content.KindName(p.Kind), p.Technique, p.Title, p.StartedAt, p.EndedAt, round1(p.Minutes()), p.CalmBefore, p.CalmAfter, "", p.Note}
		if prog, ok := content.ProgramByKey(p.Program); ok {
			if l, ok := prog.FindLesson(p.Lesson); ok {
				it.Lesson = prog.Name + ": " + l.Lesson.Title
			}
		}
		items = append(items, it)
	}
	// the streak as it stood at the end of that day
	dates, _ := s.store.PracticeDates(ctx, uid)
	var upto []string
	for _, d := range dates {
		if d <= date {
			upto = append(upto, d)
		}
	}
	streak := 0
	if len(upto) > 0 && upto[len(upto)-1] == date {
		streak, _ = streakOf(upto, date)
	}
	done, _ := s.store.LessonsDone(ctx, uid, "")
	var lessons []map[string]any
	for _, d := range done {
		if t, err := time.Parse(time.RFC3339Nano, d.DoneAt); err == nil && t.Local().Format("2006-01-02") == date {
			if prog, ok := content.ProgramByKey(d.Program); ok {
				if l, ok := prog.FindLesson(d.Lesson); ok {
					lessons = append(lessons, map[string]any{"program": prog.Name, "stage": prog.Stages[l.StageIdx].Name, "title": l.Lesson.Title})
				}
			}
		}
	}
	ach, _ := s.store.Achievements(ctx, uid)
	var milestones []string
	for k, u := range ach {
		if t, err := time.Parse(time.RFC3339Nano, u.UnlockedAt); err == nil && t.Local().Format("2006-01-02") == date {
			if a, ok := content.AchievementByKey(k); ok {
				milestones = append(milestones, a.Name)
			}
		}
	}
	sort.Strings(milestones)
	st, _ := s.store.GetSettings(ctx, uid)
	summary := ""
	if len(items) > 0 {
		var names []string
		for _, it := range items {
			names = append(names, strings.ToLower(it.Title))
		}
		summary = fmt.Sprintf("%s of practice: %s", fmtMinutes(total), strings.Join(uniq(names), ", "))
		if streak > 1 {
			summary += fmt.Sprintf(" (day %d of a streak)", streak)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"date": date, "minutes": round1(total), "sessions": len(items), "practices": items, "streak": streak,
		"goal_min": st.DailyGoalMin, "goal_met": st.DailyGoalMin > 0 && total+0.01 >= float64(st.DailyGoalMin), "lessons": lessons, "milestones": milestones, "summary": summary})
}

func uniq(v []string) []string {
	seen := map[string]bool{}
	out := v[:0]
	for _, x := range v {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func fmtMinutes(m float64) string {
	mm := int(math.Round(m))
	if mm < 60 {
		return fmt.Sprintf("%d min", mm)
	}
	if mm%60 == 0 {
		return fmt.Sprintf("%d h", mm/60)
	}
	return fmt.Sprintf("%d h %d min", mm/60, mm%60)
}

// handleOverlay lists practices as timed items for Pocket Calendar.
func (s *Server) handleOverlay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	if len(from) > 10 {
		from = from[:10]
	}
	if len(to) > 10 {
		to = to[:10]
	}
	if from == "" || to == "" || !validDate(from) || !validDate(to) {
		writeErr(w, http.StatusBadRequest, "from and to are dates (YYYY-MM-DD)")
		return
	}
	ps, _, err := s.store.ListPractices(ctx, actorID(ctx), store.PracticeFilter{From: from, To: to, Asc: true, Limit: 2000})
	if err != nil {
		s.fail(w, err, "overlay")
		return
	}
	colors := map[string]string{"breath": "#0ea5e9", "meditation": "#0f766e", "body_scan": "#14b8a6", "loving_kindness": "#ec4899", "walking": "#84cc16", "other": "#64748b"}
	items := make([]map[string]any, 0, len(ps))
	for _, p := range ps {
		end := p.EndedAt
		if end == "" {
			end = p.StartedAt
		}
		items = append(items, map[string]any{"id": fmt.Sprintf("still-%d", p.ID), "title": fmt.Sprintf("%s · %s", p.Title, fmtMinutes(p.Minutes())), "kind": p.Kind,
			"start": p.StartedAt, "end": end, "all_day": false, "color": colors[p.Kind], "url": s.cfg.BaseURL + "/journal/" + fmt.Sprint(p.ID)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-still/internal/notify"
	"github.com/abjelosevic88/pocket-homelab-still/internal/store"
)

// pushes runs every minute and sends, each at the person's own time:
// practice reminders (skipped when today's goal is already met), the
// evening "your streak ends tonight" nudge and the weekly report.
func (s *Server) pushes(ctx context.Context) {
	ids, err := s.store.SettingsUsers(ctx)
	if err != nil {
		return
	}
	now := time.Now()
	today := store.Today()
	hm := now.Format("15:04")
	wd := int(now.Weekday())
	if wd == 0 {
		wd = 7
	}
	for _, uid := range ids {
		st, err := s.store.GetSettings(ctx, uid)
		if err != nil || st.NotifyURL == "" {
			continue
		}
		n := notify.New(st.NotifyURL, st.NotifyFormat)
		once := func(key string) bool {
			k := fmt.Sprintf("push:%d:%s:%s", uid, today, key)
			if _, _, done := s.store.CacheGet(ctx, k); done {
				return false
			}
			_ = s.store.CachePut(ctx, k, "1")
			return true
		}
		mins, _ := s.store.DayMinutes(ctx, uid, today, today)
		todayMin := mins[today]
		goalMet := st.DailyGoalMin > 0 && todayMin+0.01 >= float64(st.DailyGoalMin)

		// reminders at the chosen times on the chosen weekdays
		if strings.Contains(st.ReminderDays, fmt.Sprint(wd)) {
			for _, t := range st.ReminderTimes {
				if !withinMinutes(hm, t, 4) || !once("reminder:"+t) {
					continue
				}
				if st.ReminderSkipDone && (goalMet || (st.DailyGoalMin == 0 && todayMin > 0)) {
					continue
				}
				_ = n.Send(ctx, notify.Message{Title: "Time to be still", Body: s.reminderText(ctx, uid, todayMin, st.DailyGoalMin), Priority: "info", Tags: []string{"lotus_position"}, Link: s.cfg.BaseURL + "/"})
			}
		}
		// the streak ends tonight
		if st.NotifyStreak && now.Hour() == st.StreakHour && now.Minute() < 5 && todayMin == 0 {
			if cur, _ := s.streaks(ctx, uid); cur >= 2 && once("streak") {
				_ = n.Send(ctx, notify.Message{Title: fmt.Sprintf("Keep your %d-day streak", cur), Body: "Nothing practised yet today. One minute of breathing keeps it going.", Priority: "warning", Tags: []string{"fire"}, Link: s.cfg.BaseURL + "/breathe/sigh"})
			}
		}
		// the weekly report
		if st.NotifyWeekly && wd == st.WeeklyDay && now.Hour() == st.WeeklyHour && now.Minute() < 5 && once("weekly") {
			if msg := s.weeklyReport(ctx, uid, st); msg != "" {
				_ = n.Send(ctx, notify.Message{Title: "Your week in Pocket Still", Body: msg, Priority: "info", Tags: []string{"bar_chart"}, Link: s.cfg.BaseURL + "/stats"})
			}
		}
	}
}

// withinMinutes reports whether now (HH:MM) is at or up to n minutes after t (HH:MM).
func withinMinutes(now, t string, n int) bool {
	a, err1 := time.Parse("15:04", now)
	b, err2 := time.Parse("15:04", t)
	if err1 != nil || err2 != nil {
		return false
	}
	d := a.Sub(b)
	return d >= 0 && d <= time.Duration(n)*time.Minute
}

func (s *Server) reminderText(ctx context.Context, uid int64, todayMin float64, goal int) string {
	cur, _ := s.streaks(ctx, uid)
	if v, err := s.nextPathLine(ctx, uid); err == nil && v != "" {
		return v
	}
	switch {
	case todayMin > 0 && goal > 0:
		return fmt.Sprintf("%.0f of %d minutes so far today. A few more breaths?", todayMin, goal)
	case cur >= 2:
		return fmt.Sprintf("Day %d of your streak is waiting. Even three minutes count.", cur+1)
	}
	return "A few minutes of breathing or sitting. Your future self will thank you."
}

// nextPathLine names the next lesson on the Path once it was begun ("" otherwise).
func (s *Server) nextPathLine(ctx context.Context, uid int64) (string, error) {
	for _, key := range []string{"path"} {
		v, err := s.programByKeyProgress(ctx, uid, key)
		if err != nil || v == nil || v.Next == nil || v.Done == 0 {
			continue
		}
		return fmt.Sprintf("Next on the Path: \"%s\" (%s, %.0f min).", v.Next.Title, v.StageName, v.Next.Practice.Minutes), nil
	}
	return "", nil
}

// weeklyReport is the plain-text summary of the last seven days vs the seven before.
func (s *Server) weeklyReport(ctx context.Context, uid int64, st store.Settings) string {
	to := store.Today()
	from := store.AddDays(to, -6)
	ps, _, err := s.store.ListPractices(ctx, uid, store.PracticeFilter{From: from, To: to})
	if err != nil || len(ps) == 0 {
		return ""
	}
	prev, _, _ := s.store.ListPractices(ctx, uid, store.PracticeFilter{From: store.AddDays(from, -7), To: store.AddDays(from, -1)})
	mins, days := 0.0, map[string]float64{}
	var shift []float64
	tech := map[string]int{}
	for _, p := range ps {
		mins += p.Minutes()
		days[p.Date] += p.Minutes()
		tech[p.Title]++
		if p.CalmBefore != nil && p.CalmAfter != nil {
			shift = append(shift, float64(*p.CalmAfter-*p.CalmBefore))
		}
	}
	prevMin := 0.0
	for _, p := range prev {
		prevMin += p.Minutes()
	}
	lines := []string{fmt.Sprintf("%s over %d practices on %d of 7 days", fmtMinutes(mins), len(ps), len(days))}
	if prevMin > 0 {
		lines[0] += fmt.Sprintf(" (%+.0f%% vs the week before)", (mins-prevMin)/prevMin*100)
	}
	top, topN := "", 0
	for t, c := range tech {
		if c > topN || (c == topN && t < top) {
			top, topN = t, c
		}
	}
	if top != "" {
		lines = append(lines, fmt.Sprintf("Most practised: %s (%d×)", top, topN))
	}
	if len(shift) >= 2 {
		lines = append(lines, fmt.Sprintf("On average you felt %+.1f calmer afterwards", mean(shift)))
	}
	if cur, _ := s.streaks(ctx, uid); cur >= 2 {
		lines = append(lines, fmt.Sprintf("Streak: %d days", cur))
	}
	if st.WeeklyGoalDays > 0 {
		met := 0
		for _, m := range days {
			if st.DailyGoalMin == 0 || m+0.01 >= float64(st.DailyGoalMin) {
				met++
			}
		}
		lines = append(lines, fmt.Sprintf("Goal days: %d of %d", met, st.WeeklyGoalDays))
	}
	if v, _ := s.nextPathLine(ctx, uid); v != "" {
		lines = append(lines, v)
	}
	return strings.Join(lines, "\n")
}

// handleWeeklyNow sends the weekly report right away (Settings → Notifications).
func (s *Server) handleWeeklyNow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	st, _ := s.store.GetSettings(ctx, uid)
	msg := s.weeklyReport(ctx, uid, st)
	if msg == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "no practice in the last seven days yet"})
		return
	}
	if st.NotifyURL == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "sent": false, "report": msg})
		return
	}
	n := notify.New(st.NotifyURL, st.NotifyFormat)
	if err := n.Send(ctx, notify.Message{Title: "Your week in Pocket Still", Body: msg, Priority: "info", Tags: []string{"bar_chart"}, Link: s.cfg.BaseURL + "/stats"}); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error(), "report": msg})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "sent": true, "report": msg})
}

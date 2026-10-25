package server

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/abjelosevic88/pocket-homelab-still/internal/content"
	"github.com/abjelosevic88/pocket-homelab-still/internal/notify"
	"github.com/abjelosevic88/pocket-homelab-still/internal/store"
)

// earned works out every milestone a person's practice so far has reached.
func (s *Server) earned(ctx context.Context, uid int64) map[string]bool {
	got := map[string]bool{}
	ps, _, err := s.store.ListPractices(ctx, uid, store.PracticeFilter{Asc: true})
	if err != nil {
		return got
	}
	st, _ := s.store.GetSettings(ctx, uid)
	minutes, done, notes := 0.0, 0, 0
	used := map[string]bool{}
	for _, p := range ps {
		minutes += p.Minutes()
		if p.Note != "" {
			notes++
		}
		if !p.Completed {
			continue
		}
		done++
		used[p.Technique] = true
		got["first_practice"] = true
		switch p.Kind {
		case "breath":
			got["first_breath"] = true
		case "meditation":
			got["first_sit"] = true
		case "body_scan":
			got["first_scan"] = true
		case "loving_kindness":
			got["first_kindness"] = true
		}
		if p.Kind != "breath" && p.DurationS >= 20*60 {
			got["long_sit"] = true
		}
		if p.Kind != "breath" && p.DurationS >= 45*60 {
			got["deep_sit"] = true
		}
		if t, err := time.Parse(time.RFC3339, p.StartedAt); err == nil {
			if t.Hour() < 7 && t.Hour() >= 3 {
				got["early_bird"] = true
			}
			if t.Hour() >= 22 || t.Hour() < 3 {
				got["night_owl"] = true
			}
		}
		if p.CalmBefore != nil && p.CalmAfter != nil && *p.CalmAfter-*p.CalmBefore >= 2 {
			got["calm_shift"] = true
		}
	}
	for _, m := range []int{60, 300, 1000, 3000, 10000} {
		if minutes+0.01 >= float64(m) {
			got["minutes_"+itoa(m)] = true
		}
	}
	for _, n := range []int{10, 50, 100, 500} {
		if done >= n {
			got["sessions_"+itoa(n)] = true
		}
	}
	if notes >= 10 {
		got["reflective"] = true
	}
	all := true
	for _, p := range content.Patterns {
		all = all && used[p.Key]
	}
	got["all_breaths"] = all
	all = true
	for _, sty := range content.Styles {
		all = all && used[sty.Key]
	}
	got["all_styles"] = all
	if presets, _ := s.store.ListPresets(ctx, uid, ""); len(presets) > 0 {
		got["own_pattern"] = true
	}
	// streaks: the longest run ever counts
	if dates, err := s.store.PracticeDates(ctx, uid); err == nil {
		_, best := streakOf(dates, store.Today())
		for _, n := range []int{3, 7, 14, 30, 100} {
			if best >= n {
				got["streak_"+itoa(n)] = true
			}
		}
	}
	// a calendar week with the daily goal met every day
	if _, _, met := s.goalStreaks(ctx, uid, st.DailyGoalMin); len(met) >= 7 {
		have := map[string]bool{}
		for _, d := range met {
			have[d] = true
		}
		for _, d := range met {
			if weekStartOf(d, st.WeekStart) != d {
				continue
			}
			full := true
			for i := 0; i < 7; i++ {
				full = full && have[store.AddDays(d, i)]
			}
			if full {
				got["perfect_week"] = true
				break
			}
		}
	}
	// the Path's stages and the courses
	for _, prog := range content.Programs {
		v, err := s.programProgress(ctx, uid, prog)
		if err != nil {
			continue
		}
		if prog.Kind == "path" {
			for _, sv := range v.Stages {
				if sv.Complete {
					got["path_"+sv.Key] = true
				}
			}
		} else if v.Done == v.Total {
			got["course_"+prog.Key] = true
		}
	}
	for k, v := range got {
		if !v {
			delete(got, k)
		}
	}
	return got
}

func itoa(n int) string { return strconv.Itoa(n) }

// checkAchievements unlocks what was newly reached and announces it.
func (s *Server) checkAchievements(ctx context.Context, uid int64, _ *store.Practice) []content.Achievement {
	have, _ := s.store.Achievements(ctx, uid)
	var fresh []string
	for k := range s.earned(ctx, uid) {
		if _, ok := have[k]; ok {
			continue
		}
		if _, ok := content.AchievementByKey(k); !ok {
			continue
		}
		if ok, err := s.store.Unlock(ctx, uid, k, ""); err == nil && ok {
			fresh = append(fresh, k)
		}
	}
	sort.Slice(fresh, func(i, j int) bool { return achIndex(fresh[i]) < achIndex(fresh[j]) })
	s.announceMilestones(ctx, uid, fresh)
	out := make([]content.Achievement, 0, len(fresh))
	for _, k := range fresh {
		a, _ := content.AchievementByKey(k)
		out = append(out, a)
	}
	return out
}

func achIndex(key string) int {
	for i, a := range content.Achievements {
		if a.Key == key {
			return i
		}
	}
	return len(content.Achievements)
}

// announceMilestones publishes new milestones on the bus and pushes them
// (one message for several) when the person wants that.
func (s *Server) announceMilestones(ctx context.Context, uid int64, keys []string) {
	if len(keys) == 0 {
		return
	}
	var names []string
	for _, k := range keys {
		a, ok := content.AchievementByKey(k)
		if !ok {
			continue
		}
		names = append(names, a.Name)
		s.publish(ctx, uid, "still.milestone_reached", k, map[string]any{"key": k, "name": a.Name, "description": a.Description, "group": a.Group})
	}
	st, _ := s.store.GetSettings(ctx, uid)
	if !st.NotifyMilestones || st.NotifyURL == "" || len(names) == 0 {
		return
	}
	body := names[0]
	if a, ok := content.AchievementByKey(keys[0]); ok {
		body = a.Name + ": " + a.Description
	}
	if len(names) > 1 {
		body = "New milestones: " + joinAnd(names)
	}
	n := notify.New(st.NotifyURL, st.NotifyFormat)
	go func() {
		bg, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := n.Send(bg, notify.Message{Title: "Pocket Still milestone", Body: body, Priority: "success", Tags: []string{"trophy"}, Link: s.cfg.BaseURL + "/milestones"}); err == nil {
			_ = s.store.MarkAchievementsNotified(bg, uid, keys)
		}
	}()
}

func joinAnd(v []string) string {
	switch len(v) {
	case 0:
		return ""
	case 1:
		return v[0]
	}
	out := ""
	for i, x := range v {
		switch {
		case i == 0:
			out = x
		case i == len(v)-1:
			out += " and " + x
		default:
			out += ", " + x
		}
	}
	return out
}

func (s *Server) handleAchievements(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	have, err := s.store.Achievements(ctx, uid)
	if err != nil {
		s.fail(w, err, "milestones")
		return
	}
	type item struct {
		content.Achievement
		Unlocked   bool   `json:"unlocked"`
		UnlockedAt string `json:"unlocked_at,omitempty"`
	}
	items := make([]item, 0, len(content.Achievements))
	n := 0
	for _, a := range content.Achievements {
		u, ok := have[a.Key]
		if ok {
			n++
		}
		items = append(items, item{a, ok, u.UnlockedAt})
	}
	life, _ := s.store.LifetimeTotals(ctx, uid)
	lvl, next := content.LevelFor(life.Minutes)
	writeJSON(w, http.StatusOK, map[string]any{"achievements": items, "unlocked": n, "total": len(items), "levels": content.Levels,
		"level": lvl, "next_level": next, "lifetime_min": round1(life.Minutes)})
}

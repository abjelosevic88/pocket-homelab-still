package server

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/abjelosevic88/pocket-homelab-still/internal/content"
	"github.com/abjelosevic88/pocket-homelab-still/internal/pocket"
	"github.com/abjelosevic88/pocket-homelab-still/internal/store"
)

// ---- Pocket Health ----
//
// Each practice becomes mindful minutes in Pocket Health (one sample at its
// start time, so a re-send updates it and a deletion sends 0). Back the other
// way, Still reads heart rate variability, resting heart rate and sleep to
// compare days with practice against days without.

// syncHealth writes the practices since a date that Pocket Health hasn't got yet.
func (s *Server) syncHealth(ctx context.Context, uid int64, since string) {
	st, err := s.store.GetSettings(ctx, uid)
	if err != nil || !st.HealthSync {
		return
	}
	s.healthMu.Lock()
	defer s.healthMu.Unlock()
	ps, err := s.store.PendingHealth(ctx, uid, since)
	if err != nil || len(ps) == 0 {
		return
	}
	sib, err := optional(s.clients(ctx, uid).conn(ctx, appHealth, scopeHealthWrite))
	if err != nil || sib == nil {
		if err != nil {
			for _, p := range ps {
				_ = s.store.SetHealthSync(ctx, uid, p.ID, "", err.Error())
			}
		}
		return
	}
	for _, p := range ps {
		at, err := time.Parse(time.RFC3339, p.StartedAt)
		if err != nil {
			continue
		}
		note := p.Title
		if k := content.KindName(p.Kind); k != p.Title {
			note = k + ": " + p.Title
		}
		if err := sib.client.LogMindful(ctx, at, round2(p.Minutes()), note); err != nil {
			_ = s.store.SetHealthSync(ctx, uid, p.ID, "", err.Error())
			s.log.Debug("pocket health: mindful minutes failed", "practice", p.ID, "err", err)
			return // try the rest on the next round
		}
		_ = s.store.SetHealthSync(ctx, uid, p.ID, store.Now(), "")
	}
}

// zeroHealthSoon takes a practice's minutes out of Pocket Health (it was
// deleted, or its start moved) by sending 0 at the old start.
func (s *Server) zeroHealthSoon(uid int64, startedAt string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		st, err := s.store.GetSettings(ctx, uid)
		if err != nil || !st.HealthSync {
			return
		}
		at, err := time.Parse(time.RFC3339, startedAt)
		if err != nil {
			return
		}
		sib, err := optional(s.clients(ctx, uid).conn(ctx, appHealth, scopeHealthWrite))
		if err != nil || sib == nil {
			return
		}
		if err := sib.client.LogMindful(ctx, at, 0, "removed in Pocket Still"); err != nil {
			s.log.Debug("pocket health: zeroing failed", "err", err)
		}
	}()
}

// healthMetric is one Pocket Health metric compared on days with and without practice.
type healthMetric struct {
	Key            string    `json:"key"`
	Name           string    `json:"name"`
	Unit           string    `json:"unit"`
	HigherIsBetter bool      `json:"higher_is_better"`
	When           string    `json:"when"` // same_day | next_night
	PracticeAvg    float64   `json:"practice_avg"`
	RestAvg        float64   `json:"rest_avg"`
	Diff           float64   `json:"diff"`
	DiffPct        float64   `json:"diff_pct"`
	NPractice      int       `json:"n_practice"`
	NRest          int       `json:"n_rest"`
	Correlation    float64   `json:"correlation"` // minutes vs value, Pearson
	Better         *bool     `json:"better,omitempty"`
	Points         []hmPoint `json:"points"`
}

type hmPoint struct {
	Date    string  `json:"date"`
	Value   float64 `json:"value"`
	Minutes float64 `json:"minutes"`
}

func pearson(x, y []float64) float64 {
	n := len(x)
	if n < 3 || n != len(y) {
		return 0
	}
	mx, my := mean(x), mean(y)
	var sxy, sxx, syy float64
	for i := range x {
		sxy += (x[i] - mx) * (y[i] - my)
		sxx += (x[i] - mx) * (x[i] - mx)
		syy += (y[i] - my) * (y[i] - my)
	}
	if sxx == 0 || syy == 0 {
		return 0
	}
	return sxy / math.Sqrt(sxx*syy)
}

// handleHealthInsights compares heart rate variability, resting heart rate
// and sleep on days with practice against days without (the last 90 days).
func (s *Server) handleHealthInsights(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	st, _ := s.store.GetSettings(ctx, uid)
	if !st.HealthInsights {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "switched off in Settings → Connected apps"})
		return
	}
	sib, err := s.clients(ctx, uid).conn(ctx, appHealth, scopeHealthRead)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error(), "not_connected": isNotConnected(err)})
		return
	}
	rng := r.URL.Query().Get("range")
	if rng == "" {
		rng = "3m"
	}
	days := rangeDays(rng)
	today := store.Today()
	from := store.AddDays(today, -(days - 1))
	mins, _ := s.store.DayMinutes(ctx, uid, store.AddDays(from, -1), today)
	specs := []struct {
		key, name, unit, when string
		higher                bool
	}{
		{"hrv", "Heart rate variability", "ms", "same_day", true},
		{"resting_hr", "Resting heart rate", "bpm", "same_day", false},
		{"sleep_asleep", "Time asleep", "min", "next_night", true},
	}
	var out []healthMetric
	var errs []string
	for _, sp := range specs {
		pts, err := sib.client.Metric(ctx, sp.key, rng, today)
		if err != nil {
			errs = append(errs, sp.name+": "+err.Error())
			continue
		}
		hm := healthMetric{Key: sp.key, Name: sp.name, Unit: sp.unit, When: sp.when, HigherIsBetter: sp.higher, Points: []hmPoint{}}
		var on, off, xs, ys []float64
		for _, p := range pts {
			if p.Date < from {
				continue
			}
			// a night is dated by the morning: the practice the evening before counts
			day := p.Date
			if sp.when == "next_night" {
				day = store.AddDays(p.Date, -1)
			}
			m := mins[day]
			hm.Points = append(hm.Points, hmPoint{p.Date, round1(p.Value), round1(m)})
			xs, ys = append(xs, m), append(ys, p.Value)
			if m > 0 {
				on = append(on, p.Value)
			} else {
				off = append(off, p.Value)
			}
		}
		hm.NPractice, hm.NRest = len(on), len(off)
		hm.PracticeAvg, hm.RestAvg = round1(mean(on)), round1(mean(off))
		hm.Correlation = round2(pearson(xs, ys))
		if len(on) >= 3 && len(off) >= 3 {
			hm.Diff = round1(mean(on) - mean(off))
			if mean(off) != 0 {
				hm.DiffPct = round1((mean(on) - mean(off)) / math.Abs(mean(off)) * 100)
			}
			b := (hm.Diff > 0) == sp.higher && hm.Diff != 0
			hm.Better = &b
		}
		out = append(out, hm)
	}
	if len(out) == 0 && len(errs) > 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": errs[0]})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "via": sib.via, "open_url": sib.public, "range": rng, "from": from, "to": today, "metrics": out, "errors": errs,
		"note": fmt.Sprintf("Days with practice compared with days without, over the last %d days. A pattern, not proof.", days)})
}

// healthTest is the Settings connection test for Pocket Health.
func healthTest(ctx context.Context, c *pocket.Client) (string, error) {
	pts, err := c.Metric(ctx, "mindful_min", "w", "")
	if err != nil {
		return "", err
	}
	total := 0.0
	for _, p := range pts {
		total += p.Value
	}
	return fmt.Sprintf("%.0f mindful minutes this week", total), nil
}

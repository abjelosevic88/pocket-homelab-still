package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/abjelosevic88/pocket-homelab-still/internal/config"
	"github.com/abjelosevic88/pocket-homelab-still/internal/content"
	"github.com/abjelosevic88/pocket-homelab-still/internal/store"
)

type app struct {
	t     *testing.T
	url   string
	s     *Server
	st    *store.Store
	alice string
	bob   string
	ids   map[string]int64
}

func newApp(t *testing.T, tweak func(*config.Config)) *app {
	t.Helper()
	cfg := testConfig(t.TempDir())
	cfg.BaseURL = "http://still.test"
	cfg.MaxBodyBytes = 64 << 20
	if tweak != nil {
		tweak(cfg)
	}
	st := openTestStore(t, cfg)
	ctx := context.Background()
	a := &app{t: t, st: st, ids: map[string]int64{}}
	for _, u := range []*store.User{{Email: "alice@example.com", Name: "Alice", Role: "admin"}, {Email: "bob@example.com", Name: "Bob", Role: "member"}} {
		if err := st.CreateUser(ctx, u); err != nil {
			t.Fatal(err)
		}
		tok, _, _ := st.CreateAPIToken(ctx, u.ID, "t")
		a.ids[u.Name] = u.ID
		if u.Name == "Alice" {
			a.alice = tok
		} else {
			a.bob = tok
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	a.s = New(cfg, st, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	if err := a.s.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(a.s.Router())
	t.Cleanup(ts.Close)
	a.url = ts.URL
	return a
}

func (a *app) call(auth, method, path string, body any) (int, map[string]any, []any) {
	a.t.Helper()
	var rd io.Reader
	if body != nil {
		j, _ := json.Marshal(body)
		rd = bytes.NewReader(j)
	}
	req, _ := http.NewRequest(method, a.url+path, rd)
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		a.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var m map[string]any
	var l []any
	if json.Unmarshal(raw, &m) != nil {
		_ = json.Unmarshal(raw, &l)
	}
	return res.StatusCode, m, l
}

func (a *app) ok(auth, method, path string, body any) map[string]any {
	a.t.Helper()
	code, m, _ := a.call(auth, method, path, body)
	if code >= 300 {
		a.t.Fatalf("%s %s → %d %v", method, path, code, m)
	}
	return m
}

func (a *app) expect(want int, auth, method, path string, body any) map[string]any {
	a.t.Helper()
	code, m, _ := a.call(auth, method, path, body)
	if code != want {
		a.t.Fatalf("%s %s → %d, want %d (%v)", method, path, code, want, m)
	}
	return m
}

func num(m map[string]any, k string) float64 { v, _ := m[k].(float64); return v }

// daysAgo is a time n days back at the hour, never later than a minute ago
// (so "today at 21:00" works when the test runs in the morning).
func daysAgo(n int, hour int) string {
	d := time.Now().AddDate(0, 0, -n)
	t := time.Date(d.Year(), d.Month(), d.Day(), hour, 0, 0, 0, time.Local)
	if lim := time.Now().Add(-time.Minute); t.After(lim) {
		t = lim
		if mid := time.Date(d.Year(), d.Month(), d.Day(), 0, 1, 0, 0, time.Local); t.Before(mid) {
			t = mid
		}
	}
	return t.Format(time.RFC3339)
}

func TestStreakOf(t *testing.T) {
	today := "2026-10-25"
	cases := []struct {
		dates     []string
		cur, best int
	}{
		{nil, 0, 0},
		{[]string{"2026-10-25"}, 1, 1},
		{[]string{"2026-10-23", "2026-10-24"}, 2, 2}, // today not done yet: still alive
		{[]string{"2026-10-20", "2026-10-21", "2026-10-22", "2026-10-24", "2026-10-25"}, 2, 3},
		{[]string{"2026-10-01", "2026-10-02"}, 0, 2},
		{[]string{"2026-10-30", "2026-10-31", "2026-11-01"}, 0, 3}, // across a month
	}
	for _, c := range cases {
		cur, best := streakOf(c.dates, today)
		if cur != c.cur || best != c.best {
			t.Errorf("streakOf(%v) = %d/%d, want %d/%d", c.dates, cur, best, c.cur, c.best)
		}
	}
}

func TestContentIsConsistent(t *testing.T) {
	keys := map[string]bool{}
	for _, p := range content.Patterns {
		if err := p.Config.Validate(); err != nil {
			t.Errorf("pattern %s: %v", p.Key, err)
		}
		if keys[p.Key] {
			t.Errorf("duplicate key %s", p.Key)
		}
		keys[p.Key] = true
	}
	for _, s := range content.Styles {
		if keys[s.Key] {
			t.Errorf("duplicate key %s", s.Key)
		}
		keys[s.Key] = true
		if !content.ValidKind(s.Kind) {
			t.Errorf("style %s has kind %s", s.Key, s.Kind)
		}
	}
	lessons := map[string]bool{}
	for _, p := range content.Programs {
		for _, l := range p.Lessons() {
			if lessons[l.Lesson.Key] {
				t.Errorf("duplicate lesson %s", l.Lesson.Key)
			}
			lessons[l.Lesson.Key] = true
			if content.TechniqueName(l.Lesson.Practice.Technique) == "" || l.Lesson.Practice.Minutes <= 0 {
				t.Errorf("lesson %s practises %q", l.Lesson.Key, l.Lesson.Practice.Technique)
			}
		}
	}
	for _, p := range content.Programs {
		if p.Kind == "path" {
			for _, st := range p.Stages {
				if _, ok := content.AchievementByKey("path_" + st.Key); !ok {
					t.Errorf("no milestone for stage %s", st.Key)
				}
			}
		} else if _, ok := content.AchievementByKey("course_" + p.Key); !ok {
			t.Errorf("no milestone for course %s", p.Key)
		}
	}
	if l, n := content.LevelFor(0); l.Name != "Ripple" || n == nil || n.Name != "Stream" {
		t.Fatalf("LevelFor(0) = %v %v", l, n)
	}
	if l, n := content.LevelFor(20000); l.Name != "Ocean" || n != nil {
		t.Fatalf("LevelFor(20000) = %v %v", l, n)
	}
}

func TestPracticeLifecycleAndPerPerson(t *testing.T) {
	a := newApp(t, nil)
	res := a.expect(201, a.alice, "POST", "/api/v1/practices", map[string]any{"technique": "box", "duration_s": 300, "planned_s": 300, "calm_before": 2, "calm_after": 4, "note": "steady", "external_id": "run-1"})
	p := res["practice"].(map[string]any)
	if p["kind"] != "breath" || p["title"] != "Box breathing" || p["source"] != "app" {
		t.Fatalf("practice = %v", p)
	}
	if un, _ := res["unlocked"].([]any); len(un) == 0 {
		t.Fatalf("first practice should unlock milestones: %v", res)
	}
	// the player retrying the same practice updates instead of doubling
	a.expect(201, a.alice, "POST", "/api/v1/practices", map[string]any{"technique": "box", "duration_s": 310, "external_id": "run-1"})
	list := a.ok(a.alice, "GET", "/api/v1/practices", nil)
	if num(list, "total") != 1 {
		t.Fatalf("retry doubled: %v", list)
	}
	id := int64(num(p, "id"))
	// Bob sees nothing of Alice's
	if l := a.ok(a.bob, "GET", "/api/v1/practices", nil); num(l, "total") != 0 {
		t.Fatalf("bob sees alice's practices: %v", l)
	}
	a.expect(404, a.bob, "GET", fmt.Sprintf("/api/v1/practices/%d", id), nil)
	a.expect(404, a.bob, "PUT", fmt.Sprintf("/api/v1/practices/%d", id), map[string]any{"note": "mine"})
	a.expect(404, a.bob, "DELETE", fmt.Sprintf("/api/v1/practices/%d", id), nil)
	if d := a.ok(a.bob, "GET", "/api/v1/dashboard", nil); num(d, "lifetime_min") != 0 {
		t.Fatalf("bob's dashboard counts alice: %v", d)
	}
	// edit, validation, delete
	up := a.ok(a.alice, "PUT", fmt.Sprintf("/api/v1/practices/%d", id), map[string]any{"note": "changed", "calm_after": 5})
	if pp := up["practice"].(map[string]any); pp["note"] != "changed" || num(pp, "calm_after") != 5 || num(pp, "duration_s") != 310 {
		t.Fatalf("update = %v", pp)
	}
	a.expect(400, a.alice, "PUT", fmt.Sprintf("/api/v1/practices/%d", id), map[string]any{"calm_after": 9})
	a.expect(400, a.alice, "POST", "/api/v1/practices", map[string]any{"kind": "juggling", "duration_s": 60})
	a.expect(400, a.alice, "POST", "/api/v1/practices", map[string]any{"technique": "box", "duration_s": 60, "started_at": time.Now().Add(2 * time.Hour).Format(time.RFC3339)})
	a.expect(204, a.alice, "DELETE", fmt.Sprintf("/api/v1/practices/%d", id), nil)
}

func TestPathUnlocksInOrder(t *testing.T) {
	a := newApp(t, nil)
	prog := a.ok(a.alice, "GET", "/api/v1/programs/path", nil)
	next := prog["next"].(map[string]any)
	if next["key"] != "arrive-1" || num(prog, "done") != 0 {
		t.Fatalf("start of the path = %v", next)
	}
	// a later lesson cannot be marked done before the earlier ones
	a.expect(409, a.alice, "POST", "/api/v1/programs/path/lessons/arrive-3/complete", nil)
	// practising a lesson finishes it
	res := a.expect(201, a.alice, "POST", "/api/v1/practices", map[string]any{"technique": "breath_focus", "duration_s": 180, "planned_s": 180, "program": "path", "lesson": "arrive-1"})
	if res["lesson_completed"] != true {
		t.Fatalf("lesson not completed: %v", res)
	}
	// a practice stopped early doesn't finish its lesson
	a.expect(201, a.alice, "POST", "/api/v1/practices", map[string]any{"technique": "breath_focus", "duration_s": 60, "planned_s": 180, "completed": false, "program": "path", "lesson": "arrive-2"})
	prog = a.ok(a.alice, "GET", "/api/v1/programs/path", nil)
	if num(prog, "done") != 1 || prog["next"].(map[string]any)["key"] != "arrive-2" {
		t.Fatalf("progress = %v / %v", prog["done"], prog["next"])
	}
	for _, l := range []string{"arrive-2", "arrive-3", "arrive-4"} {
		a.ok(a.alice, "POST", "/api/v1/programs/path/lessons/"+l+"/complete", nil)
	}
	ach := a.ok(a.alice, "GET", "/api/v1/achievements", nil)
	got := false
	for _, it := range ach["achievements"].([]any) {
		m := it.(map[string]any)
		if m["key"] == "path_arrive" {
			got = m["unlocked"] == true
		}
	}
	if !got {
		t.Fatal("finishing Arrive should unlock its milestone")
	}
	// bob's path is untouched
	if b := a.ok(a.bob, "GET", "/api/v1/programs/path", nil); num(b, "done") != 0 {
		t.Fatalf("bob's path = %v", b["done"])
	}
	a.expect(400, a.alice, "POST", "/api/v1/practices", map[string]any{"technique": "box", "duration_s": 60, "program": "path", "lesson": "nope"})
	a.ok(a.alice, "POST", "/api/v1/programs/path/reset", nil)
	if p := a.ok(a.alice, "GET", "/api/v1/programs/path", nil); num(p, "done") != 0 {
		t.Fatalf("reset = %v", p["done"])
	}
}

func TestStatsAndStreak(t *testing.T) {
	a := newApp(t, nil)
	for i, d := range []int{0, 1, 2, 4} {
		a.expect(201, a.alice, "POST", "/api/v1/practices", map[string]any{"technique": "downshift", "duration_s": 600, "started_at": daysAgo(d, 8), "calm_before": 2, "calm_after": 4, "external_id": fmt.Sprint("s", i)})
	}
	a.expect(201, a.alice, "POST", "/api/v1/practices", map[string]any{"technique": "silent", "duration_s": 1500, "started_at": daysAgo(0, 21)})
	st := a.ok(a.alice, "GET", "/api/v1/stats?range=w", nil)
	totals := st["totals"].(map[string]any)
	if num(totals, "sessions") != 5 || num(totals, "minutes") != 65 || num(totals, "days_practiced") != 4 {
		t.Fatalf("totals = %v", totals)
	}
	streak := st["streak"].(map[string]any)
	if num(streak, "current") != 3 || num(streak, "longest") != 3 {
		t.Fatalf("streak = %v", streak)
	}
	mood := st["mood"].(map[string]any)
	if num(mood, "avg_shift") != 2 || num(mood, "improved_pct") != 100 {
		t.Fatalf("mood = %v", mood)
	}
	if len(st["daily"].([]any)) != 7 || len(st["heatmap"].([]any)) < 365 {
		t.Fatal("series lengths")
	}
	techs := st["by_technique"].([]any)
	if techs[0].(map[string]any)["key"] != "downshift" {
		t.Fatalf("by technique = %v", techs)
	}
	home := a.ok(a.alice, "GET", "/api/v1/home", nil)
	if num(home, "streak") != 3 || home["suggestion"] == nil || home["next_lesson"] == nil {
		t.Fatalf("home = %v", home)
	}
	day := a.ok(a.alice, "GET", "/api/v1/day/"+store.Today(), nil)
	if num(day, "sessions") != 2 || num(day, "minutes") != 35 || day["summary"] == "" {
		t.Fatalf("day = %v", day)
	}
	ov := a.ok(a.alice, "GET", "/api/v1/overlay?from="+store.AddDays(store.Today(), -7)+"&to="+store.Today(), nil)
	if len(ov["items"].([]any)) != 5 {
		t.Fatalf("overlay = %v", ov)
	}
	// the 25-minute sit unlocked "Twenty minutes"
	ach := a.ok(a.alice, "GET", "/api/v1/achievements", nil)
	if num(ach, "unlocked") < 4 {
		t.Fatalf("milestones = %v", ach["unlocked"])
	}
}

func TestPresetsAndFavorites(t *testing.T) {
	a := newApp(t, nil)
	p := a.expect(201, a.alice, "POST", "/api/v1/presets", map[string]any{"kind": "breath", "name": "Mine", "config": map[string]any{"phases": []map[string]any{{"type": "in", "s": 5}, {"type": "out", "s": 7}}, "minutes": 4}})
	a.expect(400, a.alice, "POST", "/api/v1/presets", map[string]any{"kind": "breath", "name": "Bad", "config": map[string]any{"phases": []map[string]any{{"type": "hold", "s": 5}}}})
	a.expect(400, a.alice, "POST", "/api/v1/presets", map[string]any{"kind": "timer", "name": "Bad", "config": map[string]any{"minutes": 10, "end_bell": "kazoo"}})
	a.expect(201, a.alice, "POST", "/api/v1/presets", map[string]any{"kind": "timer", "name": "Sit", "config": map[string]any{"minutes": 20, "interval_min": 5, "end_bell": "gong", "ambient": "rain", "style": "breath_focus", "guidance": "light"}})
	a.expect(404, a.bob, "PUT", fmt.Sprintf("/api/v1/presets/%d", int64(num(p, "id"))), map[string]any{"name": "x"})
	a.ok(a.alice, "PUT", "/api/v1/favorites/box", map[string]any{"on": true})
	a.expect(404, a.alice, "PUT", "/api/v1/favorites/nothing", map[string]any{"on": true})
	lib := a.ok(a.alice, "GET", "/api/v1/library", nil)
	if lib["favorites"].(map[string]any)["box"] != true || len(lib["presets"].([]any)) != 2 || len(lib["patterns"].([]any)) != len(content.Patterns) {
		t.Fatalf("library = %v", lib)
	}
	if bl := a.ok(a.bob, "GET", "/api/v1/library", nil); len(bl["presets"].([]any)) != 0 || len(bl["favorites"].(map[string]any)) != 0 {
		t.Fatalf("bob's library = %v", bl)
	}
}

func TestLinkScopes(t *testing.T) {
	get := func(p string) *http.Request { r, _ := http.NewRequest("GET", p, nil); return r }
	post := func(p string) *http.Request { r, _ := http.NewRequest("POST", p, nil); return r }
	if !linkAllows([]string{"mindfulness:read"}, get("/api/v1/day/2026-10-23")) || !linkAllows([]string{"mindfulness:read"}, get("/api/v1/overlay")) {
		t.Fatal("read should allow day and overlay")
	}
	if linkAllows([]string{"mindfulness:read"}, post("/api/v1/practices")) {
		t.Fatal("read must not write")
	}
	if linkAllows([]string{"mindfulness:read"}, get("/api/v1/settings")) || linkAllows([]string{"mindfulness:read"}, get("/api/v1/export.json")) {
		t.Fatal("links never see settings or exports")
	}
	if !linkAllows([]string{"mindfulness:write"}, post("/api/v1/practices")) || linkAllows([]string{"mindfulness:write"}, get("/api/v1/practices")) {
		t.Fatal("write may log but not read")
	}
}

func TestExportImportIsIdempotent(t *testing.T) {
	a := newApp(t, nil)
	a.expect(201, a.alice, "POST", "/api/v1/practices", map[string]any{"technique": "coherent", "duration_s": 600, "started_at": daysAgo(1, 7)})
	a.expect(201, a.alice, "POST", "/api/v1/practices", map[string]any{"technique": "breath_focus", "duration_s": 180, "program": "path", "lesson": "arrive-1"})
	_, doc, _ := a.call(a.alice, "GET", "/api/v1/export.json", nil)
	r1 := a.ok(a.bob, "POST", "/api/v1/import", doc)
	if num(r1, "practices") != 2 || num(r1, "lessons") != 1 {
		t.Fatalf("import = %v", r1)
	}
	a.ok(a.bob, "POST", "/api/v1/import", doc)
	if l := a.ok(a.bob, "GET", "/api/v1/practices", nil); num(l, "total") != 2 {
		t.Fatalf("second import doubled: %v", l["total"])
	}
	if l := a.ok(a.alice, "GET", "/api/v1/practices", nil); num(l, "total") != 2 {
		t.Fatalf("alice changed: %v", l["total"])
	}
}

func TestSettingsPerPerson(t *testing.T) {
	a := newApp(t, nil)
	a.ok(a.alice, "PUT", "/api/v1/settings", map[string]any{"daily_goal_min": 20, "habits_token": "ph_secretsecret", "title": "Family Still", "reminder_times": []string{"21:00", "07:30"}})
	bob := a.ok(a.bob, "GET", "/api/v1/settings", nil)
	if num(bob, "daily_goal_min") != 10 || bob["habits_token"] != "" || bob["title"] != "Family Still" {
		t.Fatalf("bob's settings leak: %v", bob)
	}
	a.ok(a.bob, "PUT", "/api/v1/settings", map[string]any{"title": "Bob's"})
	al := a.ok(a.alice, "GET", "/api/v1/settings", nil)
	if al["title"] != "Family Still" || al["habits_token"] == "" || strings.Contains(al["habits_token"].(string), "secret") {
		t.Fatalf("alice settings = %v", al)
	}
	if rt := al["reminder_times"].([]any); len(rt) != 2 || rt[0] != "07:30" {
		t.Fatalf("reminders = %v", rt)
	}
	a.expect(400, a.alice, "PUT", "/api/v1/settings", map[string]any{"reminder_times": []string{"25:00"}})
	a.expect(400, a.alice, "PUT", "/api/v1/settings", map[string]any{"bell": "kazoo"})
}

func TestWithinMinutes(t *testing.T) {
	if !withinMinutes("08:02", "08:00", 4) || withinMinutes("07:59", "08:00", 4) || withinMinutes("08:05", "08:00", 4) {
		t.Fatal("withinMinutes")
	}
}

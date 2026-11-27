package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-still/internal/config"
	"github.com/abjelosevic88/pocket-homelab-still/internal/store"
)

// capLinks starts Still behind a fake Core whose pcx_<who>_<search|day|widget|
// read|write> tokens are links of Pocket Hub acting for alice@ or bob@.
func capLinks(t *testing.T) *app {
	t.Helper()
	coreSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/v1/auth/introspect" {
			io.WriteString(w, `{}`)
			return
		}
		var in struct {
			Token string `json:"token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		parts := strings.SplitN(strings.TrimPrefix(in.Token, "pcx_"), "_", 2)
		scopes := map[string]string{"search": core.ScopeSearch, "day": core.ScopeDay, "widget": core.ScopeWidget, "capture": core.ScopeCapture,
			"read": "mindfulness:read", "write": "mindfulness:write"}
		if len(parts) != 2 || scopes[parts[1]] == "" {
			io.WriteString(w, `{"active":false}`)
			return
		}
		email := parts[0] + "@example.com"
		fmt.Fprintf(w, `{"active":true,"kind":"delegated","scope":%q,"audience":"still","user":{"id":1,"sub":%q,"email":%q,"name":%q,"role":"member"},"app":{"id":9,"slug":"hub","name":"Pocket Hub"}}`,
			scopes[parts[1]], email, email, parts[0])
	}))
	t.Cleanup(coreSrv.Close)
	return newApp(t, func(c *config.Config) { c.CoreURL = coreSrv.URL })
}

// get reads a route's JSON answer into out.
func get[T any](a *app, tok, path string) T {
	a.t.Helper()
	var out T
	code, body := a.raw(tok, "GET", path, "", nil)
	if code != 200 {
		a.t.Fatalf("GET %s → %d %s", path, code, body)
	}
	if err := json.Unmarshal(body, &out); err != nil {
		a.t.Fatalf("GET %s: %v %s", path, err, body)
	}
	return out
}

func searchFor(a *app, tok, q string) []core.SearchItem {
	a.t.Helper()
	return get[core.SearchResults](a, tok, "/api/v1/pocket/search?q="+url.QueryEscape(q)+"&limit=50").Items
}

func TestPocketSearch(t *testing.T) {
	a := newApp(t, nil)
	pre := a.expect(201, a.alice, "POST", "/api/v1/presets", map[string]any{"kind": "breath", "name": "My box", "config": map[string]any{"phases": []map[string]any{{"type": "in", "s": 4}, {"type": "out", "s": 4}}, "minutes": 6}})
	a.expect(201, a.alice, "POST", "/api/v1/presets", map[string]any{"kind": "timer", "name": "Lake sit", "config": map[string]any{"minutes": 20}})
	ses := a.expect(201, a.alice, "POST", "/api/v1/practices", map[string]any{"technique": "silent", "duration_s": 900, "started_at": daysAgo(1, 7), "note": "Quiet morning by the LAKE"})["practice"].(map[string]any)
	a.expect(201, a.alice, "POST", "/api/v1/practices", map[string]any{"technique": "box", "duration_s": 300, "note": "nothing to see"})
	a.expect(201, a.bob, "POST", "/api/v1/presets", map[string]any{"kind": "timer", "name": "Secret box lake", "config": map[string]any{"minutes": 5}})
	a.expect(201, a.bob, "POST", "/api/v1/practices", map[string]any{"technique": "box", "duration_s": 300, "note": "secret lake box"})

	// the person's own pattern first, then the built-in ones
	box := searchFor(a, a.alice, "box")
	if len(box) != 3 {
		t.Fatalf("box: %+v", box)
	}
	want := []core.SearchItem{
		{ID: fmt.Sprintf("practice:preset:%d", int64(num(pre, "id"))), Title: "My box", Snippet: "6 min · Your breathing pattern", Kind: "practice", URL: fmt.Sprintf("/breathe/preset:%d", int64(num(pre, "id")))},
		{ID: "practice:box", Title: "Box breathing", Snippet: "4 min · Breathing pattern", Kind: "practice", URL: "/breathe/box"},
		{ID: "lesson:path/settle-4", Title: "Box breathing", Snippet: "5 min · Lesson 8 of The Path", Kind: "lesson", URL: "/path/path/settle-4"},
	}
	for i, w := range want {
		g := box[i]
		if g.ID != w.ID || g.Title != w.Title || g.Snippet != w.Snippet || g.Kind != w.Kind || g.URL != w.URL {
			t.Errorf("box[%d] = %+v, want %+v", i, g, w)
		}
	}
	if box[0].Score <= box[1].Score {
		t.Errorf("own things should rank first: %v %v", box[0].Score, box[1].Score)
	}

	// a session by its note (any case), and a timer by its name; the note is never sent back
	lake := searchFor(a, a.alice, "lake")
	if len(lake) != 2 || lake[0].Title != "Lake sit" || lake[0].URL != "/meditate" || lake[0].Snippet != "20 min · Your meditation timer" {
		t.Fatalf("lake: %+v", lake)
	}
	s := lake[1]
	day, _ := time.Parse("2006-01-02", store.AddDays(store.Today(), -1))
	if s.Kind != "session" || s.Title != "Silent sit, "+day.Format("2 Jan 2006") || s.Snippet != "15 min · Meditation" || s.URL != fmt.Sprintf("/journal/%d", int64(num(ses, "id"))) {
		t.Errorf("session: %+v", s)
	}
	raw, _ := json.Marshal(lake)
	if strings.Contains(strings.ToLower(string(raw)), "quiet morning") || strings.Contains(strings.ToLower(string(raw)), "secret") {
		t.Errorf("a note or another person's things came back: %s", raw)
	}
	// every word must be there
	if got := searchFor(a, a.alice, "morning lake"); len(got) != 1 || got[0].Kind != "session" {
		t.Errorf("morning lake: %+v", got)
	}

	// courses and lessons
	sleep := searchFor(a, a.alice, "sleep")
	if len(sleep) == 0 || sleep[0].ID != "course:sleep" || sleep[0].Snippet != "5 lessons · Course" || sleep[0].URL != "/path/sleep" {
		t.Fatalf("sleep: %+v", sleep)
	}
	if p := searchFor(a, a.alice, "the path"); len(p) == 0 || p[0].ID != "course:path" || p[0].Snippet != "28 lessons · Path in 7 stages" {
		t.Errorf("the path: %+v", p)
	}
	if st := searchFor(a, a.alice, "body scan"); len(st) == 0 || st[0].ID != "practice:body_scan" || st[0].URL != "/meditate" || st[0].Snippet != "12 min · Body scan" {
		t.Errorf("body scan: %+v", st)
	}

	// nothing to look for, nothing found; a wildcard is no wildcard
	for _, q := range []string{"??", "*", "%", "_", "zzzz"} {
		if got := searchFor(a, a.alice, q); len(got) != 0 {
			t.Errorf("%q found %+v", q, got)
		}
	}
	// Bob finds his own and none of Alice's
	if got := searchFor(a, a.bob, "lake"); len(got) != 2 || got[0].Title != "Secret box lake" || got[1].Kind != "session" {
		t.Errorf("bob: %+v", got)
	}
	if got := get[core.SearchResults](a, a.alice, "/api/v1/pocket/search?q=box&limit=2").Items; len(got) != 2 || got[0].Title != "My box" {
		t.Errorf("limit: %+v", got)
	}
}

func TestPocketDay(t *testing.T) {
	a := newApp(t, nil)
	today, at8 := store.Today(), daysAgo(0, 8)
	a.expect(201, a.alice, "POST", "/api/v1/practices", map[string]any{"technique": "breath_focus", "duration_s": 180, "planned_s": 180, "program": "path", "lesson": "arrive-1",
		"started_at": daysAgo(0, 7), "note": "felt my feet"})
	a.expect(201, a.alice, "POST", "/api/v1/practices", map[string]any{"technique": "box", "duration_s": 240, "started_at": at8})
	a.expect(201, a.alice, "POST", "/api/v1/practices", map[string]any{"technique": "box", "duration_s": 600, "started_at": daysAgo(1, 8)})
	a.expect(201, a.bob, "POST", "/api/v1/practices", map[string]any{"technique": "silent", "duration_s": 1800, "note": "bob's secret"})

	day := get[core.Day](a, a.alice, "/api/v1/pocket/day/"+today)
	old := a.ok(a.alice, "GET", "/api/v1/day/"+today, nil)
	if day.Date != today || day.Summary == "" || day.Summary != old["summary"] {
		t.Fatalf("summary %q, the day page has %q", day.Summary, old["summary"])
	}
	kinds := map[string][]core.DayItem{}
	for _, it := range day.Items {
		kinds[it.Kind] = append(kinds[it.Kind], it)
	}
	ses := kinds["session"]
	if len(ses) != 2 || ses[0].Title != "Breath focus" || ses[0].Minutes != 3 || ses[0].Note != "felt my feet" || ses[0].Sub != "Meditation · The Path: Just three minutes" ||
		ses[1].Title != "Box breathing" || ses[1].Minutes != 4 || ses[1].Sub != "Breathing" || !strings.HasPrefix(ses[1].URL, "/journal/") {
		t.Errorf("sessions: %+v", ses)
	}
	if want := clock(at8, nil); ses[1].Time != want || ses[1].End == "" {
		t.Errorf("time %q end %q, want %q", ses[1].Time, ses[1].End, want)
	}
	if l := kinds["lesson"]; len(l) != 1 || l[0].Title != "Just three minutes" || !l[0].Done || l[0].Sub != "The Path · Arrive" || l[0].URL != "/path/path/arrive-1" || l[0].ID != "lesson:path/arrive-1" {
		t.Errorf("lessons: %+v", l)
	}
	if m := kinds["milestone"]; len(m) == 0 || !m[0].Done || m[0].URL != "/milestones" || !strings.HasPrefix(m[0].ID, "milestone:") {
		t.Errorf("milestones: %+v", m)
	}
	if len(day.Metrics) != 2 || day.Metrics[0] != (core.Metric{Label: "Minutes", Value: "7 of 10"}) || day.Metrics[1] != (core.Metric{Label: "Streak", Value: "2 days"}) {
		t.Errorf("metrics: %+v", day.Metrics)
	}
	raw, _ := json.Marshal(day)
	if strings.Contains(string(raw), "secret") {
		t.Errorf("bob's things in alice's day: %s", raw)
	}

	// yesterday: one session, nothing else; a day with nothing is empty
	y := get[core.Day](a, a.alice, "/api/v1/pocket/day/"+store.AddDays(today, -1))
	if len(y.Items) != 1 || y.Items[0].Kind != "session" || y.Metrics[0].Value != "10 of 10" || y.Metrics[1].Value != "1 day" {
		t.Errorf("yesterday: %+v", y)
	}
	if e := get[core.Day](a, a.alice, "/api/v1/pocket/day/2001-01-01"); len(e.Items) != 0 || e.Summary != "" || e.Metrics != nil {
		t.Errorf("empty day: %+v", e)
	}
	if b := get[core.Day](a, a.bob, "/api/v1/pocket/day/"+today); len(b.Items) == 0 || b.Items[0].Title != "Silent sit" || b.Metrics[0].Value != "30 of 10" {
		t.Errorf("bob's day: %+v", b)
	}
	a.expect(400, a.alice, "GET", "/api/v1/pocket/day/2026-13-01", nil)

	// the caller's time zone sets the clock
	tz := get[core.Day](a, a.alice, "/api/v1/pocket/day/"+today+"?tz=Pacific/Kiritimati")
	start, _ := store.ParseTime(at8)
	kir, _ := time.LoadLocation("Pacific/Kiritimati")
	found := false
	for _, it := range tz.Items {
		if it.Kind == "session" && it.Title == "Box breathing" {
			found = it.Time == start.In(kir).Format("15:04")
		}
	}
	if !found {
		t.Errorf("time zone: %+v", tz.Items)
	}
}

func TestPocketWidget(t *testing.T) {
	a := newApp(t, nil)
	// someone new: nothing yet, and the first lesson of the Path
	w := get[core.WidgetData](a, a.alice, "/api/v1/pocket/widget")
	if w.Line != "Nothing yet today" || w.URL != "/" || len(w.Items) != 1 || w.Items[0].Title != "Just three minutes" || w.Items[0].URL != "/path/path/arrive-1" ||
		w.Items[0].Detail != "Next lesson · The Path" {
		t.Fatalf("new: %+v", w)
	}
	if len(w.Metrics) != 3 || w.Metrics[0] != (core.Metric{Label: "Today", Value: "0 min"}) || w.Metrics[1] != (core.Metric{Label: "Goal", Value: "10 min"}) || w.Metrics[2].Value != "0 days" {
		t.Errorf("metrics: %+v", w.Metrics)
	}

	// practice without a course: the most used one
	for i, d := range []int{0, 1, 2} {
		a.expect(201, a.alice, "POST", "/api/v1/practices", map[string]any{"technique": "downshift", "duration_s": 120, "started_at": daysAgo(d, 8), "external_id": fmt.Sprint("d", i)})
	}
	a.expect(201, a.alice, "POST", "/api/v1/practices", map[string]any{"technique": "box", "duration_s": 360, "started_at": daysAgo(0, 9)})
	w = get[core.WidgetData](a, a.alice, "/api/v1/pocket/widget")
	if w.Line != "8 of 10 minutes today, 3 days in a row" || w.Items[0].Title != "Long exhale" || w.Items[0].URL != "/breathe/downshift" || w.Items[0].Detail != "Your most used practice" {
		t.Fatalf("practising: %+v", w)
	}
	if w.Metrics[0].Value != "8 min" || w.Metrics[2].Value != "3 days" {
		t.Errorf("metrics: %+v", w.Metrics)
	}

	// a course in progress comes first
	a.ok(a.alice, "POST", "/api/v1/programs/sleep/start", nil)
	w = get[core.WidgetData](a, a.alice, "/api/v1/pocket/widget")
	if w.Items[0].Title != "Slow down" || w.Items[0].URL != "/path/sleep/sleep-1" || w.Items[0].Detail != "Next lesson · Sleep well" {
		t.Errorf("course: %+v", w.Items)
	}

	// no goal: just the minutes; Bob sees none of it
	a.ok(a.alice, "PUT", "/api/v1/settings", map[string]any{"daily_goal_min": 0})
	if w = get[core.WidgetData](a, a.alice, "/api/v1/pocket/widget"); w.Line != "8 minutes today, 3 days in a row" || len(w.Metrics) != 2 {
		t.Errorf("no goal: %+v", w)
	}
	if b := get[core.WidgetData](a, a.bob, "/api/v1/pocket/widget"); b.Line != "Nothing yet today" || b.Items[0].Title != "Just three minutes" {
		t.Errorf("bob: %+v", b)
	}
}

func TestGoalWords(t *testing.T) {
	for _, c := range []struct {
		min  float64
		goal int
		want string
	}{{8, 10, "8 of 10"}, {9.6, 10, "9 of 10"}, {9.995, 10, "10 of 10"}, {12.4, 10, "12 of 10"}, {0.2, 10, "1 of 10"}, {7.5, 0, "8"}, {0, 0, "0"}} {
		if got := goalWords(c.min, c.goal); got != c.want {
			t.Errorf("goalWords(%v, %d) = %q, want %q", c.min, c.goal, got, c.want)
		}
	}
}

func TestPocketLinkTokens(t *testing.T) {
	a := capLinks(t)
	today := store.Today()
	a.expect(201, a.bob, "POST", "/api/v1/presets", map[string]any{"kind": "timer", "name": "Lake sit", "config": map[string]any{"minutes": 20}})
	a.expect(201, a.bob, "POST", "/api/v1/practices", map[string]any{"technique": "box", "duration_s": 300, "note": "calm by the lake"})
	a.expect(201, a.alice, "POST", "/api/v1/presets", map[string]any{"kind": "timer", "name": "Secret lake", "config": map[string]any{"minutes": 5}})
	a.expect(201, a.alice, "POST", "/api/v1/practices", map[string]any{"technique": "box", "duration_s": 300, "note": "Secret lake note"})

	routes := []struct{ scope, path string }{
		{"search", "/api/v1/pocket/search?q=lake"},
		{"day", "/api/v1/pocket/day/" + today + "?tz=Europe/Sarajevo"},
		{"widget", "/api/v1/pocket/widget"},
	}
	for _, tok := range []string{"search", "day", "widget", "capture", "read", "write"} {
		for _, rt := range routes {
			want := 403
			if rt.scope == tok {
				want = 200
			}
			code, body := a.raw("pcx_bob_"+tok, "GET", rt.path, "", nil)
			if code != want {
				t.Errorf("pcx_bob_%s GET %s → %d, want %d", tok, rt.path, code, want)
			}
			if strings.Contains(string(body), "Secret") {
				t.Errorf("pcx_bob_%s %s: another person's things: %s", tok, rt.path, body)
			}
		}
		// no capture here, whatever the link (the route does not exist)
		if code, _ := a.raw("pcx_bob_"+tok, "POST", core.CapturePath, "application/json", []byte(`{"request_id":"hub-0000000000000001","text":"10 min breathing"}`)); code != 403 && code != 404 {
			t.Errorf("pcx_bob_%s capture → %d, want 403 or 404", tok, code)
		}
		if code, body := a.raw("pcx_bob_"+tok, "GET", "/api/v1/auth/me", "", nil); code != 200 || !strings.Contains(string(body), "bob@example.com") {
			t.Errorf("pcx_bob_%s /auth/me → %d %s", tok, code, body)
		}
		if tok == "read" || tok == "write" {
			continue
		}
		// a capability opens its one route with its one method, nothing else of the app
		for _, rt := range []struct{ method, path string }{
			{"GET", "/api/v1/practices"}, {"GET", "/api/v1/day/" + today}, {"GET", "/api/v1/dashboard"}, {"GET", "/api/v1/home"}, {"GET", "/api/v1/library"},
			{"POST", "/api/v1/practices"}, {"GET", "/api/v1/settings"}, {"GET", "/api/v1/export.json"}, {"GET", core.PortablePath},
		} {
			if code, _ := a.raw("pcx_bob_"+tok, rt.method, rt.path, "application/json", []byte(`{}`)); code != 403 {
				t.Errorf("pcx_bob_%s %s %s → %d, want 403", tok, rt.method, rt.path, code)
			}
		}
	}
	for _, rt := range []struct{ tok, method, path string }{{"search", "POST", "/api/v1/pocket/search?q=lake"}, {"widget", "DELETE", "/api/v1/pocket/widget"}, {"day", "PUT", "/api/v1/pocket/day/" + today}} {
		if code, _ := a.raw("pcx_bob_"+rt.tok, rt.method, rt.path, "", nil); code != 403 && code != 405 {
			t.Errorf("pcx_bob_%s %s %s → %d", rt.tok, rt.method, rt.path, code)
		}
	}

	// what came back is Bob's; through a link his note matches but stays out of the day
	found := searchFor(a, "pcx_bob_search", "lake")
	if len(found) != 2 || found[0].Title != "Lake sit" || found[1].Kind != "session" {
		t.Errorf("link search: %+v", found)
	}
	day := get[core.Day](a, "pcx_bob_day", "/api/v1/pocket/day/"+today)
	if len(day.Items) == 0 || day.Items[0].Title != "Box breathing" || day.Items[0].Note != "" {
		t.Errorf("link day: %+v", day)
	}
	if own := get[core.Day](a, a.bob, "/api/v1/pocket/day/"+today); own.Items[0].Note != "calm by the lake" {
		t.Errorf("bob's own day: %+v", own.Items)
	}
	w := get[core.WidgetData](a, "pcx_bob_widget", "/api/v1/pocket/widget")
	if w.Line != "5 of 10 minutes today" {
		t.Errorf("link widget: %+v", w)
	}
}

// The manifest Core gets says what Still can do, so nobody has to list it.
func TestManifestOffersCapabilities(t *testing.T) {
	a := newApp(t, nil)
	b, err := json.Marshal(a.s.manifest())
	if err != nil {
		t.Fatal(err)
	}
	var sent map[string]any
	_ = json.Unmarshal(b, &sent)
	if sent["slug"] != "still" || sent["category"] != "Health" {
		t.Errorf("manifest: slug %v, category %v", sent["slug"], sent["category"])
	}
	for _, k := range []string{"search", "day", "widget"} {
		if sent[k] == nil {
			t.Fatalf("manifest has no %q", k)
		}
	}
	if sent["capture"] != nil {
		t.Errorf("Still keeps no quick line: %v", sent["capture"])
	}
	if bangs, _ := json.Marshal(sent["search"].(map[string]any)["bangs"]); string(bangs) != `["still","meditate"]` {
		t.Errorf("bangs: %s", bangs)
	}
	if d := sent["day"].(map[string]any); d["label"] != "Still" || d["one"] != "session" || d["many"] != "sessions" || d["unit"] != "min" {
		t.Errorf("day: %v", d)
	}
	if w := sent["widget"].(map[string]any); w["title"] != "Still" {
		t.Errorf("widget: %v", w)
	}
	// Register adds the pocket:* offers next to Still's own
	m := a.s.manifest()
	have := map[string]bool{}
	for _, o := range append(append([]core.Offer{}, m.Offers...), m.Capabilities.Offers()...) {
		have[o.Scope] = true
	}
	for _, sc := range []string{"mindfulness:read", "mindfulness:write", core.ScopeSearch, core.ScopeDay, core.ScopeWidget} {
		if !have[sc] {
			t.Errorf("offer %s missing", sc)
		}
	}
	if have[core.ScopeCapture] {
		t.Error("pocket:capture offered")
	}
}

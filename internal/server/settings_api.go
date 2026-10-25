package server

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-still/internal/ai"
	"github.com/abjelosevic88/pocket-homelab-still/internal/content"
	"github.com/abjelosevic88/pocket-homelab-still/internal/notify"
	"github.com/abjelosevic88/pocket-homelab-still/internal/pocket"
	"github.com/abjelosevic88/pocket-homelab-still/internal/store"
)

// ---- settings ----

func mask(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "••••"
	}
	return s[:4] + "••••" + s[len(s)-4:]
}

// publicSettings is what one person sees: their settings with the secrets
// masked and, per Pocket app, the mode in effect (<app>_mode), whether it
// works and how (<app>_via: core | direct | "") and why not (<app>_detail).
func (s *Server) publicSettings(ctx context.Context, u *store.User, st store.Settings) map[string]any {
	if st.ReminderTimes == nil {
		st.ReminderTimes = []string{}
	}
	out := map[string]any{
		"core_configured": s.core.Configured(), "core_registered": s.coreStatus()["registered"],
		"title": st.Title, "date_format": st.DateFormat, "time_format": st.TimeFormat, "week_start": st.WeekStart,
		"daily_goal_min": st.DailyGoalMin, "weekly_goal_days": st.WeeklyGoalDays,
		"bell": st.Bell, "ambient": st.Ambient, "breath_sound": st.BreathSound, "volume": st.Volume, "voice": st.Voice, "voice_name": st.VoiceName,
		"haptics": st.Haptics, "ask_calm": st.AskCalm, "keep_awake": st.KeepAwake, "player_theme": st.PlayerTheme, "breath_visual": st.BreathVisual, "default_sit_min": st.DefaultSitMin,
		"reminder_times": st.ReminderTimes, "reminder_days": st.ReminderDays, "reminder_skip_done": st.ReminderSkipDone,
		"notify_streak": st.NotifyStreak, "streak_hour": st.StreakHour, "notify_milestones": st.NotifyMilestones,
		"notify_weekly": st.NotifyWeekly, "weekly_day": st.WeeklyDay, "weekly_hour": st.WeeklyHour,
		"notify_url": st.NotifyURL, "notify_format": st.NotifyFormat, "notify_configured": st.NotifyURL != "",
		"habits_url": st.HabitsURL, "habits_token": mask(st.HabitsToken), "habits_public_url": st.HabitsPublicURL,
		"health_url": st.HealthURL, "health_token": mask(st.HealthToken), "health_public_url": st.HealthPublicURL,
		"health_sync": st.HealthSync, "health_insights": st.HealthInsights,
		"music_url": st.MusicURL, "music_user": st.MusicUser, "music_password": mask(st.MusicPassword),
		"music_mode":  s.connMode(st.MusicMode, st.MusicPassword),
		"ai_provider": st.AIProvider, "ai_pocket_model": st.AIPocketModel, "openai_api_key": mask(st.OpenAIKey), "openai_model": st.OpenAIModel, "openai_base_url": st.OpenAIBaseURL,
		"setup_complete": st.SetupComplete, "base_url": s.cfg.BaseURL,
	}
	for _, a := range pocketApps {
		mode, via, detail := s.connStatus(ctx, u, st, a)
		out[a.slug+"_mode"], out[a.slug+"_via"], out[a.slug+"_detail"] = mode, via, detail
		out[a.slug+"_configured"] = via != ""
		_, _, _, public := connSettings(st, a)
		if public == "" && via != "" {
			public = s.resolveSibling(ctx, a.slug)
		}
		out[a.slug+"_open_url"] = public
	}
	return out
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.GetSettings(r.Context(), actorID(r.Context()))
	if err != nil {
		s.fail(w, err, "settings")
		return
	}
	writeJSON(w, http.StatusOK, s.publicSettings(r.Context(), userFrom(r.Context()), st))
}

var hhmm = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

func validAmbient(v string) bool {
	if strings.HasPrefix(v, "music:") && len(v) > 6 && len(v) < 120 {
		return true
	}
	for _, a := range content.Ambients {
		if a == v {
			return true
		}
	}
	return false
}

// handlePutSettings saves the caller's personal settings. The box-wide title
// is only taken from an admin.
func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())
	isAdmin := me.Role == "admin"
	cur, err := s.store.GetSettings(r.Context(), me.ID)
	if err != nil {
		s.fail(w, err, "settings")
		return
	}
	var in store.Settings
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	bad := func(msg string) { writeErr(w, http.StatusBadRequest, msg) }
	if keys["title"] && isAdmin {
		cur.Title = strings.TrimSpace(in.Title)
		if cur.Title == "" {
			cur.Title = store.DefaultTitle
		}
	}
	if keys["date_format"] {
		cur.DateFormat = in.DateFormat
	}
	if keys["time_format"] {
		cur.TimeFormat = in.TimeFormat
	}
	if keys["week_start"] && (in.WeekStart == 1 || in.WeekStart == 7 || in.WeekStart == 6) {
		cur.WeekStart = in.WeekStart
	}
	intIn := func(k string, dst *int, v, lo, hi int) bool {
		if !keys[k] {
			return true
		}
		if v < lo || v > hi {
			bad(fmt.Sprintf("%s is %d to %d", k, lo, hi))
			return false
		}
		*dst = v
		return true
	}
	if !intIn("daily_goal_min", &cur.DailyGoalMin, in.DailyGoalMin, 0, 600) || !intIn("weekly_goal_days", &cur.WeeklyGoalDays, in.WeeklyGoalDays, 0, 7) ||
		!intIn("streak_hour", &cur.StreakHour, in.StreakHour, 0, 23) || !intIn("weekly_day", &cur.WeeklyDay, in.WeeklyDay, 1, 7) ||
		!intIn("weekly_hour", &cur.WeeklyHour, in.WeeklyHour, 0, 23) {
		return
	}
	pick := func(k string, dst *string, v string, allowed []string) bool {
		if !keys[k] {
			return true
		}
		for _, a := range allowed {
			if v == a {
				*dst = v
				return true
			}
		}
		bad(fmt.Sprintf("%s must be one of %s", k, strings.Join(allowed, ", ")))
		return false
	}
	if !pick("bell", &cur.Bell, in.Bell, content.Bells) || !pick("breath_sound", &cur.BreathSound, in.BreathSound, []string{"soft", "bell", "none"}) ||
		!pick("player_theme", &cur.PlayerTheme, in.PlayerTheme, []string{"dusk", "ocean", "forest", "night", "minimal"}) ||
		!pick("breath_visual", &cur.BreathVisual, in.BreathVisual, []string{"orb", "box", "wave"}) {
		return
	}
	if keys["ambient"] {
		if !validAmbient(in.Ambient) {
			bad("unknown background sound")
			return
		}
		cur.Ambient = in.Ambient
	}
	if keys["volume"] {
		if in.Volume < 0 || in.Volume > 1 {
			bad("volume is 0 to 1")
			return
		}
		cur.Volume = in.Volume
	}
	if keys["default_sit_min"] {
		if in.DefaultSitMin < 1 || in.DefaultSitMin > 240 {
			bad("a sit is 1 to 240 minutes")
			return
		}
		cur.DefaultSitMin = in.DefaultSitMin
	}
	if keys["reminder_times"] {
		ts := []string{}
		seen := map[string]bool{}
		for _, t := range in.ReminderTimes {
			t = strings.TrimSpace(t)
			if !hhmm.MatchString(t) {
				bad("reminder times are HH:MM")
				return
			}
			if !seen[t] {
				seen[t] = true
				ts = append(ts, t)
			}
		}
		if len(ts) > 6 {
			bad("up to six reminders a day")
			return
		}
		sort.Strings(ts)
		cur.ReminderTimes = ts
	}
	if keys["reminder_days"] {
		d := ""
		for _, c := range "1234567" {
			if strings.ContainsRune(in.ReminderDays, c) {
				d += string(c)
			}
		}
		cur.ReminderDays = d
	}
	secret := func(k string, dst *string, v string) {
		if keys[k] && !strings.Contains(v, "••") {
			*dst = strings.TrimSpace(v)
		}
	}
	url := func(k string, dst *string, v string) {
		if keys[k] {
			*dst = strings.TrimRight(strings.TrimSpace(v), "/")
		}
	}
	text := func(k string, dst *string, v string) {
		if keys[k] {
			*dst = strings.TrimSpace(v)
		}
	}
	boolIn := func(k string, dst *bool, v bool) {
		if keys[k] {
			*dst = v
		}
	}
	boolIn("voice", &cur.Voice, in.Voice)
	text("voice_name", &cur.VoiceName, in.VoiceName)
	boolIn("haptics", &cur.Haptics, in.Haptics)
	boolIn("ask_calm", &cur.AskCalm, in.AskCalm)
	boolIn("keep_awake", &cur.KeepAwake, in.KeepAwake)
	boolIn("reminder_skip_done", &cur.ReminderSkipDone, in.ReminderSkipDone)
	boolIn("notify_streak", &cur.NotifyStreak, in.NotifyStreak)
	boolIn("notify_milestones", &cur.NotifyMilestones, in.NotifyMilestones)
	boolIn("notify_weekly", &cur.NotifyWeekly, in.NotifyWeekly)
	text("notify_url", &cur.NotifyURL, in.NotifyURL)
	if keys["notify_format"] && in.NotifyFormat != "" {
		cur.NotifyFormat = in.NotifyFormat
	}
	url("habits_url", &cur.HabitsURL, in.HabitsURL)
	secret("habits_token", &cur.HabitsToken, in.HabitsToken)
	url("habits_public_url", &cur.HabitsPublicURL, in.HabitsPublicURL)
	url("health_url", &cur.HealthURL, in.HealthURL)
	secret("health_token", &cur.HealthToken, in.HealthToken)
	url("health_public_url", &cur.HealthPublicURL, in.HealthPublicURL)
	boolIn("health_sync", &cur.HealthSync, in.HealthSync)
	boolIn("health_insights", &cur.HealthInsights, in.HealthInsights)
	url("music_url", &cur.MusicURL, in.MusicURL)
	text("music_user", &cur.MusicUser, in.MusicUser)
	secret("music_password", &cur.MusicPassword, in.MusicPassword)
	if keys["ai_provider"] {
		if !validAIProvider(in.AIProvider) {
			bad("AI is automatic, Pocket AI or OpenAI")
			return
		}
		cur.AIProvider = in.AIProvider
	}
	if keys["ai_pocket_model"] {
		if !validPocketModel(in.AIPocketModel) {
			bad("unknown model")
			return
		}
		cur.AIPocketModel = in.AIPocketModel
	}
	secret("openai_api_key", &cur.OpenAIKey, in.OpenAIKey)
	text("openai_model", &cur.OpenAIModel, in.OpenAIModel)
	url("openai_base_url", &cur.OpenAIBaseURL, in.OpenAIBaseURL)
	mode := func(k string, dst *string, v string) {
		if keys[k] && (v == modeCore || v == modeDirect || v == modeOff) {
			*dst = v
		}
	}
	mode("habits_mode", &cur.HabitsMode, in.HabitsMode)
	mode("health_mode", &cur.HealthMode, in.HealthMode)
	mode("music_mode", &cur.MusicMode, in.MusicMode)
	if err := s.store.SaveUserSettings(r.Context(), me.ID, cur); err != nil {
		s.fail(w, err, "save settings")
		return
	}
	if isAdmin && keys["title"] {
		if err := s.store.SaveBoxSettings(r.Context(), cur); err != nil {
			s.fail(w, err, "save settings")
			return
		}
	}
	for _, k := range modeKeys {
		if keys[k] {
			s.reportModesSoon()
			break
		}
	}
	if keys["health_sync"] && cur.HealthSync {
		go func(uid int64) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			s.syncHealth(ctx, uid, store.AddDays(store.Today(), -7))
		}(me.ID)
	}
	if keys["daily_goal_min"] || keys["week_start"] {
		go func(uid int64) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			s.checkAchievements(ctx, uid, nil)
		}(me.ID)
	}
	writeJSON(w, http.StatusOK, s.publicSettings(r.Context(), me, cur))
}

// handleIntegrationTest checks a connection with the caller's own saved
// settings (or the URL/key typed into the form).
func (s *Server) handleIntegrationTest(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Which  string `json:"which"`
		URL    string `json:"url"`
		Key    string `json:"key"`
		User   string `json:"user"`
		Format string `json:"format"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	uid := actorID(r.Context())
	st, _ := s.store.GetSettings(r.Context(), uid)
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	keyOr := func(saved string) string {
		if strings.Contains(in.Key, "••") || in.Key == "" {
			return saved
		}
		return in.Key
	}
	urlOr := func(saved string) string {
		if strings.TrimSpace(in.URL) == "" {
			return saved
		}
		return strings.TrimRight(strings.TrimSpace(in.URL), "/")
	}
	var msg string
	var err error
	if a, ok := appBySlug(in.Which); ok {
		mode, savedURL, savedTok, _ := connSettings(st, a)
		typed := in.Key != "" && !strings.Contains(in.Key, "••")
		var conn *siblingConn
		if tok := keyOr(savedTok); tok != "" && (typed || s.connMode(mode, savedTok) == modeDirect) {
			base := urlOr(savedURL)
			if base == "" {
				base = s.resolveSibling(ctx, a.slug)
			}
			conn = &siblingConn{client: pocket.New(base, tok, 15*time.Second), via: modeDirect}
		} else {
			conn, err = s.clients(ctx, uid).conn(ctx, a, a.scopes...)
		}
		if err == nil {
			how := "direct connection"
			if conn.via == modeCore {
				how = "through Pocket Core"
			}
			var v string
			if v, err = conn.client.Health(ctx); err == nil {
				switch a.slug {
				case "habits":
					var hs []pocket.Habit
					hs, err = conn.client.Habits(ctx)
					msg = fmt.Sprintf("Pocket Habits %s, %s, %d habits", v, how, len(hs))
				case "health":
					var what string
					what, err = healthTest(ctx, conn.client)
					msg = fmt.Sprintf("Pocket Health %s, %s, %s", v, how, what)
				}
			}
		}
	} else if in.Which == "music" {
		var n *navidrome
		if in.User != "" || (in.Key != "" && !strings.Contains(in.Key, "••")) || in.URL != "" {
			n = &navidrome{base: urlOr(st.MusicURL), user: firstNonEmpty(in.User, st.MusicUser), pass: keyOr(st.MusicPassword), via: modeDirect, http: &http.Client{Timeout: 15 * time.Second}}
		} else {
			n, err = s.navidromeFor(ctx, uid)
		}
		if err == nil {
			var out struct {
				Playlists struct {
					Playlist []navPlaylist `json:"playlist"`
				} `json:"playlists"`
			}
			if err = n.call(ctx, "getPlaylists", nil, &out); err == nil {
				how := "direct connection"
				if n.via == modeCore {
					how = "through Pocket Core"
				}
				msg = fmt.Sprintf("Navidrome, %s, %d playlists", how, len(out.Playlists.Playlist))
			}
		}
	} else if in.Which == "openai" {
		c := ai.New(urlOr(st.OpenAIBaseURL), keyOr(st.OpenAIKey), st.OpenAIModel, 20*time.Second)
		var ids []string
		if ids, err = c.Models(ctx); err == nil {
			msg = fmt.Sprintf("The key works (%d models)", len(ids))
		}
	} else if in.Which == "notify" {
		f := in.Format
		if f == "" {
			f = st.NotifyFormat
		}
		n := notify.New(urlOr(st.NotifyURL), f)
		err = n.Send(ctx, notify.Message{Title: "Pocket Still", Body: "Notifications work. Reminders, milestones and the weekly report will arrive here.", Priority: "info", Tags: []string{"lotus_position"}})
		msg = "Test message sent via " + n.Format
	} else {
		writeErr(w, http.StatusBadRequest, "unknown integration")
		return
	}
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": msg})
}

func (s *Server) handleNotifyTest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	st, _ := s.store.GetSettings(ctx, actorID(ctx))
	n := notify.New(st.NotifyURL, st.NotifyFormat)
	if err := n.Send(ctx, notify.Message{Title: "Pocket Still", Body: "Notifications work.", Priority: "info", Tags: []string{"lotus_position"}}); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---- system / backup / export ----

func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	var size int64
	if fi, err := os.Stat(s.cfg.DBPath); err == nil {
		size = fi.Size()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"version": Version, "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH,
		"core":      s.coreStatus(),
		"auth_mode": s.cfg.AuthMode, "data_dir": s.cfg.DataDir, "db_path": s.cfg.DBPath, "db_bytes": size,
		"uptime": time.Since(s.started).Round(time.Second).String(), "timezone": time.Now().Location().String(),
		"counts": s.store.CountsFor(r.Context(), actorID(r.Context())), "saved": s.saved.Load(),
	})
}

// handleBackup downloads the whole database, an admin-only box operation.
func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	dir := filepath.Join(s.cfg.DataDir, "backups")
	name := fmt.Sprintf("pocket-still-%s.db", time.Now().Format("20060102-150405"))
	dst := filepath.Join(dir, name)
	if err := s.store.Backup(r.Context(), dst); err != nil {
		s.fail(w, err, "backup")
		return
	}
	w.Header().Set("Content-Type", "application/vnd.sqlite3")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	http.ServeFile(w, r, dst)
}

// exportDoc is the JSON export of one person's data.
type exportDoc struct {
	App       string             `json:"app"`
	Version   string             `json:"version"`
	Exported  string             `json:"exported_at"`
	Practices []store.Practice   `json:"practices"`
	Presets   []store.Preset     `json:"presets"`
	Lessons   []store.LessonDone `json:"lessons_done"`
	Favorites []string           `json:"favorites"`
	Rules     []store.HabitRule  `json:"habit_rules"`
}

func (s *Server) handleExportJSON(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	doc := exportDoc{App: "pocket-still", Version: Version, Exported: store.Now(), Favorites: []string{}}
	doc.Practices, _, _ = s.store.ListPractices(ctx, uid, store.PracticeFilter{Asc: true})
	doc.Presets, _ = s.store.ListPresets(ctx, uid, "")
	doc.Lessons, _ = s.store.LessonsDone(ctx, uid, "")
	doc.Rules, _ = s.store.ListRules(ctx, uid)
	if favs, err := s.store.Favorites(ctx, uid); err == nil {
		for k := range favs {
			doc.Favorites = append(doc.Favorites, k)
		}
		sort.Strings(doc.Favorites)
	}
	w.Header().Set("Content-Disposition", `attachment; filename="pocket-still-export.json"`)
	writeJSON(w, http.StatusOK, doc)
}

// handleExportCSV writes every practice as a CSV row.
func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ps, _, err := s.store.ListPractices(ctx, actorID(ctx), store.PracticeFilter{Asc: true})
	if err != nil {
		s.fail(w, err, "export")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="pocket-still-practices.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"date", "started_at", "kind", "technique", "title", "minutes", "planned_minutes", "completed", "cycles", "calm_before", "calm_after", "hr_before", "hr_after", "tags", "program", "lesson", "source", "note"})
	opt := func(p *int) string {
		if p == nil {
			return ""
		}
		return strconv.Itoa(*p)
	}
	optf := func(p *float64) string {
		if p == nil {
			return ""
		}
		return strconv.FormatFloat(*p, 'f', -1, 64)
	}
	for _, p := range ps {
		_ = cw.Write([]string{p.Date, p.StartedAt, p.Kind, p.Technique, p.Title, strconv.FormatFloat(round2(p.Minutes()), 'f', -1, 64),
			strconv.FormatFloat(round2(float64(p.PlannedS)/60), 'f', -1, 64), strconv.FormatBool(p.Completed), strconv.Itoa(p.Cycles),
			opt(p.CalmBefore), opt(p.CalmAfter), optf(p.HRBefore), optf(p.HRAfter), p.Tags, p.Program, p.Lesson, p.Source, p.Note})
	}
	cw.Flush()
}

// handleImportJSON merges an export into the caller's own data. Practices
// keep their identity (source "import" + the original id), so importing the
// same file twice adds nothing; imported practices never go to Pocket Health
// or tick habits again.
func (s *Server) handleImportJSON(w http.ResponseWriter, r *http.Request) {
	var doc exportDoc
	if err := decode(r, &doc); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if doc.App != "" && doc.App != "pocket-still" {
		writeErr(w, http.StatusBadRequest, "this is not a Pocket Still export")
		return
	}
	ctx := r.Context()
	uid := actorID(ctx)
	counts := map[string]int{}
	ids := map[int64]int64{}
	for _, p := range doc.Practices {
		old := p.ID
		p.ID = 0
		if p.ExternalID == "" {
			p.ExternalID = fmt.Sprintf("%s:%d", firstNonEmpty(p.Source, "app"), old)
		}
		p.Source = "import"
		if !content.ValidKind(p.Kind) || p.StartedAt == "" || p.DurationS < 0 {
			counts["skipped"]++
			continue
		}
		if err := s.store.CreatePractice(ctx, uid, &p); err != nil {
			counts["skipped"]++
			continue
		}
		ids[old] = p.ID
		now := store.Now()
		_ = s.store.SetHealthSync(ctx, uid, p.ID, now, "")
		_ = s.store.SetHabitMark(ctx, uid, p.ID, now, "")
		counts["practices"]++
	}
	for _, pr := range doc.Presets {
		in := presetInput{Kind: pr.Kind, Name: pr.Name, Config: pr.Config, Favorite: pr.Favorite, Sort: pr.Sort}
		cfg, msg := in.check()
		if msg != "" {
			continue
		}
		p := store.Preset{Kind: in.Kind, Name: in.Name, Config: cfg, Favorite: in.Favorite, Sort: in.Sort}
		if err := s.store.CreatePreset(ctx, uid, &p); err == nil {
			counts["presets"]++
		}
	}
	for _, l := range doc.Lessons {
		prog, ok := content.ProgramByKey(l.Program)
		if !ok {
			continue
		}
		if _, ok := prog.FindLesson(l.Lesson); !ok {
			continue
		}
		var pid int64
		if l.PracticeID != nil {
			pid = ids[*l.PracticeID]
		}
		if fresh, err := s.store.CompleteLesson(ctx, uid, l.Program, l.Lesson, pid); err == nil && fresh {
			counts["lessons"]++
		}
	}
	for _, k := range doc.Favorites {
		if content.TechniqueName(k) != "" && s.store.SetFavorite(ctx, uid, k, true) == nil {
			counts["favorites"]++
		}
	}
	for _, ru := range doc.Rules {
		ru.ID = 0
		if err := s.store.CreateRule(ctx, uid, &ru); err == nil {
			counts["rules"]++
		}
	}
	s.checkAchievements(ctx, uid, nil)
	writeJSON(w, http.StatusOK, counts)
}

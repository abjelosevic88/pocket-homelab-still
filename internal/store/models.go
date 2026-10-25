package store

import (
	"encoding/json"
	"strings"
)

// User is an account on this box.
type User struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"` // admin | member
	CreatedAt    string `json:"created_at"`
}

// APIToken is a personal access token (hash stored, plaintext shown once).
type APIToken struct {
	ID         int64   `json:"id"`
	UserID     int64   `json:"user_id"`
	Name       string  `json:"name"`
	Prefix     string  `json:"prefix"`
	LastUsedAt *string `json:"last_used_at"`
	CreatedAt  string  `json:"created_at"`
}

// Practice is one breathing exercise, sit, body scan or mindful walk.
type Practice struct {
	ID             int64    `json:"id"`
	UserID         int64    `json:"-"`
	Kind           string   `json:"kind"`
	Technique      string   `json:"technique"`
	Title          string   `json:"title"`
	StartedAt      string   `json:"started_at"`
	EndedAt        string   `json:"ended_at"`
	Date           string   `json:"date"`
	DurationS      int      `json:"duration_s"`
	PlannedS       int      `json:"planned_s"`
	Completed      bool     `json:"completed"`
	Cycles         int      `json:"cycles"`
	CalmBefore     *int     `json:"calm_before"`
	CalmAfter      *int     `json:"calm_after"`
	HRBefore       *float64 `json:"hr_before"`
	HRAfter        *float64 `json:"hr_after"`
	Note           string   `json:"note"`
	Tags           string   `json:"tags"`
	Program        string   `json:"program"`
	Lesson         string   `json:"lesson"`
	Source         string   `json:"source"`
	ExternalID     string   `json:"external_id"`
	HealthSyncedAt string   `json:"health_synced_at"`
	HealthError    string   `json:"health_error"`
	HabitMarkedAt  string   `json:"habit_marked_at"`
	HabitError     string   `json:"habit_error"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
}

// Minutes is the practice's length in minutes.
func (p Practice) Minutes() float64 { return float64(p.DurationS) / 60 }

// Preset is a person's own breathing pattern (kind breath) or timer (kind timer).
type Preset struct {
	ID        int64           `json:"id"`
	UserID    int64           `json:"-"`
	Kind      string          `json:"kind"`
	Name      string          `json:"name"`
	Config    json.RawMessage `json:"config"`
	Favorite  bool            `json:"favorite"`
	Sort      int             `json:"sort"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at"`
}

// HabitRule ticks a Pocket Habits habit when the day's practice reaches a length.
type HabitRule struct {
	ID         int64   `json:"id"`
	UserID     int64   `json:"-"`
	Kind       string  `json:"kind"` // practice kind or "any"
	MinMinutes float64 `json:"min_minutes"`
	HabitID    int64   `json:"habit_id"`
	HabitName  string  `json:"habit_name"`
	Enabled    bool    `json:"enabled"`
	CreatedAt  string  `json:"created_at"`
}

// Settings are the box-wide values plus one person's preferences and their
// own connections. Connection modes: "core" (an app link through Pocket
// Core), "direct" (URL + token) or "off"; "" = not chosen yet.
type Settings struct {
	Title         string `json:"title"`
	SetupComplete bool   `json:"setup_complete"`

	DateFormat string `json:"date_format"`
	TimeFormat string `json:"time_format"`
	WeekStart  int    `json:"week_start"` // 1 = Monday, 7 = Sunday

	// Goals: minutes a day, and on how many days a week.
	DailyGoalMin   int `json:"daily_goal_min"`
	WeeklyGoalDays int `json:"weekly_goal_days"`

	// The player.
	Bell          string  `json:"bell"`          // the default bell
	Ambient       string  `json:"ambient"`       // the default background sound
	BreathSound   string  `json:"breath_sound"`  // soft | bell | none, at each breathing phase
	Volume        float64 `json:"volume"`        // 0–1
	Voice         bool    `json:"voice"`         // spoken cues
	VoiceName     string  `json:"voice_name"`    // a browser voice, "" = default
	Haptics       bool    `json:"haptics"`       // vibrate on phase changes (phones that can)
	AskCalm       bool    `json:"ask_calm"`      // ask "how do you feel" before and after
	KeepAwake     bool    `json:"keep_awake"`    // keep the screen on while practising
	PlayerTheme   string  `json:"player_theme"`  // dusk | ocean | forest | night | minimal
	BreathVisual  string  `json:"breath_visual"` // orb | box | wave
	DefaultSitMin float64 `json:"default_sit_min"`

	// Reminders.
	ReminderTimes    []string `json:"reminder_times"` // "08:00"
	ReminderDays     string   `json:"reminder_days"`  // ISO weekdays "1234567"
	ReminderSkipDone bool     `json:"reminder_skip_done"`
	NotifyStreak     bool     `json:"notify_streak"` // evening nudge when today would break the streak
	StreakHour       int      `json:"streak_hour"`
	NotifyMilestones bool     `json:"notify_milestones"`
	NotifyWeekly     bool     `json:"notify_weekly"`
	WeeklyDay        int      `json:"weekly_day"` // ISO weekday the weekly report goes out
	WeeklyHour       int      `json:"weekly_hour"`
	NotifyURL        string   `json:"notify_url"`
	NotifyFormat     string   `json:"notify_format"` // apprise | ntfy | json

	HabitsMode      string `json:"habits_mode"`
	HabitsURL       string `json:"habits_url"`
	HabitsToken     string `json:"habits_token"`
	HabitsPublicURL string `json:"habits_public_url"`

	HealthMode      string `json:"health_mode"`
	HealthURL       string `json:"health_url"`
	HealthToken     string `json:"health_token"`
	HealthPublicURL string `json:"health_public_url"`
	HealthSync      bool   `json:"health_sync"`     // write mindful minutes to Pocket Health
	HealthInsights  bool   `json:"health_insights"` // read heart rate variability, resting heart rate and sleep back

	// Background music from Navidrome (through Pocket Core or directly).
	MusicMode     string `json:"music_mode"`
	MusicURL      string `json:"music_url"`
	MusicUser     string `json:"music_user"`
	MusicPassword string `json:"music_password"`

	// AI for the reflection and questions: Pocket AI through Core, or an
	// OpenAI-compatible API with the person's own key.
	AIProvider    string `json:"ai_provider"` // auto | pocket | openai
	AIPocketModel string `json:"ai_pocket_model"`
	OpenAIKey     string `json:"openai_api_key"`
	OpenAIModel   string `json:"openai_model"`
	OpenAIBaseURL string `json:"openai_base_url"`
}

// SplitTags turns "a, b ,c" into clean lowercase tags.
func SplitTags(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range strings.Split(s, ",") {
		t = strings.ToLower(strings.TrimSpace(t))
		if t != "" && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// JoinTags is the stored form of tags.
func JoinTags(s string) string { return strings.Join(SplitTags(s), ", ") }

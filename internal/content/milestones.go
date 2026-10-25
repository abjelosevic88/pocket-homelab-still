package content

// Level is a name for a person's lifetime minutes, from a ripple to the ocean.
type Level struct {
	N       int    `json:"n"`
	Name    string `json:"name"`
	Minutes int    `json:"minutes"` // lifetime minutes to reach it
	Blurb   string `json:"blurb"`
}

// Levels in order.
var Levels = []Level{
	{1, "Ripple", 0, "Every practice starts with one breath."},
	{2, "Stream", 60, "An hour of practice. The habit is forming."},
	{3, "River", 300, "Five hours: practice is part of your days."},
	{4, "Lake", 1000, "A thousand minutes of stillness."},
	{5, "Sea", 3000, "Fifty hours. Deep water."},
	{6, "Ocean", 10000, "Ten thousand minutes. Vast and calm."},
}

// LevelFor returns the level for lifetime minutes and the next one (nil at the top).
func LevelFor(minutes float64) (Level, *Level) {
	at := 0
	for i, l := range Levels {
		if minutes >= float64(l.Minutes) {
			at = i
		}
	}
	if at+1 < len(Levels) {
		n := Levels[at+1]
		return Levels[at], &n
	}
	return Levels[at], nil
}

// Achievement is a milestone.
type Achievement struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Group       string `json:"group"` // start | streak | time | sessions | explore | path | moments
}

// Achievements in the order they're shown.
var Achievements = []Achievement{
	{"first_practice", "First breath", "Finish your first practice.", "sparkles", "start"},
	{"first_breath", "Breather", "Finish a breathing exercise.", "wind", "start"},
	{"first_sit", "First sit", "Finish a meditation.", "lotus", "start"},
	{"first_scan", "Head to toe", "Finish a body scan.", "scan", "start"},
	{"first_kindness", "Kind heart", "Finish a loving-kindness practice.", "heart", "start"},

	{"streak_3", "Three in a row", "Practise three days in a row.", "flame", "streak"},
	{"streak_7", "A full week", "Practise seven days in a row.", "flame", "streak"},
	{"streak_14", "Two weeks", "Practise fourteen days in a row.", "flame", "streak"},
	{"streak_30", "A month of stillness", "Practise thirty days in a row.", "flame", "streak"},
	{"streak_100", "Hundred days", "Practise a hundred days in a row.", "crown", "streak"},
	{"perfect_week", "Perfect week", "Reach your daily goal every day of a calendar week.", "calendar-check", "streak"},

	{"minutes_60", "First hour", "Practise for 60 minutes in total.", "clock", "time"},
	{"minutes_300", "Five hours", "Practise for 300 minutes in total.", "clock", "time"},
	{"minutes_1000", "A thousand minutes", "Practise for 1,000 minutes in total.", "clock", "time"},
	{"minutes_3000", "Fifty hours", "Practise for 3,000 minutes in total.", "clock", "time"},
	{"minutes_10000", "Ten thousand", "Practise for 10,000 minutes in total.", "clock", "time"},
	{"long_sit", "Twenty minutes", "Sit for 20 minutes or more at once.", "hourglass", "time"},
	{"deep_sit", "Deep sit", "Sit for 45 minutes or more at once.", "hourglass", "time"},

	{"sessions_10", "Ten practices", "Finish 10 practices.", "check", "sessions"},
	{"sessions_50", "Fifty practices", "Finish 50 practices.", "check", "sessions"},
	{"sessions_100", "A hundred practices", "Finish 100 practices.", "check", "sessions"},
	{"sessions_500", "Five hundred", "Finish 500 practices.", "check", "sessions"},

	{"all_breaths", "Every breath", "Try every built-in breathing pattern.", "compass", "explore"},
	{"all_styles", "Every style", "Try every meditation style.", "compass", "explore"},
	{"own_pattern", "Your own rhythm", "Make your own breathing pattern or timer.", "sliders", "explore"},

	{"early_bird", "Early bird", "Practise before 7 in the morning.", "sunrise", "moments"},
	{"night_owl", "Night owl", "Practise after 10 in the evening.", "moon", "moments"},
	{"calm_shift", "From storm to calm", "Feel two steps calmer after a practice than before.", "smile", "moments"},
	{"reflective", "Reflective", "Write a note on 10 practices.", "pen", "moments"},

	{"path_arrive", "Arrived", "Finish the Arrive stage of the Path.", "mountain", "path"},
	{"path_settle", "Settled", "Finish the Settle stage of the Path.", "mountain", "path"},
	{"path_steady", "Steady", "Finish the Steady stage of the Path.", "mountain", "path"},
	{"path_notice", "Noticing", "Finish the Notice stage of the Path.", "mountain", "path"},
	{"path_feel", "Embodied", "Finish the Feel stage of the Path.", "mountain", "path"},
	{"path_open", "Open-hearted", "Finish the Open stage of the Path.", "mountain", "path"},
	{"path_still", "Still", "Finish the whole Path.", "crown", "path"},
	{"course_breathe", "Breathes better", "Finish the Breathe better course.", "wind", "path"},
	{"course_sleep", "Sleeps well", "Finish the Sleep well course.", "moon", "path"},
	{"course_stress", "Resilient", "Finish the Stress reset course.", "shield", "path"},
}

// AchievementByKey finds a milestone.
func AchievementByKey(key string) (Achievement, bool) {
	for _, a := range Achievements {
		if a.Key == key {
			return a, true
		}
	}
	return Achievement{}, false
}

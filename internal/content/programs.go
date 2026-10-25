package content

// Program is the Path or a short course: stages of lessons, each a short
// teaching plus one practice. Lessons unlock one after another; finishing a
// lesson's practice finishes the lesson.
type Program struct {
	Key         string  `json:"key"`
	Name        string  `json:"name"`
	Kind        string  `json:"kind"` // path | course
	Tagline     string  `json:"tagline"`
	Description string  `json:"description"`
	Color       string  `json:"color"`
	Icon        string  `json:"icon"`
	Stages      []Stage `json:"stages"`
}

// Stage is a group of lessons with one theme.
type Stage struct {
	Key     string   `json:"key"`
	Name    string   `json:"name"`
	Tagline string   `json:"tagline"`
	Lessons []Lesson `json:"lessons"`
}

// Lesson is one day of a program.
type Lesson struct {
	Key      string   `json:"key"`
	Title    string   `json:"title"`
	Teaching []string `json:"teaching"`
	Tip      string   `json:"tip,omitempty"`
	Practice Practice `json:"practice"`
}

// Practice is what a lesson asks you to do.
type Practice struct {
	Kind      string  `json:"kind"`      // breath | meditation | body_scan | …
	Technique string  `json:"technique"` // a pattern or style key
	Minutes   float64 `json:"minutes"`
}

func br(key string, min float64) Practice {
	return Practice{Kind: "breath", Technique: key, Minutes: min}
}
func sit(key string, min float64) Practice {
	k, _ := StyleByKey(key)
	return Practice{Kind: k.Kind, Technique: key, Minutes: min}
}

// Programs: the Path first, then the courses.
var Programs = []Program{
	{
		Key: "path", Name: "The Path", Kind: "path", Color: "#0f766e", Icon: "mountain",
		Tagline:     "Seven stages from your first breath to sitting still",
		Description: "A step-by-step way into meditation. Each stage teaches one skill with four short lessons; practices grow from three minutes to twenty. Take one lesson a day, or repeat one until it feels easy. The next stage opens when the last one is done.",
		Stages: []Stage{
			{Key: "arrive", Name: "Arrive", Tagline: "Notice that you are breathing", Lessons: []Lesson{
				{Key: "arrive-1", Title: "Just three minutes", Teaching: []string{
					"Meditation starts smaller than most people think. You don't need to empty your mind or sit cross-legged. You need a few minutes and something to pay attention to: the breath, which is always with you.",
					"Today, simply sit and notice that you are breathing. You don't have to change it."},
					Tip: "Sit on a chair with both feet on the floor. Upright, but not stiff.", Practice: sit("breath_focus", 3)},
				{Key: "arrive-2", Title: "Where do you feel it?", Teaching: []string{
					"The breath shows up in different places: the cool air at the nostrils, the chest rising, the belly swelling. Today, find the place where you feel it most clearly.",
					"That spot becomes your anchor. When you get lost, that's where you come back to."},
					Practice: sit("breath_focus", 3)},
				{Key: "arrive-3", Title: "The wandering mind", Teaching: []string{
					"Within seconds the mind will wander: to plans, memories, a song. Everyone's does. This is not a sign you're bad at it; it's what minds do.",
					"The moment you notice you've drifted is the moment of mindfulness. Each return to the breath is one repetition, like a rep at the gym."},
					Tip: "When you notice you were thinking, smile a little and come back.", Practice: sit("breath_focus", 4)},
				{Key: "arrive-4", Title: "A daily anchor", Teaching: []string{
					"Consistency matters more than length. Three minutes every day changes more than thirty minutes once a week.",
					"Pick a moment you already have: after coffee, before opening the laptop, after brushing your teeth. Tie the practice to it."},
					Tip: "Set a reminder under Settings → Practice at the time you chose.", Practice: sit("breath_focus", 5)},
			}},
			{Key: "settle", Name: "Settle", Tagline: "Use the breath to calm the body", Lessons: []Lesson{
				{Key: "settle-1", Title: "Even breathing", Teaching: []string{
					"Now you'll start shaping the breath. Equal breathing, the same count in and out, is the gentlest way to steady a restless body.",
					"Follow the circle on screen: it grows as you breathe in and shrinks as you breathe out."},
					Practice: br("equal", 3)},
				{Key: "settle-2", Title: "The longer exhale", Teaching: []string{
					"The out-breath is linked to the body's rest-and-digest system. Making it longer than the in-breath is one of the fastest ways to calm down.",
					"Notice how the shoulders drop and the jaw loosens as you breathe out."},
					Practice: br("downshift", 4)},
				{Key: "settle-3", Title: "The sigh", Teaching: []string{
					"Your body already knows how to reset: it sighs. A double inhale followed by a long exhale empties the lungs and releases tension.",
					"Use one or two of these whenever you feel stress rising, not only in practice."},
					Practice: br("sigh", 4)},
				{Key: "settle-4", Title: "Box breathing", Teaching: []string{
					"Box breathing adds holds. The precise counting gives an anxious mind a job, which is why it's used in high-pressure work.",
					"Keep the holds relaxed: no clenching, no strain. Shorten the counts to three if four feels like too much."},
					Practice: br("box", 5)},
			}},
			{Key: "steady", Name: "Steady", Tagline: "Hold attention for longer", Lessons: []Lesson{
				{Key: "steady-1", Title: "Counting to ten", Teaching: []string{
					"Counting breaths is a simple way to see how attention behaves. Count each out-breath from one to ten, then begin again.",
					"Most people lose count before ten. That's the point: you'll see exactly when you drifted."},
					Practice: sit("counting", 6)},
				{Key: "steady-2", Title: "Gentle effort", Teaching: []string{
					"Attention is like holding a bird: grip too hard and you hurt it, too loose and it flies away. Aim for light, interested attention.",
					"If you notice frustration, let the next breath be a little softer."},
					Practice: sit("counting", 7)},
				{Key: "steady-3", Title: "Restlessness and sleepiness", Teaching: []string{
					"Two visitors come to every meditator: restlessness and dullness. When restless, feel the out-breath and the weight of the body. When sleepy, sit taller and open the eyes slightly.",
					"Both are passing weather, not a problem with you."},
					Practice: sit("breath_focus", 8)},
				{Key: "steady-4", Title: "Ten minutes", Teaching: []string{
					"Today you sit for ten minutes, longer than the stages before. Begin with three slow breaths, then let the breath be natural and rest with it.",
					"If the time feels long, that's useful information too: notice the urge to check the clock."},
					Practice: sit("breath_focus", 10)},
			}},
			{Key: "notice", Name: "Notice", Tagline: "See thoughts and feelings clearly", Lessons: []Lesson{
				{Key: "notice-1", Title: "Naming what arises", Teaching: []string{
					"Noting means giving a soft one-word label to whatever takes your attention: thinking, hearing, feeling, planning.",
					"A label creates a little distance. You're no longer inside the thought; you're noticing it."},
					Practice: sit("noting", 8)},
				{Key: "notice-2", Title: "Sounds", Teaching: []string{
					"Sounds make a good meditation object: they arrive by themselves and leave by themselves. Let them come to you without naming the source.",
					"Notice the silence between sounds as well."},
					Practice: sit("noting", 8)},
				{Key: "notice-3", Title: "Thoughts as events", Teaching: []string{
					"Thoughts are events in the mind, like sounds are events in the room. You don't have to believe them, argue with them or follow them.",
					"Try picturing them as cars passing on a street while you sit on the pavement."},
					Practice: sit("noting", 10)},
				{Key: "notice-4", Title: "Emotions in the body", Teaching: []string{
					"Every feeling has a body: tightness in the chest, heat in the face, a knot in the stomach. Today, when an emotion appears, find where it lives in the body.",
					"Breathe with that place without trying to fix it."},
					Practice: sit("noting", 10)},
			}},
			{Key: "feel", Name: "Feel", Tagline: "Come back into the body", Lessons: []Lesson{
				{Key: "feel-1", Title: "Your first body scan", Teaching: []string{
					"A body scan moves attention slowly from head to toe. You're not trying to relax; you're finding out what's there.",
					"Lying down is fine. If you fall asleep, maybe you needed the sleep."},
					Practice: sit("body_scan", 10)},
				{Key: "feel-2", Title: "Tension and release", Teaching: []string{
					"As you scan, you'll find places that are tight. Breathe into them on the in-breath; on the out-breath, let them soften a little, or not.",
					"Acceptance doesn't mean liking it; it means letting it be there."},
					Practice: sit("body_scan", 12)},
				{Key: "feel-3", Title: "The breathing body", Teaching: []string{
					"Combine what you've learnt: start with three long exhales, then feel the whole body breathing at once.",
					"The breath moves more of the body than you'd expect: ribs, back, belly, even the shoulders."},
					Practice: br("coherent", 8)},
				{Key: "feel-4", Title: "A long scan", Teaching: []string{
					"Today's scan is slower. With more time per body part, you'll notice finer sensations: pulse, tingling, the touch of clothing.",
					"Many people use this one in bed. It's one of the best practices for sleep."},
					Practice: sit("body_scan", 15)},
			}},
			{Key: "open", Name: "Open", Tagline: "Kindness, for yourself and others", Lessons: []Lesson{
				{Key: "open-1", Title: "Kindness to yourself", Teaching: []string{
					"Loving-kindness practice starts with yourself, which is often the hardest. Repeat: may I be safe, may I be well, may I be at ease.",
					"If it feels mechanical, that's fine. The intention is the practice; the warmth comes later."},
					Practice: sit("loving_kindness", 8)},
				{Key: "open-2", Title: "The circle grows", Teaching: []string{
					"Today the wishes widen: someone you love, someone you barely know, and finally someone you find difficult.",
					"Go only as far with the difficult person as feels right. You can always return to yourself."},
					Practice: sit("loving_kindness", 10)},
				{Key: "open-3", Title: "Gratitude", Teaching: []string{
					"Start today's sit by naming three things you're grateful for, however small: warm coffee, a message from a friend, a quiet room.",
					"Then rest with the breath and notice how the body feels after gratitude."},
					Practice: sit("breath_focus", 10)},
				{Key: "open-4", Title: "Kindness in daily life", Teaching: []string{
					"Practise once more, then carry it with you: silently wish someone well while you wait in a queue or sit in traffic.",
					"Small, unseen moments of goodwill change how a day feels."},
					Practice: sit("loving_kindness", 12)},
			}},
			{Key: "still", Name: "Still", Tagline: "Sit on your own", Lessons: []Lesson{
				{Key: "still-1", Title: "Open awareness", Teaching: []string{
					"Until now you've had an anchor: the breath, the body, phrases. Now let attention open to everything at once and rest as the awareness in which it all happens.",
					"If it gets too vague, go back to the breath for a while. That's not a step back."},
					Practice: sit("open", 12)},
				{Key: "still-2", Title: "Fewer words", Teaching: []string{
					"Today the guidance is light: a few cues and a lot of silence. You know what to do by now.",
					"Notice what the mind does without instructions to follow."},
					Practice: sit("open", 15)},
				{Key: "still-3", Title: "Your own practice", Teaching: []string{
					"Choose for yourself today: breath, body, noting, kindness or open awareness. Sit with the silent timer.",
					"Knowing what you need on a given day is a skill in itself."},
					Practice: sit("silent", 15)},
				{Key: "still-4", Title: "Twenty minutes, still", Teaching: []string{
					"The last lesson of the Path: twenty minutes of silent sitting, just the bells.",
					"This isn't an ending. The Path starts again every time you sit; repeat any lesson whenever you like, and keep going with the timer and the courses."},
					Practice: sit("silent", 20)},
			}},
		},
	},
	{
		Key: "breathe", Name: "Breathe better", Kind: "course", Color: "#0ea5e9", Icon: "wind",
		Tagline:     "Seven days, seven ways to breathe",
		Description: "A week-long tour of the breathing patterns: what each one does and when to use it. By the end you'll know which breath to reach for when you need calm, focus, sleep or energy.",
		Stages: []Stage{{Key: "week", Name: "Seven days", Lessons: []Lesson{
			{Key: "breathe-1", Title: "Nose and belly", Teaching: []string{"Most of us breathe high in the chest and through the mouth. Slow nose breathing that moves the belly is the foundation of every pattern.", "Put a hand on your belly and feel it rise on the in-breath."}, Practice: br("equal", 3)},
			{Key: "breathe-2", Title: "Calm on demand", Teaching: []string{"A longer exhale slows the heart. This is the pattern for a tense meeting or a difficult phone call."}, Practice: br("downshift", 4)},
			{Key: "breathe-3", Title: "The reset button", Teaching: []string{"The physiological sigh is the fastest single breath for releasing stress. Practise it now so it's there when you need it."}, Practice: br("sigh", 4)},
			{Key: "breathe-4", Title: "Focus", Teaching: []string{"Box breathing steadies attention before work that needs concentration."}, Practice: br("box", 5)},
			{Key: "breathe-5", Title: "Resonance", Teaching: []string{"Breathing at around five and a half breaths a minute raises heart rate variability, a marker of resilience. Ten minutes is the classic dose."}, Practice: br("coherent", 10)},
			{Key: "breathe-6", Title: "Wake up", Teaching: []string{"Breathing can raise energy too: a long in-breath and a short out-breath. Use it in the morning or after lunch, sitting down."}, Practice: br("energize", 2)},
			{Key: "breathe-7", Title: "For sleep", Teaching: []string{"4-7-8 breathing is made for bedtime. Practise it today so it's familiar tonight."}, Practice: br("relax478", 3)},
		}}},
	},
	{
		Key: "sleep", Name: "Sleep well", Kind: "course", Color: "#7c3aed", Icon: "moon",
		Tagline:     "Five nights of winding down",
		Description: "Short practices for the last minutes before sleep: slowing the breath, scanning the body, and letting the day go. Best done in bed with the screen dimmed.",
		Stages: []Stage{{Key: "nights", Name: "Five nights", Lessons: []Lesson{
			{Key: "sleep-1", Title: "Slow down", Teaching: []string{"Sleep comes more easily to a slow body. Begin with long exhales and let each one be a little heavier than the last."}, Practice: br("downshift", 5)},
			{Key: "sleep-2", Title: "4-7-8", Teaching: []string{"Four rounds of the relaxing breath, then breathe normally and let the eyes stay closed."}, Practice: br("relax478", 3)},
			{Key: "sleep-3", Title: "Body scan in bed", Teaching: []string{"Scan the body lying down. If you fall asleep before the end, the practice worked."}, Practice: sit("body_scan", 12)},
			{Key: "sleep-4", Title: "Letting the day go", Teaching: []string{"Note each thought about today or tomorrow as \"planning\" or \"remembering\", and let it go with an out-breath."}, Practice: sit("noting", 8)},
			{Key: "sleep-5", Title: "Kindness before sleep", Teaching: []string{"End the day by wishing yourself and the people in your life well. A softer mind sleeps better."}, Practice: sit("loving_kindness", 8)},
		}}},
	},
	{
		Key: "stress", Name: "Stress reset", Kind: "course", Color: "#e11d48", Icon: "shield",
		Tagline:     "Five tools for hard days",
		Description: "Practical tools for when it's too much: what to do in the moment, how to come down afterwards, and how to build some resilience for next time.",
		Stages: []Stage{{Key: "tools", Name: "Five tools", Lessons: []Lesson{
			{Key: "stress-1", Title: "Two sighs", Teaching: []string{"In the moment, one or two physiological sighs bring the body down faster than anything else. Practise until it's automatic."}, Practice: br("sigh", 3)},
			{Key: "stress-2", Title: "Name it to tame it", Teaching: []string{"Naming an emotion (\"this is anxiety\") calms the brain's alarm. Practise noting feelings as they arise."}, Practice: sit("noting", 6)},
			{Key: "stress-3", Title: "Feel your feet", Teaching: []string{"When thoughts spin, go to the body. Feel the feet on the floor, the weight in the chair. Then scan upwards."}, Practice: sit("body_scan", 8)},
			{Key: "stress-4", Title: "Steady under pressure", Teaching: []string{"Box breathing gives an overloaded mind a simple job and keeps you thinking clearly."}, Practice: br("box", 5)},
			{Key: "stress-5", Title: "Building resilience", Teaching: []string{"Daily resonance breathing raises heart rate variability over weeks. Make it a habit and the hard days hit a little softer."}, Practice: br("coherent", 10)},
		}}},
	},
}

// ProgramByKey finds a program.
func ProgramByKey(key string) (Program, bool) {
	for _, p := range Programs {
		if p.Key == key {
			return p, true
		}
	}
	return Program{}, false
}

// LessonRef is a lesson with where it sits.
type LessonRef struct {
	Program, Stage string
	StageIdx       int
	Index          int // across the whole program
	Lesson         Lesson
}

// Lessons lists a program's lessons in order.
func (p Program) Lessons() []LessonRef {
	var out []LessonRef
	for si, st := range p.Stages {
		for _, l := range st.Lessons {
			out = append(out, LessonRef{Program: p.Key, Stage: st.Key, StageIdx: si, Index: len(out), Lesson: l})
		}
	}
	return out
}

// FindLesson finds a lesson in a program.
func (p Program) FindLesson(key string) (LessonRef, bool) {
	for _, l := range p.Lessons() {
		if l.Lesson.Key == key {
			return l, true
		}
	}
	return LessonRef{}, false
}

// Package content is what Pocket Still ships with: breathing patterns,
// meditation styles with their spoken cues, the Path (stages of lessons that
// unlock one after another), the short courses, levels and milestones.
// Everything here is plain data; people's own patterns and timers live in
// the database as presets.
package content

import (
	"fmt"
	"math"
	"strings"
)

// ---------------- breathing ----------------

// Phase kinds of a breathing cycle.
const (
	In      = "in"       // breathe in
	InTop   = "in2"      // a second, short sip of air on top (the physiological sigh)
	Hold    = "hold"     // hold with full lungs
	Out     = "out"      // breathe out
	HoldOut = "hold_out" // hold with empty lungs
)

// Phase is one step of a breathing cycle.
type Phase struct {
	Type  string  `json:"type"`
	S     float64 `json:"s"`               // seconds
	Label string  `json:"label,omitempty"` // shown instead of the default ("In through the left nostril")
}

// BreathConfig is a breathing exercise: one cycle of phases, repeated for a
// number of cycles or minutes. Ease starts the first cycles shorter and
// stretches them to the full pattern (for long holds).
type BreathConfig struct {
	Phases  []Phase `json:"phases"`
	Minutes float64 `json:"minutes,omitempty"` // used when Cycles is 0
	Cycles  int     `json:"cycles,omitempty"`
	Ease    bool    `json:"ease,omitempty"`
	Sound   string  `json:"sound,omitempty"`   // phase cue: soft | bell | none
	Ambient string  `json:"ambient,omitempty"` // background: none | rain | ocean | brown | pink | stream | fire | music:<playlist>
	Voice   bool    `json:"voice,omitempty"`   // spoken "in", "hold", "out"
}

// CycleSeconds is one cycle's length.
func (c BreathConfig) CycleSeconds() float64 {
	t := 0.0
	for _, p := range c.Phases {
		t += p.S
	}
	return t
}

// Validate checks a person's own pattern.
func (c BreathConfig) Validate() error {
	if len(c.Phases) == 0 || len(c.Phases) > 12 {
		return fmt.Errorf("a pattern has 1 to 12 phases")
	}
	breathes := false
	for _, p := range c.Phases {
		switch p.Type {
		case In, InTop, Out:
			breathes = true
		case Hold, HoldOut:
		default:
			return fmt.Errorf("unknown phase %q", p.Type)
		}
		if p.S < 0.5 || p.S > 60 || math.IsNaN(p.S) {
			return fmt.Errorf("each phase lasts 0.5 to 60 seconds")
		}
		if len(p.Label) > 80 {
			return fmt.Errorf("a phase label is at most 80 characters")
		}
	}
	if !breathes {
		return fmt.Errorf("a pattern needs a breath in or out")
	}
	if c.Cycles < 0 || c.Cycles > 500 || c.Minutes < 0 || c.Minutes > 120 {
		return fmt.Errorf("1 to 500 cycles or up to 120 minutes")
	}
	return nil
}

// Pattern is a built-in breathing exercise.
type Pattern struct {
	Key         string       `json:"key"`
	Name        string       `json:"name"`
	Tagline     string       `json:"tagline"`
	Description string       `json:"description"`
	How         []string     `json:"how"`
	Good        string       `json:"good_for"` // calm | focus | sleep | energy | balance
	Level       string       `json:"level"`    // beginner | intermediate
	Color       string       `json:"color"`
	Caution     string       `json:"caution,omitempty"`
	Config      BreathConfig `json:"config"`
}

func ph(t string, s float64) Phase { return Phase{Type: t, S: s} }

// Patterns are the built-in breathing exercises, roughly from gentlest to most demanding.
var Patterns = []Pattern{
	{Key: "equal", Name: "Equal breathing", Tagline: "In for four, out for four", Level: "beginner", Good: "balance", Color: "#0ea5e9",
		Description: "The simplest pattern there is: breathe in and out for the same count. It gives a busy mind one easy thing to follow and evens the breath out within a minute or two.",
		How:         []string{"Breathe through the nose if you can.", "Count four on the way in and four on the way out.", "Let the breath be smooth rather than big."},
		Config:      BreathConfig{Phases: []Phase{ph(In, 4), ph(Out, 4)}, Minutes: 3}},
	{Key: "downshift", Name: "Long exhale", Tagline: "Breathe out longer than you breathe in", Level: "beginner", Good: "calm", Color: "#14b8a6",
		Description: "A longer out-breath than in-breath leans the nervous system towards rest. Four in, six out is enough to feel the shift; it is the quickest way down after a stressful moment.",
		How:         []string{"In through the nose for four.", "Out slowly for six, through the nose or pursed lips.", "Let the shoulders drop on every out-breath."},
		Config:      BreathConfig{Phases: []Phase{ph(In, 4), ph(Out, 6)}, Minutes: 4}},
	{Key: "sigh", Name: "Physiological sigh", Tagline: "Two sips in, one long sigh out", Level: "beginner", Good: "calm", Color: "#06b6d4",
		Description: "A deep breath in, a short second sip on top, then a long slow exhale. Studies found a few minutes of this a day lowered stress more than other breathing drills, and one or two sighs help in the moment.",
		How:         []string{"Breathe in through the nose until the lungs feel full.", "Take one more short sip of air on top.", "Let it all out slowly through the mouth."},
		Config:      BreathConfig{Phases: []Phase{ph(In, 2.5), ph(InTop, 1), ph(Out, 6.5)}, Minutes: 5}},
	{Key: "coherent", Name: "Coherent breathing", Tagline: "About five and a half breaths a minute", Level: "beginner", Good: "balance", Color: "#22c55e",
		Description: "Breathing at about 5.5 breaths a minute lines the heartbeat up with the breath (resonance) and raises heart rate variability. Ten minutes a day is the classic dose.",
		How:         []string{"In for 5.5 seconds, out for 5.5 seconds, no pauses.", "Keep the breath soft and low, into the belly.", "If it feels like too little air, breathe a little less deeply, not faster."},
		Config:      BreathConfig{Phases: []Phase{ph(In, 5.5), ph(Out, 5.5)}, Minutes: 10}},
	{Key: "box", Name: "Box breathing", Tagline: "Four sides of four", Level: "beginner", Good: "focus", Color: "#6366f1",
		Description: "In, hold, out, hold, four counts each. Used by pilots, surgeons and soldiers to stay level-headed under pressure: the holds give the mind something precise to do.",
		How:         []string{"In for four.", "Hold for four with full lungs, without straining.", "Out for four.", "Hold for four with empty lungs, then start again."},
		Config:      BreathConfig{Phases: []Phase{ph(In, 4), ph(Hold, 4), ph(Out, 4), ph(HoldOut, 4)}, Minutes: 4}},
	{Key: "triangle", Name: "Triangle breathing", Tagline: "In, hold, out", Level: "beginner", Good: "focus", Color: "#8b5cf6",
		Description: "Box breathing without the empty hold: easier if holding with empty lungs feels uncomfortable, and still enough structure to steady attention.",
		How:         []string{"In for four.", "Hold for four.", "Out for four."},
		Config:      BreathConfig{Phases: []Phase{ph(In, 4), ph(Hold, 4), ph(Out, 4)}, Minutes: 4}},
	{Key: "relax478", Name: "4-7-8 breathing", Tagline: "The relaxing breath, for falling asleep", Level: "intermediate", Good: "sleep", Color: "#a855f7",
		Description: "In for four, hold for seven, out for eight with a soft whoosh. The long hold and longer exhale make it a favourite for falling asleep. Start with four rounds; the first rounds ease in.",
		How:         []string{"Rest the tip of the tongue behind the upper front teeth.", "In quietly through the nose for four.", "Hold for seven.", "Out through the mouth for eight, with a whoosh."},
		Caution:     "If you get light-headed, go back to normal breathing for a moment.",
		Config:      BreathConfig{Phases: []Phase{ph(In, 4), ph(Hold, 7), ph(Out, 8)}, Cycles: 4, Ease: true}},
	{Key: "alternate", Name: "Alternate nostril", Tagline: "Nadi shodhana, side to side", Level: "intermediate", Good: "balance", Color: "#f59e0b",
		Description: "A yoga breathing practice: close one nostril with a finger, breathe in through the other, switch sides for the out-breath. Slow, quiet and balancing; a good bridge into sitting.",
		How:         []string{"Right thumb rests by the right nostril, ring finger by the left.", "Follow the cues: in on one side, hold, out on the other.", "Then in on the same side you breathed out of."},
		Config: BreathConfig{Phases: []Phase{
			{Type: In, S: 4, Label: "In through the left"}, {Type: Hold, S: 4, Label: "Hold, close both"}, {Type: Out, S: 6, Label: "Out through the right"},
			{Type: In, S: 4, Label: "In through the right"}, {Type: Hold, S: 4, Label: "Hold, close both"}, {Type: Out, S: 6, Label: "Out through the left"},
		}, Minutes: 5}},
	{Key: "energize", Name: "Energizing breath", Tagline: "Long in, short out, to wake up", Level: "beginner", Good: "energy", Color: "#f97316",
		Description: "The mirror image of the long exhale: a full, longer in-breath and a quick out-breath nudge you towards alertness. Two or three minutes in the morning or after lunch.",
		How:         []string{"In through the nose, filling from the belly up, for six.", "Out briskly for two.", "Sit upright; stop if you feel dizzy."},
		Caution:     "Do it sitting down, never in water or while driving.",
		Config:      BreathConfig{Phases: []Phase{ph(In, 6), ph(Out, 2)}, Minutes: 2}},
}

// PatternByKey finds a built-in pattern.
func PatternByKey(key string) (Pattern, bool) {
	for _, p := range Patterns {
		if p.Key == key {
			return p, true
		}
	}
	return Pattern{}, false
}

// ---------------- meditation ----------------

// Sounds the timer and the breathing player can make (synthesised in the browser).
var Bells = []string{"bowl", "bell", "chime", "gong", "wood", "none"}

// Ambients are the background sounds (synthesised in the browser; music:<id>
// plays a playlist from Navidrome).
var Ambients = []string{"none", "rain", "ocean", "stream", "brown", "pink", "fire"}

// TimerConfig is a meditation timer.
type TimerConfig struct {
	Minutes      float64 `json:"minutes"`       // 0 = open-ended
	WarmupS      int     `json:"warmup_s"`      // settle in before the first bell
	IntervalMin  float64 `json:"interval_min"`  // a bell every n minutes, 0 = none
	StartBell    string  `json:"start_bell"`    // see Bells
	IntervalBell string  `json:"interval_bell"` // see Bells
	EndBell      string  `json:"end_bell"`      // see Bells
	Ambient      string  `json:"ambient"`       // see Ambients, or music:<playlist id>
	Style        string  `json:"style"`         // a meditation style key
	Guidance     string  `json:"guidance"`      // off | light | full: the style's cues, spoken and on screen
}

// Validate checks a person's own timer.
func (c TimerConfig) Validate() error {
	if c.Minutes < 0 || c.Minutes > 240 || c.WarmupS < 0 || c.WarmupS > 600 || c.IntervalMin < 0 || c.IntervalMin > 120 {
		return fmt.Errorf("up to 240 minutes, a warm-up of up to 10 minutes, a bell up to every 120 minutes")
	}
	for _, b := range []string{c.StartBell, c.IntervalBell, c.EndBell} {
		if b != "" && !contains(Bells, b) {
			return fmt.Errorf("unknown bell %q", b)
		}
	}
	if c.Ambient != "" && !contains(Ambients, c.Ambient) && !strings.HasPrefix(c.Ambient, "music:") {
		return fmt.Errorf("unknown background sound %q", c.Ambient)
	}
	if c.Style != "" {
		if _, ok := StyleByKey(c.Style); !ok {
			return fmt.Errorf("unknown meditation style %q", c.Style)
		}
	}
	switch c.Guidance {
	case "", "off", "light", "full":
	default:
		return fmt.Errorf("guidance is off, light or full")
	}
	return nil
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// Cue is something said during a sit, at a fraction of the way through
// (0 = the start, 1 = the end). Light guidance keeps only the Key cues.
type Cue struct {
	At   float64 `json:"at"`
	Text string  `json:"text"`
	Key  bool    `json:"key,omitempty"`
}

// Style is a way of meditating.
type Style struct {
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Kind        string   `json:"kind"` // the practice kind it is logged as
	Tagline     string   `json:"tagline"`
	Description string   `json:"description"`
	How         []string `json:"how"`
	Minutes     float64  `json:"minutes"` // a good first length
	Color       string   `json:"color"`
	Cues        []Cue    `json:"cues"`
}

// Practice kinds.
var Kinds = []struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}{
	{"breath", "Breathing", "wind"}, {"meditation", "Meditation", "lotus"}, {"body_scan", "Body scan", "scan"},
	{"loving_kindness", "Loving-kindness", "heart"}, {"walking", "Mindful walk", "footprints"}, {"other", "Other", "sparkles"},
}

// KindName is the display name of a practice kind.
func KindName(k string) string {
	for _, x := range Kinds {
		if x.Key == k {
			return x.Name
		}
	}
	return "Practice"
}

// ValidKind reports whether k is a practice kind.
func ValidKind(k string) bool {
	for _, x := range Kinds {
		if x.Key == k {
			return true
		}
	}
	return false
}

var bodyParts = []string{"the top of your head", "your forehead and the muscles around the eyes", "your jaw; let it loosen", "your neck and throat",
	"your shoulders; let them sink", "your upper arms", "your forearms and wrists", "your hands, palms and fingers", "your chest, rising and falling",
	"your upper back", "your belly, soft with each breath", "your lower back", "your hips and pelvis", "your thighs", "your knees",
	"your calves and shins", "your ankles", "your feet, heels, soles and toes"}

func bodyScanCues() []Cue {
	cues := []Cue{{At: 0, Text: "Settle in, lying down or sitting. Close your eyes if you like, and take three slow breaths.", Key: true}}
	n := len(bodyParts)
	for i, p := range bodyParts {
		at := 0.06 + 0.82*float64(i)/float64(n)
		t := "Bring your attention to " + p + "."
		if i == 0 {
			t = "Begin at " + p + ". Notice whatever is there: warmth, tingling, pressure, or nothing at all."
		}
		cues = append(cues, Cue{At: math.Round(at*1000) / 1000, Text: t, Key: i%4 == 0})
	}
	cues = append(cues, Cue{At: 0.9, Text: "Now feel the whole body at once, breathing.", Key: true},
		Cue{At: 0.97, Text: "Slowly let the movement come back to your fingers and toes."})
	return cues
}

// Styles are the meditation styles the timer and the lessons use.
var Styles = []Style{
	{Key: "breath_focus", Name: "Breath focus", Kind: "meditation", Minutes: 5, Color: "#0ea5e9",
		Tagline:     "Rest attention on the breath",
		Description: "The foundation of most practice: feel the breath where it is clearest and come back to it every time the mind wanders. The coming back is the exercise, not a failure.",
		How:         []string{"Sit upright but relaxed.", "Feel the breath at the nostrils, the chest or the belly.", "When you notice you're thinking, note \"thinking\" and return to the breath."},
		Cues: []Cue{{0, "Settle into your seat. Let the eyes close or rest the gaze on the floor.", true}, {0.08, "Find the breath where you feel it most clearly.", true},
			{0.35, "If the mind has wandered, that's fine. Notice where it went, and come back.", false}, {0.6, "Feel one full breath, from beginning to end.", true},
			{0.85, "Rest here, with the breath, a little longer.", false}}},
	{Key: "counting", Name: "Counting breaths", Kind: "meditation", Minutes: 8, Color: "#6366f1",
		Tagline:     "Count one to ten, then start again",
		Description: "Counting gives attention a handrail. Count each out-breath up to ten and start over at one; if you lose count or find yourself at seventeen, simply begin again.",
		How:         []string{"Breathe naturally.", "Count \"one\" on the out-breath, \"two\" on the next, up to ten.", "Lost count? Back to one, kindly."},
		Cues: []Cue{{0, "Settle in and let the breath find its own pace.", true}, {0.05, "On the next out-breath, count one.", true},
			{0.4, "If you've lost count, start again at one. No problem at all.", false}, {0.7, "Keep counting, one to ten.", true}, {0.9, "Let the counting go, and just breathe.", false}}},
	{Key: "noting", Name: "Noting", Kind: "meditation", Minutes: 10, Color: "#f59e0b",
		Tagline:     "Name what shows up, lightly",
		Description: "Rest with the breath, and when something pulls attention away, give it a soft one-word label: thinking, hearing, feeling, planning. Labelling creates a little space between you and the experience.",
		How:         []string{"Anchor on the breath.", "When something arises, note it once: \"hearing\", \"thinking\", \"itching\".", "Return to the breath."},
		Cues: []Cue{{0, "Settle in and rest with the breath.", true}, {0.1, "When something takes your attention, name it softly: thinking, hearing, feeling.", true},
			{0.4, "No need to push anything away. Note it, and come back.", false}, {0.65, "Notice sounds now, as they come and go. Hearing.", true}, {0.88, "Return to the breath for these last moments.", false}}},
	{Key: "body_scan", Name: "Body scan", Kind: "body_scan", Minutes: 12, Color: "#14b8a6",
		Tagline:     "Head to toe, part by part",
		Description: "Move attention slowly through the body and notice what each part feels like, without changing anything. Good for tension, for falling asleep and for learning to feel what is actually there.",
		How:         []string{"Lie down or sit comfortably.", "Follow the cues from the head to the feet.", "If a part feels like nothing, notice the nothing."},
		Cues:        bodyScanCues()},
	{Key: "loving_kindness", Name: "Loving-kindness", Kind: "loving_kindness", Minutes: 10, Color: "#ec4899",
		Tagline:     "Wish yourself and others well",
		Description: "Metta: silently offer simple wishes, first to yourself, then to someone you love, someone neutral, someone difficult, and finally everyone. Warmth is welcome but not required; the intention is the practice.",
		How:         []string{"Repeat the phrases slowly, in your own rhythm.", "Picture each person as you say them.", "If a feeling resists, notice that kindly too."},
		Cues: []Cue{{0, "Settle in and take a few easy breaths.", true}, {0.05, "Bring yourself to mind. May I be safe. May I be well. May I be at ease.", true},
			{0.25, "Now someone you love. May you be safe. May you be well. May you be at ease.", true}, {0.45, "Someone you barely know, a neighbour or a cashier. May you be safe and well.", true},
			{0.62, "Someone you find difficult, as far as feels right. May you be at ease.", true}, {0.8, "Everyone, everywhere. May all beings be safe, well and at ease.", true},
			{0.94, "Rest in whatever is here now.", false}}},
	{Key: "open", Name: "Open awareness", Kind: "meditation", Minutes: 15, Color: "#64748b",
		Tagline:     "Let everything come and go",
		Description: "Choiceless awareness: no single anchor. Sounds, sensations, thoughts and feelings come and go, and you rest as the space they happen in. Best after some practice with an anchor.",
		How:         []string{"Start with a few breaths to settle.", "Widen attention to everything at once.", "Don't follow anything; let it pass through."},
		Cues: []Cue{{0, "Settle in with a few slow breaths.", true}, {0.1, "Let go of the breath as an anchor. Open to whatever is here.", true},
			{0.5, "Sounds, sensations, thoughts. Let them come and let them go.", false}, {0.9, "Notice the awareness itself, quiet and spacious.", false}}},
	{Key: "walking", Name: "Mindful walk", Kind: "walking", Minutes: 10, Color: "#84cc16",
		Tagline:     "Feel every step",
		Description: "Walk slower than usual and feel the lifting, moving and placing of each foot. Indoors along a short line, or outside on a quiet path; phone in the pocket with the voice on.",
		How:         []string{"Walk at a slow, natural pace.", "Feel the soles of the feet with each step.", "When the mind wanders, come back to the feet."},
		Cues: []Cue{{0, "Stand still for a moment and feel your weight on your feet.", true}, {0.05, "Begin walking slowly. Feel the lifting, moving and placing of each foot.", true},
			{0.4, "Notice the ground, the air, the sounds around you.", false}, {0.7, "Back to the feet: lifting, moving, placing.", true}, {0.95, "Come to a stop and stand still for a few breaths.", false}}},
	{Key: "silent", Name: "Silent sit", Kind: "meditation", Minutes: 10, Color: "#475569",
		Tagline:     "Just the bells",
		Description: "No guidance: a bell to begin, optional interval bells, a bell to end. Practise in whichever way you like.",
		How:         []string{"Choose your length and bells.", "Sit.", "That's all."}},
}

// StyleByKey finds a style.
func StyleByKey(key string) (Style, bool) {
	for _, s := range Styles {
		if s.Key == key {
			return s, true
		}
	}
	return Style{}, false
}

// TechniqueName names a pattern or style key ("" when unknown).
func TechniqueName(key string) string {
	if p, ok := PatternByKey(key); ok {
		return p.Name
	}
	if s, ok := StyleByKey(key); ok {
		return s.Name
	}
	return ""
}

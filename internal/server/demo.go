package server

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/abjelosevic88/pocket-homelab-still/internal/content"
	"github.com/abjelosevic88/pocket-homelab-still/internal/store"
)

// seedDemo fills the owner's account with four months of plausible practice
// (DEMO_DATA=true, for screenshots and trying the app). Runs once.
func (s *Server) seedDemo(ctx context.Context, owner int64) error {
	key := fmt.Sprintf("seeded:%d:demo", owner)
	if _, _, done := s.store.CacheGet(ctx, key); done {
		return nil
	}
	rng := rand.New(rand.NewSource(42))
	today, _ := store.ParseDate(store.Today())
	path, _ := content.ProgramByKey("path")
	lessons := path.Lessons()
	breathe, _ := content.ProgramByKey("breathe")
	courseLessons := breathe.Lessons()
	li, ci := 0, 0
	notes := []string{
		"Mind was busy with work at first, settled after a few minutes.",
		"Felt the breath mostly in the belly today.",
		"Sleepy. Sat up straighter and it helped.",
		"Noticed how much tension sits in my jaw.",
		"Calm afterwards, a good start to the day.",
		"Lots of planning thoughts. Kept coming back.",
		"Did this before a difficult call; it worked.",
		"Rain outside, very peaceful.",
		"", "", "", "", "", "",
	}
	clamp := func(v int) *int {
		if v < 1 {
			v = 1
		}
		if v > 5 {
			v = 5
		}
		return &v
	}
	for d := 120; d >= 0; d-- {
		day := today.AddDate(0, 0, -d)
		// practice on most days, more often as the habit forms; a gap now and then
		chance := 0.55 + 0.35*float64(120-d)/120
		if d > 60 && d < 66 {
			chance = 0.1 // a holiday
		}
		if rng.Float64() > chance {
			continue
		}
		sessions := 1
		if rng.Float64() < 0.25 {
			sessions = 2
		}
		for n := 0; n < sessions; n++ {
			hour := 7 + rng.Intn(2)
			if n == 1 || rng.Float64() < 0.3 {
				hour = 20 + rng.Intn(3)
			}
			start := time.Date(day.Year(), day.Month(), day.Day(), hour, rng.Intn(60), 0, 0, time.Local)
			if start.After(time.Now()) {
				continue
			}
			p := store.Practice{Source: "app", Completed: rng.Float64() > 0.06}
			switch {
			case li < len(lessons) && n == 0 && rng.Float64() < 0.7:
				l := lessons[li]
				p.Kind, p.Technique, p.Title = l.Lesson.Practice.Kind, l.Lesson.Practice.Technique, l.Lesson.Title
				p.PlannedS = int(l.Lesson.Practice.Minutes * 60)
				p.Program, p.Lesson = "path", l.Lesson.Key
				p.Completed = true
				li++
			case ci < len(courseLessons) && d < 40 && rng.Float64() < 0.4:
				l := courseLessons[ci]
				p.Kind, p.Technique, p.Title = l.Lesson.Practice.Kind, l.Lesson.Practice.Technique, l.Lesson.Title
				p.PlannedS = int(l.Lesson.Practice.Minutes * 60)
				p.Program, p.Lesson = "breathe", l.Lesson.Key
				p.Completed = true
				ci++
			case hour >= 20:
				opts := []string{"relax478", "downshift", "body_scan", "coherent"}
				p.Technique = opts[rng.Intn(len(opts))]
			default:
				opts := []string{"box", "coherent", "sigh", "breath_focus", "counting", "noting", "silent", "equal"}
				p.Technique = opts[rng.Intn(len(opts))]
			}
			if p.Kind == "" {
				if pt, ok := content.PatternByKey(p.Technique); ok {
					p.Kind, p.Title = "breath", pt.Name
					p.PlannedS = int(pt.Config.Minutes * 60)
					if pt.Config.Cycles > 0 {
						p.PlannedS = int(float64(pt.Config.Cycles) * pt.Config.CycleSeconds())
						p.Cycles = pt.Config.Cycles
					}
				} else if st, ok := content.StyleByKey(p.Technique); ok {
					p.Kind, p.Title = st.Kind, st.Name
					m := st.Minutes + float64(rng.Intn(3)*5)*float64(120-d)/120
					p.PlannedS = int(m * 60)
				}
			}
			p.DurationS = p.PlannedS
			if !p.Completed {
				p.DurationS = int(float64(p.PlannedS) * (0.3 + 0.5*rng.Float64()))
			}
			if p.Kind == "breath" && p.Cycles == 0 {
				if pt, ok := content.PatternByKey(p.Technique); ok && pt.Config.CycleSeconds() > 0 {
					p.Cycles = int(float64(p.DurationS) / pt.Config.CycleSeconds())
				}
			}
			p.StartedAt = start.Format(time.RFC3339)
			if rng.Float64() < 0.8 {
				b := 2 + rng.Intn(3)
				gain := 1
				if p.Technique == "downshift" || p.Technique == "sigh" || p.Technique == "body_scan" {
					gain = 1 + rng.Intn(2)
				}
				if rng.Float64() < 0.15 {
					gain = 0
				}
				p.CalmBefore, p.CalmAfter = clamp(b), clamp(b+gain)
			}
			p.Note = notes[rng.Intn(len(notes))]
			if rng.Float64() < 0.2 {
				p.Tags = []string{"morning", "work", "sleep", "travel"}[rng.Intn(4)]
			}
			p.ExternalID = fmt.Sprintf("demo-%d-%d", d, n)
			if err := s.store.CreatePractice(ctx, owner, &p); err != nil {
				return err
			}
			now := store.Now()
			_ = s.store.SetHealthSync(ctx, owner, p.ID, now, "")
			_ = s.store.SetHabitMark(ctx, owner, p.ID, now, "")
			if p.Program != "" {
				if _, err := s.store.CompleteLesson(ctx, owner, p.Program, p.Lesson, p.ID); err != nil {
					return err
				}
			}
		}
	}
	_ = s.store.StartProgram(ctx, owner, "path")
	_ = s.store.StartProgram(ctx, owner, "breathe")
	for _, k := range []string{"box", "coherent", "relax478", "body_scan"} {
		_ = s.store.SetFavorite(ctx, owner, k, true)
	}
	cfg, _ := json.Marshal(content.BreathConfig{Phases: []content.Phase{{Type: content.In, S: 5}, {Type: content.Hold, S: 2}, {Type: content.Out, S: 7}}, Minutes: 6})
	_ = s.store.CreatePreset(ctx, owner, &store.Preset{Kind: "breath", Name: "Evening 5-2-7", Config: cfg, Favorite: true})
	tc, _ := json.Marshal(content.TimerConfig{Minutes: 20, WarmupS: 30, IntervalMin: 5, StartBell: "bowl", IntervalBell: "chime", EndBell: "gong", Ambient: "rain", Style: "silent", Guidance: "off"})
	_ = s.store.CreatePreset(ctx, owner, &store.Preset{Kind: "timer", Name: "Morning sit, 20 min", Config: tc, Favorite: true})
	// milestones on the day they were reached would need replaying; unlock them as of now
	s.checkAchievements(ctx, owner, nil)
	return s.store.CachePut(ctx, key, "1")
}

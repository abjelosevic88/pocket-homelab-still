package server

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-still/internal/ai"
	"github.com/abjelosevic88/pocket-homelab-still/internal/content"
	"github.com/abjelosevic88/pocket-homelab-still/internal/store"
)

// aiFor is the person's AI: Pocket AI through Core, their own
// OpenAI-compatible key, or nil with why.
func (s *Server) aiFor(ctx context.Context, uid int64, st store.Settings) (*ai.Client, string, string) {
	email := ""
	if u, err := s.store.GetUser(ctx, uid); err == nil {
		email = u.Email
	}
	pocket, status := s.core.SelectAI(ctx, email, st.AIProvider, st.OpenAIKey != "")
	if pocket != nil {
		if !status.Ready {
			return nil, "", "Pocket AI: " + status.Detail
		}
		c := ai.New("", "", "", 150*time.Second)
		c.Pocket = pocket.WithModel(st.AIPocketModel)
		return c, "pocket", ""
	}
	if st.OpenAIKey == "" {
		return nil, "", status.Detail
	}
	return ai.New(st.OpenAIBaseURL, st.OpenAIKey, st.OpenAIModel, 150*time.Second), "openai", ""
}

func (s *Server) handleAIStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	st, _ := s.store.GetSettings(ctx, actorID(ctx))
	cl, used, why := s.aiFor(ctx, actorID(ctx), st)
	writeJSON(w, http.StatusOK, map[string]any{"ready": cl != nil, "provider": used, "detail": why})
}

func (s *Server) handleAIModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c, status := s.core.SelectAI(ctx, userFrom(ctx).Email, "pocket", false)
	if c == nil || !status.Ready {
		writeErr(w, http.StatusServiceUnavailable, status.Detail)
		return
	}
	models, err := c.Models(ctx)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, models)
}

func validAIProvider(v string) bool { return core.ValidAIProvider(v) }

func validPocketModel(v string) bool { return core.ValidInferenceModel(v) }

const aiVoice = `You are a calm, plain-spoken meditation teacher looking at someone's own practice log: breathing exercises and sits with their length, the time of day, how they felt before and after (1 tense … 5 calm) and their own notes. Use only the data given; never invent sessions or numbers. You are not a therapist: no diagnoses; if notes suggest real distress, gently suggest talking to someone they trust or a professional. Be concrete, short and warm. No emojis, no exclamation marks, no spiritual jargon, no motivational filler.`

// practiceDigest renders a period's practice as compact text for the model.
func (s *Server) practiceDigest(ctx context.Context, uid int64, from, to string) string {
	ps, _, err := s.store.ListPractices(ctx, uid, store.PracticeFilter{From: from, To: to, Asc: true})
	if err != nil || len(ps) == 0 {
		return ""
	}
	var b strings.Builder
	st, _ := s.store.GetSettings(ctx, uid)
	cur, best := s.streaks(ctx, uid)
	life, _ := s.store.LifetimeTotals(ctx, uid)
	lvl, _ := content.LevelFor(life.Minutes)
	fmt.Fprintf(&b, "Period %s to %s. Daily goal %d min. Current streak %d days (longest %d). Lifetime %s over %d practices (level %s).\n",
		from, to, st.DailyGoalMin, cur, best, fmtMinutes(life.Minutes), life.Sessions, lvl.Name)
	if v, err := s.programByKeyProgress(ctx, uid, "path"); err == nil && v != nil {
		fmt.Fprintf(&b, "The Path: %d of %d lessons done", v.Done, v.Total)
		if v.Next != nil {
			fmt.Fprintf(&b, ", next \"%s\" in stage %s", v.Next.Title, v.StageName)
		}
		b.WriteString(".\n")
	}
	b.WriteString("Practices (date time | what | minutes | calm before→after | note):\n")
	for _, p := range ps {
		t := ""
		if tt, err := time.Parse(time.RFC3339, p.StartedAt); err == nil {
			t = tt.Format("Mon 2006-01-02 15:04")
		}
		calm := "-"
		if p.CalmBefore != nil || p.CalmAfter != nil {
			calm = fmt.Sprintf("%s→%s", ptrStr(p.CalmBefore), ptrStr(p.CalmAfter))
		}
		note := strings.ReplaceAll(p.Note, "\n", " ")
		if len(note) > 280 {
			note = note[:280] + "…"
		}
		done := ""
		if !p.Completed {
			done = " (stopped early)"
		}
		fmt.Fprintf(&b, "%s | %s (%s)%s | %.0f | %s | %s\n", t, p.Title, content.KindName(p.Kind), done, p.Minutes(), calm, note)
	}
	return b.String()
}

func ptrStr(p *int) string {
	if p == nil {
		return "?"
	}
	return fmt.Sprint(*p)
}

// handleReflect writes a reflection on a period (the week by default).
func (s *Server) handleReflect(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	st, _ := s.store.GetSettings(ctx, uid)
	cl, used, why := s.aiFor(ctx, uid, st)
	if cl == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": why})
		return
	}
	var in struct {
		Range string `json:"range"`
	}
	_ = decode(r, &in)
	days := rangeDays(firstNonEmpty(in.Range, "w"))
	to := store.Today()
	from := store.AddDays(to, -(days - 1))
	digest := s.practiceDigest(ctx, uid, from, to)
	if digest == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "no practice in this period yet"})
		return
	}
	prompt := "Write a short reflection on this period of practice in 4–7 sentences: how consistent it was, what seemed to help (techniques, times of day, what the notes say), one thing worth noticing, and one small, concrete suggestion for the next week, naming a specific technique or lesson. End with one line 'Try next:' and the suggestion in a few words.\n\n" + digest
	text, err := cl.Complete(ctx, aiVoice, prompt, false)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "provider": used, "from": from, "to": to, "text": strings.TrimSpace(text)})
}

// handleAsk answers a question about the person's practice.
func (s *Server) handleAsk(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	st, _ := s.store.GetSettings(ctx, uid)
	cl, used, why := s.aiFor(ctx, uid, st)
	if cl == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": why})
		return
	}
	var in struct {
		Question string `json:"question"`
		Range    string `json:"range"`
	}
	if err := decode(r, &in); err != nil || strings.TrimSpace(in.Question) == "" {
		writeErr(w, http.StatusBadRequest, "question is required")
		return
	}
	days := rangeDays(firstNonEmpty(in.Range, "3m"))
	to := store.Today()
	from := store.AddDays(to, -(days - 1))
	digest := s.practiceDigest(ctx, uid, from, to)
	if digest == "" {
		digest = "No practice logged in this period.\n"
	}
	var tech []string
	for _, p := range content.Patterns {
		tech = append(tech, p.Name+" ("+p.Good+")")
	}
	for _, sty := range content.Styles {
		tech = append(tech, sty.Name)
	}
	sort.Strings(tech)
	prompt := "Question: " + strings.TrimSpace(in.Question) + "\n\nTechniques available in the app: " + strings.Join(tech, ", ") + ".\n\nTheir practice:\n" + digest +
		"\nAnswer in a few sentences, with the data that backs it. If the data cannot answer it, say so and answer from general practice knowledge, marked as such."
	text, err := cl.Complete(ctx, aiVoice, prompt, false)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "provider": used, "text": strings.TrimSpace(text)})
}

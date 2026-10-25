package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/abjelosevic88/pocket-homelab-still/internal/content"
	"github.com/abjelosevic88/pocket-homelab-still/internal/store"
)

// handleLibrary is everything a person can start: the built-in breathing
// patterns and meditation styles (with their stars and how often they were
// used), their own presets, the bells and background sounds.
func (s *Server) handleLibrary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	favs, _ := s.store.Favorites(ctx, uid)
	presets, err := s.store.ListPresets(ctx, uid, "")
	if err != nil {
		s.fail(w, err, "library")
		return
	}
	all, _, _ := s.store.ListPractices(ctx, uid, store.PracticeFilter{})
	type use struct {
		Count   int     `json:"count"`
		Minutes float64 `json:"minutes"`
		Last    string  `json:"last"`
	}
	uses := map[string]*use{}
	for _, p := range all {
		u := uses[p.Technique]
		if u == nil {
			u = &use{}
			uses[p.Technique] = u
		}
		u.Count++
		u.Minutes += p.Minutes()
		if p.Date > u.Last {
			u.Last = p.Date
		}
	}
	for _, u := range uses {
		u.Minutes = round1(u.Minutes)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"patterns": content.Patterns, "styles": content.Styles, "kinds": content.Kinds,
		"bells": content.Bells, "ambients": content.Ambients,
		"presets": presets, "favorites": favs, "uses": uses,
	})
}

// handleTechnique is one built-in pattern or style.
func (s *Server) handleTechnique(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if p, ok := content.PatternByKey(key); ok {
		writeJSON(w, http.StatusOK, map[string]any{"type": "pattern", "pattern": p})
		return
	}
	if st, ok := content.StyleByKey(key); ok {
		writeJSON(w, http.StatusOK, map[string]any{"type": "style", "style": st})
		return
	}
	writeErr(w, http.StatusNotFound, "not found")
}

func (s *Server) handleSetFavorite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	key := chi.URLParam(r, "key")
	if content.TechniqueName(key) == "" {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	var in struct {
		On bool `json:"on"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := s.store.SetFavorite(ctx, actorID(ctx), key, in.On); err != nil {
		s.fail(w, err, "favorite")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"key": key, "on": in.On})
}

type presetInput struct {
	Kind     string          `json:"kind"`
	Name     string          `json:"name"`
	Config   json.RawMessage `json:"config"`
	Favorite bool            `json:"favorite"`
	Sort     int             `json:"sort"`
}

// check validates a preset's config for its kind and returns it normalized.
func (in *presetInput) check() (json.RawMessage, string) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len(in.Name) > 80 {
		return nil, "a name of up to 80 characters is required"
	}
	switch in.Kind {
	case "breath":
		var c content.BreathConfig
		if err := json.Unmarshal(in.Config, &c); err != nil {
			return nil, "config is not a breathing pattern"
		}
		if err := c.Validate(); err != nil {
			return nil, err.Error()
		}
		if c.Cycles == 0 && c.Minutes == 0 {
			c.Minutes = 5
		}
		b, _ := json.Marshal(c)
		return b, ""
	case "timer":
		var c content.TimerConfig
		if err := json.Unmarshal(in.Config, &c); err != nil {
			return nil, "config is not a timer"
		}
		if err := c.Validate(); err != nil {
			return nil, err.Error()
		}
		b, _ := json.Marshal(c)
		return b, ""
	}
	return nil, "kind is breath or timer"
}

func (s *Server) handleListPresets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ps, err := s.store.ListPresets(ctx, actorID(ctx), r.URL.Query().Get("kind"))
	if err != nil {
		s.fail(w, err, "presets")
		return
	}
	writeJSON(w, http.StatusOK, ps)
}

func (s *Server) handleCreatePreset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in presetInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	cfg, msg := in.check()
	if msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	p := store.Preset{Kind: in.Kind, Name: in.Name, Config: cfg, Favorite: in.Favorite, Sort: in.Sort}
	if err := s.store.CreatePreset(ctx, uid, &p); err != nil {
		s.fail(w, err, "save preset")
		return
	}
	if ok, _ := s.store.Unlock(ctx, uid, "own_pattern", ""); ok {
		s.announceMilestones(ctx, uid, []string{"own_pattern"})
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleUpdatePreset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	cur, err := s.store.GetPreset(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "preset")
		return
	}
	in := presetInput{Kind: cur.Kind, Name: cur.Name, Config: cur.Config, Favorite: cur.Favorite, Sort: cur.Sort}
	if _, err := decodeKeys(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	in.Kind = cur.Kind // a preset keeps its kind
	cfg, msg := in.check()
	if msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	cur.Name, cur.Config, cur.Favorite, cur.Sort = in.Name, cfg, in.Favorite, in.Sort
	if err := s.store.UpdatePreset(ctx, uid, cur); err != nil {
		s.fail(w, err, "save preset")
		return
	}
	writeJSON(w, http.StatusOK, cur)
}

func (s *Server) handleDeletePreset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.store.DeletePreset(ctx, actorID(ctx), idParam(r, "id")); err != nil {
		s.fail(w, err, "delete preset")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

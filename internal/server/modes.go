package server

import (
	"context"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
)

// ---- connection modes on Pocket Core's map (appkit.ModeReporter sends them) ----
//
// Core can switch its own links but cannot see a URL and token pasted into
// Still, so Still tells it how each person actually connects to
// Pocket Habits: "core", "direct" or "off", the mode in effect
// (what Settings shows, defaults included). Reported after each registration
// (on start and every 30 minutes) and right after a person changes a mode or
// disconnects; one batched call for everybody. Report only: failures are
// logged and nothing depends on them.

// connModes is every person's effective mode for every Pocket app.
func (s *Server) connModes(ctx context.Context) ([]core.ConnMode, error) {
	users, err := s.store.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]core.ConnMode, 0, len(users)*len(pocketApps))
	for _, u := range users {
		if u.Email == "" {
			continue
		}
		st, err := s.store.GetSettings(ctx, u.ID)
		if err != nil {
			return nil, err
		}
		for _, a := range pocketApps {
			mode, _, token, _ := connSettings(st, a)
			out = append(out, core.ConnMode{User: u.Email, Target: a.slug, Mode: s.connMode(mode, token)})
		}
		out = append(out, core.ConnMode{User: u.Email, Target: "service:navidrome", Mode: s.connMode(st.MusicMode, st.MusicPassword)})
	}
	return out, nil
}

// modeKeys are the settings that can change a person's effective mode: the
// mode itself, and the token (a saved token makes the default direct).
var modeKeys = []string{"habits_mode", "habits_token", "health_mode", "health_token", "music_mode", "music_password"}

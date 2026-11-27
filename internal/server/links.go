package server

import (
	"context"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-core/sdk/appkit"
)

// Pocket Core app links (docs/links.md in pocket-homelab-core), Still as the
// target: another Pocket app the person allowed in Core may act here for
// them with a pcx_ token, only within these scopes. Every route not listed
// in linkRoutes stays closed to it (403).

var linkOffers = []core.Offer{
	{Scope: "mindfulness:read", Name: "See your practice", Description: "Read your breathing and meditation practices, minutes, streaks and progress on the Path — for your journal, calendar, health and dashboards. Never your notes."},
	{Scope: "mindfulness:write", Name: "Log practices", Description: "Add breathing or meditation practices done elsewhere (a watch, another app). It may change nothing you logged yourself."},
}

// portable is what Pocket Move may copy out and bring in for a person
// (docs/move.md in pocket-homelab-core); Register adds move:export/import.
var portable = &core.Portable{
	Exports: []core.PortableExport{
		{Path: "/api/v1/export.json", File: "still.json", Kind: core.KindPocket, Title: "Every practice with its notes, your presets, lessons done, favourites and habit rules"},
		{Path: "/api/v1/export.csv", File: "practices.csv", Kind: core.KindCSV, Title: "Every practice as a table"},
	},
	Imports: []core.PortableImport{
		{Path: "/api/v1/import", Kind: core.KindPocket, ContentType: "application/json", MaxBytes: 32 << 20, Title: "Adds the practices, presets, lessons, favourites and habit rules from another box; what is already here is skipped"},
	},
}

// capabilities is what Pocket Search, Pocket Day and Pocket Hub may ask Still
// (docs/capabilities.md in pocket-homelab-core); Register adds the pocket:*
// offers, capabilities.go answers them. No quick capture: Still has no
// one-line way to log a practice, only the form.
var capabilities = core.Capabilities{
	Search: &core.Searchable{What: "Your own patterns and timers, the built-in practices, courses and lessons by name, and your sessions by the note you wrote", Kinds: []string{"practice", "course", "lesson", "session"}, Bangs: []string{"still", "meditate"}},
	Day:    &core.DayFeed{Label: "Still", One: "session", Many: "sessions", Unit: "min", What: "Your breathing and meditation sessions that day, the lessons you finished and the milestones you reached"},
	Widget: &core.Widget{Title: "Still", What: "Today's minutes against your daily goal, your streak, and what to practise next"},
}

// linkRoutes are the requests a linked app may make with its scopes; every
// other route stays closed to it (appkit.Routes, with the Portable and
// Capabilities routes added in newAuth).
var linkRoutes = appkit.Routes{
	appkit.R("GET", `^/api/v1/auth/me$`, ""),
	appkit.R("GET", `^/api/v1/(dashboard|overlay|stats|practices|programs|achievements)$`, "mindfulness:read"),
	appkit.R("GET", `^/api/v1/day/\d{4}-\d{2}-\d{2}$`, "mindfulness:read"),
	appkit.R("GET", `^/api/v1/summary/\d{4}-\d{2}-\d{2}$`, "mindfulness:read"),
	appkit.R("POST", `^/api/v1/practices$`, "mindfulness:write"),
	appkit.R("PUT DELETE", `^/api/v1/practices/\d+$`, "mindfulness:write"),
}

// linkAllows reports whether a link token with these scopes may make a
// request: linkRoutes plus the Portable and Capabilities routes.
var linkAllows = linkRoutes.Allow(portable, capabilities)

const linkAppKey ctxKey = 2

// linkApp is the acting app's slug when the request came in on a link token.
func linkApp(ctx context.Context) string {
	a, _ := ctx.Value(linkAppKey).(string)
	return a
}

package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"dapper.local/platform/internal/postgres"
)

// durationMinutes turns a BookingPress duration such as "2" with unit "h" into minutes; ok is false for anything else.
func durationMinutes(val, unit string) (int, bool) {
	var n int
	if _, err := fmt.Sscan(strings.TrimSpace(val), &n); err != nil || n < 1 {
		return 0, false
	}
	switch strings.TrimSpace(unit) {
	case "m":
		return n, true
	case "h":
		return n * 60, true
	case "d":
		return n * 24 * 60, true
	}
	return 0, false
}

// setDurations writes the export's service durations on the matching central main services. Add-ons carry no duration in the export,
// so they keep none. Linked franchise services follow through the central revision. Only changes are sent.
func setDurations(ctx context.Context, store *postgres.Store, actor, org string, ex export, visible map[string]bool, central map[string]postgres.CentralService, apply bool) {
	type change struct {
		label   string
		c       postgres.CentralService
		minutes int
	}
	var todo []change
	var missing int
	for _, sv := range ex.Services {
		if !visible[sv.ID] {
			continue
		}
		name := strings.TrimSpace(sv.Name)
		c, ok := central["|"+strings.ToLower(name)]
		m, valid := durationMinutes(sv.Duration, sv.Unit)
		if !ok || !valid {
			missing++
			reason := "not in the active central catalogue"
			if ok {
				reason = "unusable duration " + sv.Duration + sv.Unit
			}
			fmt.Printf("  no duration set: %s (%s)\n", name, reason)
			continue
		}
		if c.DurationMinutes != nil && *c.DurationMinutes == m {
			continue
		}
		todo = append(todo, change{name, c, m})
	}
	for _, t := range todo {
		fmt.Printf("  %s: %d min\n", t.label, t.minutes)
	}
	fmt.Printf("Durations: %d to set, %d without a usable match\n", len(todo), missing)
	if !apply {
		fmt.Println("Preview only. Re-run with -apply to write them.")
		return
	}
	for _, t := range todo {
		m := t.minutes
		if _, err := store.SaveCentralService(ctx, actor, org, postgres.CentralServiceInput{ID: t.c.ID, Name: t.c.Name, Kind: t.c.Kind, ParentID: t.c.ParentID, Version: t.c.Version, DurationSet: true, DurationMinutes: &m}); err != nil {
			log.Fatalf("duration for %q failed: %v", t.label, err)
		}
	}
	fmt.Printf("Done. Set %d durations.\n", len(todo))
}

// Operator tool: creates the franchise's resources, weekly hours and one scheduling rule per active catalogue service, using the
// durations and capacities recorded in the owner-supplied BookingPress export. Buffers are 0 as in the export.
// Resources: "<franchise> bays" (4 at a time) for services the export limits to 4 or 2 at a time (2-at-a-time services use 2 of 4 units),
// and "<franchise> large jobs" (1 at a time) for services limited to 1. Weekly hours are applied to NEW resources only.
// Services longer than the longest opening day are NOT created (they could never fit) and are reported instead.
// Without -apply it only reports. Re-running is safe: existing resources and rules are kept.
package main

import (
	"context"
	"dapper.local/platform/internal/postgres"
	"dapper.local/platform/internal/scheduling"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type export struct {
	Services []struct {
		ID       string `json:"bookingpress_service_id"`
		Name     string `json:"bookingpress_service_name"`
		Duration string `json:"bookingpress_service_duration_val"`
		Unit     string `json:"bookingpress_service_duration_unit"`
	} `json:"services"`
	Meta []struct {
		ServiceID string `json:"bookingpress_service_id"`
		Name      string `json:"bookingpress_servicemeta_name"`
		Value     string `json:"bookingpress_servicemeta_value"`
	} `json:"bookingpress_servicesmeta"`
}

type plan struct {
	name     string
	minutes  int
	capacity int
	units    int
	group    string
	working  bool
}

func main() {
	email := flag.String("email", "", "administrator email who performs the change")
	code := flag.String("franchise", "", "franchise code")
	file := flag.String("export", "", "path to the BookingPress export JSON")
	zone := flag.String("timezone", "America/Toronto", "IANA timezone of the franchise")
	open := flag.String("open", "08:00", "daily opening time")
	closeAt := flag.String("close", "21:00", "daily closing time")
	workingTime := flag.Bool("working-time", false, "also create the services longer than one opening day as working-time rules (a day of PPF = one full opening day)")
	apply := flag.Bool("apply", false, "write the changes (default: report only)")
	flag.Parse()
	if *email == "" || *code == "" || *file == "" {
		log.Fatal("-email, -franchise and -export are required")
	}
	raw, err := os.ReadFile(*file)
	if err != nil {
		log.Fatal("export file unreadable")
	}
	var ex export
	if err = json.Unmarshal(raw, &ex); err != nil {
		log.Fatal("export file is not the expected BookingPress JSON")
	}
	visible := map[string]bool{}
	capacity := map[string]int{}
	for _, m := range ex.Meta {
		switch m.Name {
		case "show_service_on_site":
			visible[m.ServiceID] = m.Value == "true"
		case "max_capacity":
			capacity[m.ServiceID], _ = strconv.Atoi(m.Value)
		}
	}
	loc, err := time.LoadLocation(*zone)
	if err != nil {
		log.Fatal("invalid timezone")
	}
	_ = loc
	openMin, closeMin := clock(*open), clock(*closeAt)
	if openMin < 0 || closeMin <= openMin {
		log.Fatal("opening and closing times must be HH:MM with closing after opening")
	}
	longest := closeMin - openMin
	var plans []plan
	var tooLong, unsupported []string
	for _, s := range ex.Services {
		if !visible[s.ID] {
			continue
		}
		v, convErr := strconv.ParseFloat(s.Duration, 64)
		unit := map[string]float64{"m": 1, "h": 60, "d": 1440}[s.Unit]
		if convErr != nil || unit == 0 || v <= 0 {
			unsupported = append(unsupported, s.Name)
			continue
		}
		minutes := int(v * unit)
		if *workingTime && minutes > longest && s.Unit == "d" {
			minutes = int(v) * longest // legacy "N days" = N full opening days of work
		}
		c := capacity[s.ID]
		var p plan
		switch {
		case c == 1:
			p = plan{group: "large jobs", capacity: 1, units: 1}
		case c == 2 || c == 4:
			p = plan{group: "bays", capacity: 4, units: 4 / c}
		default:
			unsupported = append(unsupported, s.Name)
			continue
		}
		p.name, p.minutes = strings.TrimSpace(s.Name), minutes
		if minutes > longest {
			if !*workingTime {
				tooLong = append(tooLong, fmt.Sprintf("%s (%dh%02dm)", p.name, minutes/60, minutes%60))
				continue
			}
			p.working = true
		} else if *workingTime && s.Unit == "d" {
			p.working = true // a one-day job may start mid-day and continue the next morning
		}
		plans = append(plans, p)
	}
	groups := map[string]int{}
	for _, p := range plans {
		groups[p.group]++
	}
	fmt.Printf("Rules to create: %d (bays %d, large jobs %d). Opening day %s-%s (%d minutes).\n", len(plans), groups["bays"], groups["large jobs"], *open, *closeAt, longest)
	fmt.Printf("Not created, longer than one opening day (%d): %s\n", len(tooLong), strings.Join(tooLong, "; "))
	if len(unsupported) > 0 {
		fmt.Printf("Not created, unusable duration or capacity (%d): %s\n", len(unsupported), strings.Join(unsupported, "; "))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	dsn := os.Getenv("DAPPER_DATABASE_URL")
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal("database configuration unavailable")
	}
	defer pool.Close()
	var actor, org, site, siteName string
	err = pool.QueryRow(ctx, `SELECT c.principal_id::text,m.organization_id::text,l.id::text,l.name FROM password_credentials c
 JOIN organization_memberships m ON m.principal_id=c.principal_id JOIN locations l ON l.organization_id=m.organization_id AND upper(l.code)=upper($2)
 WHERE c.email=lower($1)`, *email, *code).Scan(&actor, &org, &site, &siteName)
	if err != nil {
		log.Fatal("administrator or franchise code not found")
	}
	services := map[string]string{} // lower(name) -> franchise catalogue service id
	rows, err := pool.Query(ctx, `SELECT id::text,name FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND kind='main' AND NOT archived`, org, site)
	if err != nil {
		log.Fatal("cannot read the franchise catalogue")
	}
	for rows.Next() {
		var id, name string
		if err = rows.Scan(&id, &name); err == nil {
			services[strings.ToLower(strings.TrimSpace(name))] = id
		}
	}
	rows.Close()
	missing := 0
	for _, p := range plans {
		if _, ok := services[strings.ToLower(p.name)]; !ok {
			missing++
		}
	}
	fmt.Printf("%d of the services are not in this franchise's catalogue and will be skipped.\n", missing)
	if !*apply {
		fmt.Printf("Preview only. Re-run with -apply to create the resources, weekly hours and rules at %s.\n", siteName)
		return
	}
	store, err := postgres.Open(ctx, dsn)
	if err != nil {
		log.Fatal("database unavailable")
	}
	defer store.Close()
	scope := postgres.Scope{ActorID: actor, OrganizationID: org, LocationID: site}
	week := make([]postgres.HourWindow, 0, 7)
	for d := 0; d < 7; d++ {
		week = append(week, postgres.HourWindow{Day: d, Start: *open, End: *closeAt})
	}
	resourceIDs := map[string]string{}
	for group, capacityUnits := range map[string]int{"bays": 4, "large jobs": 1} {
		if groups[group] == 0 {
			continue
		}
		name := siteName + " " + group
		var id string
		err = pool.QueryRow(ctx, `SELECT id::text FROM scheduling_resources WHERE organization_id=$1 AND location_id=$2 AND lower(name)=lower($3)`, org, site, name).Scan(&id)
		if err != nil {
			made, cerr := store.CreateSchedulingResource(ctx, scope, postgres.SchedulingResource{Name: name, Timezone: *zone, Plan: scheduling.Plan{Capacity: capacityUnits}})
			if cerr != nil {
				log.Fatalf("create resource %q failed: %v", name, cerr)
			}
			id = made.ID
			if cerr = store.SaveWeeklyHours(ctx, scope, id, week); cerr != nil {
				log.Fatalf("set weekly hours for %q failed: %v", name, cerr)
			}
			fmt.Printf("Created resource %q (%d at a time) with weekly hours %s-%s every day.\n", name, capacityUnits, *open, *closeAt)
		}
		resourceIDs[group] = id
	}
	created, existing, skipped := 0, 0, 0
	sort.Slice(plans, func(i, j int) bool { return plans[i].name < plans[j].name })
	for _, p := range plans {
		serviceID, ok := services[strings.ToLower(p.name)]
		if !ok {
			skipped++
			continue
		}
		_, err := store.SaveSchedulingRule(ctx, scope, postgres.SchedulingRule{ResourceID: resourceIDs[p.group], ServiceID: serviceID, DurationMinutes: p.minutes, Units: p.units, WorkingTime: p.working})
		switch {
		case err == nil:
			created++
		case errors.Is(err, postgres.ErrSchedulingRuleConflict):
			existing++
		default:
			log.Fatalf("rule for %q failed: %v", p.name, err)
		}
	}
	fmt.Printf("Done. Created %d rules, %d already existed, %d skipped (not in catalogue).\n", created, existing, skipped)
}

func clock(s string) int {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return -1
	}
	return t.Hour()*60 + t.Minute()
}

// Operator tool: pre-fills the service catalogue from the owner-supplied BookingPress export.
// Central catalogue gets one main service per visible BookingPress service and its extras as add-ons; the franchise then enables
// them. Names only: no prices, durations or deposits are imported (those need approved values). Hidden services are skipped.
// Without -apply it only reports what it would do. Re-running is safe: names that already exist are skipped.
package main

import (
	"context"
	"dapper.local/platform/internal/postgres"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type export struct {
	Services []struct {
		ID   string `json:"bookingpress_service_id"`
		Name  string `json:"bookingpress_service_name"`
		Price string `json:"bookingpress_service_price"`
	} `json:"services"`
	Meta []struct {
		ServiceID string `json:"bookingpress_service_id"`
		Name      string `json:"bookingpress_servicemeta_name"`
		Value     string `json:"bookingpress_servicemeta_value"`
	} `json:"bookingpress_servicesmeta"`
	Extras []struct {
		ServiceID string `json:"bookingpress_service_id"`
		Name      string `json:"bookingpress_extra_service_name"`
		Price     string `json:"bookingpress_extra_service_price"`
	} `json:"extra_services"`
}

func main() {
	email := flag.String("email", "", "administrator email who performs the change")
	code := flag.String("franchise", "", "franchise code to enable the services at")
	file := flag.String("export", "", "path to the BookingPress export JSON")
	apply := flag.Bool("apply", false, "write the changes (default: report only)")
	prices := flag.Bool("prices", false, "set the export's prices (before tax) on the existing central services and add-ons instead of creating anything")
	hiddenArchived := flag.Bool("hidden-as-archived", false, "also import the services hidden on the old site, already archived so they stay off booking")
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
	for _, m := range ex.Meta {
		if m.Name == "show_service_on_site" && m.Value == "true" {
			visible[m.ServiceID] = true
		}
	}
	type item struct{ src, name, parent string }
	var mains, addons []item
	hidden := 0
	var hiddenNames []string
	for _, s := range ex.Services {
		if visible[s.ID] {
			mains = append(mains, item{src: s.ID, name: strings.TrimSpace(s.Name)})
		} else {
			hidden++
			hiddenNames = append(hiddenNames, strings.TrimSpace(s.Name))
		}
	}
	skippedExtras := 0
	for _, e := range ex.Extras {
		if visible[e.ServiceID] {
			addons = append(addons, item{name: strings.TrimSpace(e.Name), parent: e.ServiceID})
		} else {
			skippedExtras++
		}
	}
	fmt.Printf("Export: %d visible services (+%d hidden skipped), %d add-ons (+%d belonging to hidden services skipped)\n", len(mains), hidden, len(addons), skippedExtras)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	dsn := os.Getenv("DAPPER_DATABASE_URL")
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal("database configuration unavailable")
	}
	defer pool.Close()
	var actor, org, loc, locName string
	err = pool.QueryRow(ctx, `SELECT c.principal_id::text,m.organization_id::text,l.id::text,l.name FROM password_credentials c
 JOIN organization_memberships m ON m.principal_id=c.principal_id JOIN locations l ON l.organization_id=m.organization_id AND upper(l.code)=upper($2)
 WHERE c.email=lower($1)`, *email, *code).Scan(&actor, &org, &loc, &locName)
	if err != nil {
		log.Fatal("administrator or franchise code not found")
	}
	store, err := postgres.Open(ctx, dsn)
	if err != nil {
		log.Fatal("database unavailable")
	}
	defer store.Close()
	// Read straight from the table (no list limit) so a re-run recognises everything it created before.
	central := map[string]postgres.CentralService{} // key: parent|name
	crows, err := pool.Query(ctx, `SELECT id::text,name,kind,parent_id::text,version,price_cents FROM central_catalog_services WHERE organization_id=$1 AND NOT archived`, org)
	if err != nil {
		log.Fatal("cannot read the central catalogue")
	}
	existing := 0
	for crows.Next() {
		var c postgres.CentralService
		if err = crows.Scan(&c.ID, &c.Name, &c.Kind, &c.ParentID, &c.Version, &c.PriceCents); err != nil {
			log.Fatal("cannot read the central catalogue")
		}
		p := ""
		if c.ParentID != nil {
			p = *c.ParentID
		}
		central[p+"|"+strings.ToLower(c.Name)] = c
		existing++
	}
	crows.Close()
	newMains, newAddons := 0, 0
	byCentralMain := map[string]string{} // bookingpress service id -> central id
	for _, m := range mains {
		if c, ok := central["|"+strings.ToLower(m.name)]; ok {
			byCentralMain[m.src] = c.ID
			continue
		}
		newMains++
	}
	for _, a := range addons {
		if parent, ok := byCentralMain[a.parent]; ok {
			if _, ok = central[parent+"|"+strings.ToLower(a.name)]; ok {
				continue
			}
		}
		newAddons++
	}
	if *prices {
		setPrices(ctx, store, actor, org, ex, visible, central, byCentralMain, *apply)
		return
	}
	fmt.Printf("Central catalogue: %d existing active, would create %d services and %d add-ons\n", existing, newMains, newAddons)
	// Hidden services: imported already archived (a name that exists at all, archived or not, is left alone).
	var hiddenToAdd []string
	if *hiddenArchived {
		for _, n := range hiddenNames {
			var found bool
			if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM central_catalog_services WHERE organization_id=$1 AND lower(name)=lower($2) AND parent_id IS NULL)`, org, n).Scan(&found); err != nil {
				log.Fatal("cannot read the central catalogue")
			}
			if !found {
				hiddenToAdd = append(hiddenToAdd, n)
			}
		}
		fmt.Printf("Hidden services: would import %d as archived (%s)\n", len(hiddenToAdd), strings.Join(hiddenToAdd, "; "))
	}
	if !*apply {
		fmt.Printf("Preview only. Re-run with -apply to create them and enable them at %s.\n", locName)
		return
	}
	for _, m := range mains {
		if _, ok := byCentralMain[m.src]; ok {
			continue
		}
		c, err := store.SaveCentralService(ctx, actor, org, postgres.CentralServiceInput{Name: m.name, Kind: "main"})
		if err != nil {
			log.Fatalf("create %q failed: %v", m.name, err)
		}
		byCentralMain[m.src] = c.ID
		central["|"+strings.ToLower(m.name)] = c
	}
	for _, a := range addons {
		parent := byCentralMain[a.parent]
		if _, ok := central[parent+"|"+strings.ToLower(a.name)]; ok {
			continue
		}
		p := parent
		c, err := store.SaveCentralService(ctx, actor, org, postgres.CentralServiceInput{Name: a.name, Kind: "addon", ParentID: &p})
		if err != nil {
			log.Fatalf("create add-on %q failed: %v", a.name, err)
		}
		central[parent+"|"+strings.ToLower(a.name)] = c
	}
	// Enable at the franchise: mains first, then add-ons (an add-on needs its main enabled).
	linked := map[string]bool{}
	rows, err := pool.Query(ctx, `SELECT central_service_id::text FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND central_service_id IS NOT NULL AND NOT archived`, org, loc)
	if err != nil {
		log.Fatal("cannot read franchise catalogue")
	}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err == nil {
			linked[id] = true
		}
	}
	rows.Close()
	scope := postgres.Scope{ActorID: actor, OrganizationID: org, LocationID: loc}
	all := make([]postgres.CentralService, 0, len(central))
	for _, c := range central {
		all = append(all, c)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Kind < all[j].Kind }) // "addon" < "main": reverse below
	enabled := 0
	for _, kind := range []string{"main", "addon"} {
		for _, c := range all {
			if c.Kind != kind || linked[c.ID] {
				continue
			}
			if _, err := store.ImportCentralService(ctx, scope, c.ID, c.Version); err != nil {
				log.Fatalf("enable %q at %s failed: %v", c.Name, locName, err)
			}
			enabled++
		}
	}
	archived := 0
	for _, n := range hiddenToAdd {
		c, err := store.SaveCentralService(ctx, actor, org, postgres.CentralServiceInput{Name: n, Kind: "main"})
		if err != nil {
			log.Fatalf("create hidden %q failed: %v", n, err)
		}
		svc, err := store.ImportCentralService(ctx, scope, c.ID, c.Version)
		if err != nil {
			log.Fatalf("enable hidden %q failed: %v", n, err)
		}
		if _, err = store.ArchiveService(ctx, scope, svc.ID, svc.Version); err != nil {
			log.Fatalf("archive hidden %q at %s failed: %v", n, locName, err)
		}
		if _, err = store.ArchiveCentralService(ctx, actor, org, c.ID, c.Version); err != nil {
			log.Fatalf("archive hidden %q centrally failed: %v", n, err)
		}
		archived++
	}
	fmt.Printf("Done. Created %d central services and %d add-ons; enabled %d at %s; imported %d hidden services as archived.\n", newMains, newAddons, enabled, locName, archived)
}

// cents turns a BookingPress price such as "149.99" or "150" into whole cents; ok is false for anything else.
func cents(v string) (int64, bool) {
	v = strings.TrimSpace(v)
	whole, frac, _ := strings.Cut(v, ".")
	if whole == "" || len(frac) > 2 {
		return 0, false
	}
	for _, r := range whole + frac {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	for len(frac) < 2 {
		frac += "0"
	}
	var w, f int64
	fmt.Sscan(whole, &w)
	fmt.Sscan(frac, &f)
	return w*100 + f, true
}

// setPrices writes the export's prices (CAD, before tax) on the matching central services; linked franchise services follow through
// the central revision. Only price changes are sent, so the report lists exactly what differs.
func setPrices(ctx context.Context, store *postgres.Store, actor, org string, ex export, visible map[string]bool, central map[string]postgres.CentralService, byMain map[string]string, apply bool) {
	type change struct {
		label string
		c     postgres.CentralService
		price int64
	}
	var todo []change
	var missing, same int
	add := func(label string, c postgres.CentralService, ok bool, raw string) {
		p, valid := cents(raw)
		if !ok || !valid {
			missing++
			fmt.Printf("  no price set: %s (%s)\n", label, map[bool]string{true: "unusable price " + raw, false: "not in the active central catalogue"}[ok])
			return
		}
		if c.PriceCents != nil && *c.PriceCents == p {
			same++
			return
		}
		todo = append(todo, change{label, c, p})
	}
	for _, sv := range ex.Services {
		if !visible[sv.ID] {
			fmt.Printf("  hidden on the old site, archived here, price not set: %s (%s)\n", strings.TrimSpace(sv.Name), sv.Price)
			continue
		}
		c, ok := central["|"+strings.ToLower(strings.TrimSpace(sv.Name))]
		add(strings.TrimSpace(sv.Name), c, ok, sv.Price)
	}
	for _, e := range ex.Extras {
		if !visible[e.ServiceID] {
			continue
		}
		c, ok := central[byMain[e.ServiceID]+"|"+strings.ToLower(strings.TrimSpace(e.Name))]
		add("add-on "+strings.TrimSpace(e.Name), c, ok, e.Price)
	}
	for _, t := range todo {
		fmt.Printf("  %s: %d.%02d\n", t.label, t.price/100, t.price%100)
	}
	fmt.Printf("Prices: %d to set, %d already correct, %d without a usable match\n", len(todo), same, missing)
	if !apply {
		fmt.Println("Preview only. Re-run with -apply to write them.")
		return
	}
	for _, t := range todo {
		p := t.price
		if _, err := store.SaveCentralService(ctx, actor, org, postgres.CentralServiceInput{ID: t.c.ID, Name: t.c.Name, Kind: t.c.Kind, ParentID: t.c.ParentID, Version: t.c.Version, PriceSet: true, PriceCents: &p}); err != nil {
			log.Fatalf("price for %q failed: %v", t.label, err)
		}
	}
	fmt.Printf("Done. Set %d prices.\n", len(todo))
}

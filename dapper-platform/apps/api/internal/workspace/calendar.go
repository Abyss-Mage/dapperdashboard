package workspace

import (
	"dapper.local/platform/internal/postgres"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func escapeICalText(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `;`, `\;`)
	s = strings.ReplaceAll(s, `,`, `\,`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

func formatICalTime(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}

func registerCalendarRoutes(mux *http.ServeMux, store *postgres.Store) {
	route := "/api/v1/organizations/{organization}/locations/{location}/calendar.ics"

	mux.HandleFunc("GET "+route, func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return
		}

		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "bookings.read")
		if err != nil {
			failure(w, err)
			return
		}

		// Retrieve bookings from past 7 days to next 60 days
		now := time.Now().UTC()
		from := now.Add(-7 * 24 * time.Hour)
		to := now.Add(60 * 24 * time.Hour)

		bookings, err := store.ListBookings(r.Context(), scope, from, to, "", 200)
		if err != nil {
			bookingFailure(w, err)
			return
		}

		var sb strings.Builder
		sb.WriteString("BEGIN:VCALENDAR\r\n")
		sb.WriteString("VERSION:2.0\r\n")
		sb.WriteString("PRODID:-//Dapper Detailing//Appointment Calendar//EN\r\n")
		sb.WriteString("CALSCALE:GREGORIAN\r\n")
		sb.WriteString("METHOD:PUBLISH\r\n")
		sb.WriteString(fmt.Sprintf("X-WR-CALNAME:Dapper Detailing (%s)\r\n", loc[:8]))
		sb.WriteString("X-WR-TIMEZONE:UTC\r\n")

		for _, b := range bookings {
			// Skip cancelled appointments from active calendar feed
			if b.Status == "cancelled" {
				continue
			}

			start := b.BookingTime
			// Default 2-hour window if duration not specified
			end := start.Add(2 * time.Hour)

			summary := fmt.Sprintf("%s - %s", b.ServiceName, b.CustomerName)
			if b.CustomerName == "" {
				summary = b.ServiceName
			}

			descParts := []string{
				fmt.Sprintf("Booking ID: %s", b.ID),
				fmt.Sprintf("Service: %s", b.ServiceName),
				fmt.Sprintf("Status: %s", b.Status),
				fmt.Sprintf("Channel: %s", b.Channel),
			}
			if len(b.Addons) > 0 {
				var addonNames []string
				for _, a := range b.Addons {
					addonNames = append(addonNames, a.Name)
				}
				descParts = append(descParts, fmt.Sprintf("Add-ons: %s", strings.Join(addonNames, ", ")))
			}
			if b.Notes != "" {
				descParts = append(descParts, fmt.Sprintf("Notes: %s", b.Notes))
			}

			sb.WriteString("BEGIN:VEVENT\r\n")
			sb.WriteString(fmt.Sprintf("UID:booking-%s@dapper.local\r\n", b.ID))
			sb.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", formatICalTime(now)))
			sb.WriteString(fmt.Sprintf("DTSTART:%s\r\n", formatICalTime(start)))
			sb.WriteString(fmt.Sprintf("DTEND:%s\r\n", formatICalTime(end)))
			sb.WriteString(fmt.Sprintf("SUMMARY:%s\r\n", escapeICalText(summary)))
			sb.WriteString(fmt.Sprintf("DESCRIPTION:%s\r\n", escapeICalText(strings.Join(descParts, "\n"))))
			sb.WriteString("STATUS:CONFIRMED\r\n")
			sb.WriteString("END:VEVENT\r\n")
		}

		sb.WriteString("END:VCALENDAR\r\n")

		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.Header().Set("Content-Disposition", "inline; filename=\"dapper-calendar.ics\"")
		w.Header().Set("Cache-Control", "no-cache, no-store, max-age=0, must-revalidate")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sb.String()))
	})
}

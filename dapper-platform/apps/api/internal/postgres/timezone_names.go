package postgres

// legacyZones maps the older time-zone names some browsers still report to the current names. Go knows both, but a database
// without the legacy tz files (the default on a minimal Ubuntu) rejects the old ones, which broke the overview charts for
// people whose browser says Asia/Calcutta.
var legacyZones = map[string]string{
	"Asia/Calcutta":        "Asia/Kolkata",
	"Asia/Katmandu":        "Asia/Kathmandu",
	"Asia/Rangoon":         "Asia/Yangon",
	"Asia/Saigon":          "Asia/Ho_Chi_Minh",
	"Asia/Dacca":           "Asia/Dhaka",
	"Asia/Thimbu":          "Asia/Thimphu",
	"Asia/Ulan_Bator":      "Asia/Ulaanbaatar",
	"Asia/Macao":           "Asia/Macau",
	"Asia/Istanbul":        "Europe/Istanbul",
	"Europe/Kiev":          "Europe/Kyiv",
	"America/Buenos_Aires": "America/Argentina/Buenos_Aires",
	"America/Indianapolis": "America/Indiana/Indianapolis",
	"America/Louisville":   "America/Kentucky/Louisville",
	"Atlantic/Faeroe":      "Atlantic/Faroe",
	"Pacific/Samoa":        "Pacific/Pago_Pago",
	"Pacific/Truk":         "Pacific/Chuuk",
	"Pacific/Ponape":       "Pacific/Pohnpei",
	"US/Eastern":           "America/New_York",
	"US/Central":           "America/Chicago",
	"US/Mountain":          "America/Denver",
	"US/Pacific":           "America/Los_Angeles",
	"Canada/Eastern":       "America/Toronto",
	"Canada/Central":       "America/Winnipeg",
	"Canada/Mountain":      "America/Edmonton",
	"Canada/Pacific":       "America/Vancouver",
}

// canonicalZone returns the current name for a legacy time-zone name and leaves any other name as it was.
func canonicalZone(tz string) string {
	if c, ok := legacyZones[tz]; ok {
		return c
	}
	return tz
}

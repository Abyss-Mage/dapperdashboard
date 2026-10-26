package main

import (
	_ "embed"
	"encoding/json"
)

// Source-derived public facts only. No published label establishes an approved
// package/category mapping, currency, tax rule, duration or checkout price.
//
//go:embed data/website-reference.json
var referenceCatalogue json.RawMessage

//go:embed data/bookingpress-extras-reference.json
var bookingpressCatalogue json.RawMessage

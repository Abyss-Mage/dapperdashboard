package workspace

import (
	"dapper.local/platform/internal/postgres"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func verifySchedulingRulesHTTP(t *testing.T, send func(string, string, string, string, string) *httptest.ResponseRecorder, base, resource, token, origin string) {
	t.Helper()
	mainResponse := send("POST", base+"services", `{"name":"Synthetic HTTP scheduling main","kind":"main"}`, token, origin)
	var main postgres.CatalogService
	if mainResponse.Code != 201 || json.Unmarshal(mainResponse.Body.Bytes(), &main) != nil {
		t.Fatal("rule main fixture failed")
	}
	addonResponse := send("POST", base+"services", `{"name":"Synthetic HTTP scheduling add-on","kind":"addon","parentId":"`+main.ID+`"}`, token, origin)
	var addon postgres.CatalogService
	if addonResponse.Code != 201 || json.Unmarshal(addonResponse.Body.Bytes(), &addon) != nil {
		t.Fatal("rule addon fixture failed")
	}
	path := base + "scheduling/resources/" + resource + "/rules"
	payload := `{"serviceId":"` + main.ID + `","addonIds":["` + addon.ID + `"],"durationMinutes":60,"beforeMinutes":0,"afterMinutes":15,"units":1,"slots":2}`
	if send("GET", path, "", "", origin).Code != 401 {
		t.Fatal("anonymous rule list accepted")
	}
	if send("POST", path, payload, token, "https://untrusted.example").Code != 403 {
		t.Fatal("rule origin bypass")
	}
	if send("POST", path, strings.Replace(payload, `,"beforeMinutes":0`, "", 1), token, origin).Code != 422 {
		t.Fatal("missing buffer silently defaulted")
	}
	if send("POST", path, strings.TrimSuffix(payload, "}")+`,"actorId":"forged"}`, token, origin).Code != 400 {
		t.Fatal("rule actor injection accepted")
	}
	response := send("POST", path, payload, token, origin)
	var rule postgres.SchedulingRule
	if response.Code != 201 || json.Unmarshal(response.Body.Bytes(), &rule) != nil || rule.Version != 1 {
		t.Fatal("HTTP rule create failed", response.Code)
	}
	if send("POST", path, payload, token, origin).Code != 409 {
		t.Fatal("HTTP duplicate rule accepted")
	}
	if listed := send("GET", path, "", token, origin); listed.Code != 200 || !strings.Contains(listed.Body.String(), rule.ID) {
		t.Fatal("rule list missing")
	}
	edited := strings.TrimSuffix(strings.Replace(payload, `"durationMinutes":60`, `"durationMinutes":90`, 1), "}") + `,"version":1}`
	response = send("PUT", path+"/"+rule.ID, edited, token, origin)
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"version":2`) {
		t.Fatal("rule edit failed", response.Code)
	}
	if send("PUT", path+"/"+rule.ID, edited, token, origin).Code != 409 {
		t.Fatal("stale HTTP rule edit accepted")
	}
	if send("PUT", path+"/"+rule.ID, strings.Replace(edited, `"version":1`, `"version":2`, 1), token, origin).Code != 200 {
		t.Fatal("current rule version denied")
	}
	reservePath := path + "/" + rule.ID + "/reservations"
	if send("POST", reservePath, `{"start":"2026-11-01T03:00:00-05:00","ruleVersion":2}`, token, origin).Code != 409 {
		t.Fatal("stale rule reservation accepted")
	}
	if send("POST", reservePath, `{"start":"2026-11-01T03:00:00-05:00","ruleVersion":3,"durationMinutes":1}`, token, origin).Code != 400 {
		t.Fatal("caller timing override accepted")
	}
	reserved := send("POST", reservePath, `{"start":"2026-11-01T03:00:00-05:00","ruleVersion":3}`, token, origin)
	var receipt struct {
		ID      string                   `json:"id"`
		Minutes int                      `json:"durationMinutes"`
		Rule    *postgres.SchedulingRule `json:"schedulingRule"`
	}
	if reserved.Code != 201 || json.Unmarshal(reserved.Body.Bytes(), &receipt) != nil || receipt.Minutes != 90 || receipt.Rule == nil || receipt.Rule.Version != 3 {
		t.Fatal("server rule reservation receipt failed", reserved.Code)
	}
	if send("POST", base+"scheduling/resources/"+resource+"/reservations/"+receipt.ID+"/reschedule", `{"start":"2026-11-01T05:00:00-05:00","durationMinutes":1,"beforeMinutes":0,"afterMinutes":0,"units":1,"version":1}`, token, origin).Code != 422 {
		t.Fatal("generic timing bypass accepted")
	}
	movePath := reservePath + "/" + receipt.ID + "/reschedule"
	moveBody := `{"start":"2026-11-01T05:00:00-05:00","ruleVersion":3,"version":1}`
	if send("POST", movePath, strings.TrimSuffix(moveBody, "}")+`,"durationMinutes":1}`, token, origin).Code != 400 {
		t.Fatal("rule move timing override accepted")
	}
	if send("POST", movePath, moveBody, token, origin).Code != 200 {
		t.Fatal("HTTP rule-aware move failed")
	}
	if send("POST", movePath, moveBody, token, origin).Code != 409 {
		t.Fatal("stale HTTP reservation move accepted")
	}
	if send("POST", base+"scheduling/resources/"+resource+"/reservations/"+receipt.ID+"/release", `{"version":2}`, token, origin).Code != 200 {
		t.Fatal("rule reservation release failed")
	}
}

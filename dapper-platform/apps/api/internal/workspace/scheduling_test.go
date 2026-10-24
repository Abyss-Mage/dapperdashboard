package workspace

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func verifySchedulingHTTP(t *testing.T, send func(string, string, string, string, string) *httptest.ResponseRecorder, base, foreignBase, token, origin string) {
	t.Helper()
	path := base + "scheduling/resources"
	if send("GET", path, "", "", origin).Code != 401 {
		t.Fatal("anonymous scheduling access")
	}
	if send("GET", foreignBase+"scheduling/resources", "", token, origin).Code != 403 {
		t.Fatal("foreign resource access")
	}
	resourceBody := `{"name":"Synthetic HTTP bay","timezone":"America/Toronto","capacity":1,"open":[{"start":"2026-11-01T00:00:00-04:00","end":"2026-11-01T08:00:00-05:00"}],"closed":[]}`
	if send("POST", path, resourceBody, token, "https://untrusted.example").Code != 403 {
		t.Fatal("resource write accepted wrong origin")
	}
	if send("POST", path, strings.TrimSuffix(resourceBody, "}")+`,"actorId":"forged"}`, token, origin).Code != 400 {
		t.Fatal("resource actor injection accepted")
	}
	created := send("POST", path, resourceBody, token, origin)
	var resource struct {
		ID string `json:"id"`
	}
	if created.Code != 201 || json.Unmarshal(created.Body.Bytes(), &resource) != nil || resource.ID == "" {
		t.Fatal("resource create failed", created.Code)
	}
	if listed := send("GET", path, "", token, origin); listed.Code != 200 || !strings.Contains(listed.Body.String(), resource.ID) {
		t.Fatal("resource list missing")
	}
	verifySchedulingRulesHTTP(t, send, base, resource.ID, token, origin)
	reservations := path + "/" + resource.ID + "/reservations"
	payload := `{"start":"2026-11-01T01:00:00-04:00","durationMinutes":60,"beforeMinutes":0,"afterMinutes":0,"units":1}`
	if send("POST", reservations, `{"start":"2026-11-01T01:00:00-04:00","durationMinutes":60,"units":1}`, token, origin).Code != 422 {
		t.Fatal("omitted buffers silently defaulted")
	}
	if send("POST", reservations, strings.TrimSuffix(payload, "}")+`,"bookingConfirmed":true}`, token, origin).Code != 400 {
		t.Fatal("confirmation injection accepted")
	}
	reserved := send("POST", reservations, payload, token, origin)
	var reservation struct {
		ID        string `json:"id"`
		Version   int    `json:"version"`
		Confirmed bool   `json:"bookingConfirmed"`
	}
	if reserved.Code != 201 || json.Unmarshal(reserved.Body.Bytes(), &reservation) != nil || reservation.Confirmed || reservation.Version != 1 {
		t.Fatal("technical reservation failed", reserved.Code)
	}
	if send("POST", reservations, payload, token, origin).Code != 409 {
		t.Fatal("overlap accepted")
	}
	movedPayload := strings.Replace(payload, "01:00:00-04:00", "01:00:00-05:00", 1)
	movedPayload = strings.TrimSuffix(movedPayload, "}") + `,"version":1}`
	if send("POST", reservations+"/"+reservation.ID+"/reschedule", movedPayload, token, origin).Code != 200 {
		t.Fatal("adjacent repeated-clock move failed")
	}
	if send("POST", reservations+"/"+reservation.ID+"/reschedule", movedPayload, token, origin).Code != 409 {
		t.Fatal("stale HTTP move accepted")
	}
	if send("POST", reservations+"/"+reservation.ID+"/release", `{"version":1}`, token, origin).Code != 409 {
		t.Fatal("stale release accepted")
	}
	released := send("POST", reservations+"/"+reservation.ID+"/release", `{"version":2}`, token, origin)
	if released.Code != 200 || !strings.Contains(released.Body.String(), `"cancelledBy":"`) {
		t.Fatal("release receipt missing", released.Code)
	}
	if send("POST", reservations+"/"+reservation.ID+"/release", `{"version":3}`, token, origin).Code != 409 {
		t.Fatal("duplicate release accepted")
	}
	if send("POST", reservations, payload, token, origin).Code != 201 {
		t.Fatal("released capacity not reusable")
	}
}

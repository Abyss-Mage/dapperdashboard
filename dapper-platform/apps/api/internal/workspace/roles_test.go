package workspace

import (
	"context"
	"dapper.local/platform/internal/postgres"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func verifyRolesHTTP(t *testing.T, ctx context.Context, store *postgres.Store, setup *pgxpool.Pool, handler http.Handler, actor, org, loc, token, origin string) {
	t.Helper()

	base := "/api/v1/organizations/" + org

	send := func(method, path string, input any, bearer string) *httptest.ResponseRecorder {
		var body []byte
		if input != nil {
			body, _ = json.Marshal(input)
		}
		r := httptest.NewRequest(method, path, strings.NewReader(string(body)))
		r.Host = "127.0.0.1:8121"
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}

	// 1. GET Capabilities Catalogue
	capResp := send("GET", base+"/capabilities", nil, token)
	if capResp.Code != 200 {
		t.Fatalf("expected 200 for capabilities catalogue, got %d: %s", capResp.Code, capResp.Body.String())
	}
	var cat postgres.CapabilityCatalogue
	if err := json.Unmarshal(capResp.Body.Bytes(), &cat); err != nil {
		t.Fatalf("failed to decode catalogue: %v", err)
	}
	if len(cat.LocationCapabilities) == 0 || len(cat.OrganizationCapabilities) == 0 {
		t.Fatalf("catalogue is empty: %+v", cat)
	}

	// 2. Ensure actor has roles.manage and roles.assign grants
	for _, action := range []string{"roles.manage", "roles.assign"} {
		if _, err := setup.Exec(ctx, `INSERT INTO organization_grants(principal_id,organization_id,action) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, actor, org, action); err != nil {
			t.Fatal(err)
		}
	}

	// 3. Reject invalid capability in role creation
	badRoleReq := map[string]any{
		"name":         "Bad Role",
		"description":  "Invalid",
		"capabilities": []string{"unknown.fake.action"},
	}
	if send("POST", base+"/roles", badRoleReq, token).Code != 422 {
		t.Fatal("expected 422 for role with invalid capability")
	}

	// 4. Create valid role
	createReq := map[string]any{
		"name":         "Detailing Specialist",
		"description":  "Core detailing technician",
		"capabilities": []string{"jobs.read", "jobs.contribute", "attendance.record"},
	}
	createResp := send("POST", base+"/roles", createReq, token)
	if createResp.Code != 201 {
		t.Fatalf("expected 201 for create role, got %d: %s", createResp.Code, createResp.Body.String())
	}
	var createdRole postgres.Role
	if err := json.Unmarshal(createResp.Body.Bytes(), &createdRole); err != nil {
		t.Fatalf("failed to decode created role: %v", err)
	}
	if createdRole.Name != "Detailing Specialist" || createdRole.Version != 1 {
		t.Fatalf("unexpected created role details: %+v", createdRole)
	}

	// 5. GET single role
	getResp := send("GET", base+"/roles/"+createdRole.ID, nil, token)
	if getResp.Code != 200 {
		t.Fatalf("expected 200 for get role, got %d: %s", getResp.Code, getResp.Body.String())
	}

	// 6. Update role with optimistic concurrency
	staleUpdate := map[string]any{
		"name":         "Detailing Specialist II",
		"description":  "Updated",
		"capabilities": []string{"jobs.read", "jobs.contribute", "jobs.complete"},
		"version":      99,
	}
	if send("PUT", base+"/roles/"+createdRole.ID, staleUpdate, token).Code != 409 {
		t.Fatal("expected 409 for stale role version")
	}

	validUpdate := map[string]any{
		"name":         "Detailing Specialist II",
		"description":  "Updated",
		"capabilities": []string{"jobs.read", "jobs.contribute", "jobs.complete"},
		"version":      1,
	}
	updateResp := send("PUT", base+"/roles/"+createdRole.ID, validUpdate, token)
	if updateResp.Code != 200 {
		t.Fatalf("expected 200 for update role, got %d: %s", updateResp.Code, updateResp.Body.String())
	}
	var updatedRole postgres.Role
	_ = json.Unmarshal(updateResp.Body.Bytes(), &updatedRole)
	if updatedRole.Version != 2 || updatedRole.Name != "Detailing Specialist II" {
		t.Fatalf("unexpected updated role: %+v", updatedRole)
	}

	// 7. Assign role to actor at location scope
	assignReq := map[string]any{
		"principalId": actor,
		"roleId":      createdRole.ID,
		"scopeType":   "location",
		"scopeId":     loc,
	}
	assignResp := send("POST", base+"/role-assignments", assignReq, token)
	if assignResp.Code != 201 {
		t.Fatalf("expected 201 for assign role, got %d: %s", assignResp.Code, assignResp.Body.String())
	}
	var assignment postgres.MemberRoleAssignment
	if err := json.Unmarshal(assignResp.Body.Bytes(), &assignment); err != nil {
		t.Fatalf("failed to decode assignment: %v", err)
	}
	if assignment.RoleID != createdRole.ID || assignment.ScopeID != loc {
		t.Fatalf("unexpected assignment: %+v", assignment)
	}

	// 8. Duplicate assignment returns 409
	if send("POST", base+"/role-assignments", assignReq, token).Code != 409 {
		t.Fatal("expected 409 for duplicate role assignment")
	}

	// 9. List role assignments
	listAssignResp := send("GET", base+"/role-assignments", nil, token)
	if listAssignResp.Code != 200 {
		t.Fatalf("expected 200 for list role assignments, got %d: %s", listAssignResp.Code, listAssignResp.Body.String())
	}
	var assignments []postgres.MemberRoleAssignment
	_ = json.Unmarshal(listAssignResp.Body.Bytes(), &assignments)
	if len(assignments) == 0 {
		t.Fatal("expected non-empty role assignments list")
	}

	// 10. Attempt to delete in-use role returns 409
	if send("DELETE", base+"/roles/"+createdRole.ID, nil, token).Code != 409 {
		t.Fatal("expected 409 when deleting in-use role")
	}

	// 11. Revoke assignment with stale version returns 409
	if send("DELETE", base+"/role-assignments/"+assignment.ID+"?version=99", nil, token).Code != 409 {
		t.Fatal("expected 409 for stale role assignment version on revoke")
	}

	// Revoke assignment with matching version returns 204
	revokeResp := send("DELETE", base+"/role-assignments/"+assignment.ID+"?version=1", nil, token)
	if revokeResp.Code != 204 {
		t.Fatalf("expected 204 for revoke assignment, got %d: %s", revokeResp.Code, revokeResp.Body.String())
	}

	// 12. Delete role now succeeds returns 204
	deleteResp := send("DELETE", base+"/roles/"+createdRole.ID, nil, token)
	if deleteResp.Code != 204 {
		t.Fatalf("expected 204 for delete role, got %d: %s", deleteResp.Code, deleteResp.Body.String())
	}
}

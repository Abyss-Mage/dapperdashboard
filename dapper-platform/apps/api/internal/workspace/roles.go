package workspace

import (
	"dapper.local/platform/internal/postgres"
	"errors"
	"net/http"
	"strconv"
)

func registerRoleRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}"

	// GET capabilities catalogue
	mux.HandleFunc("GET "+base+"/capabilities", func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return
		}
		reply(w, 200, postgres.GetCapabilityCatalogue())
	})

	// GET list of roles
	mux.HandleFunc("GET "+base+"/roles", func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return
		}
		roles, err := store.ListRoles(r.Context(), requestSession(r).principalID, org)
		if err != nil {
			handleRoleError(w, err)
			return
		}
		reply(w, 200, roles)
	})

	// POST create role
	mux.HandleFunc("POST "+base+"/roles", func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return
		}
		var input struct {
			Name         string   `json:"name"`
			Description  string   `json:"description"`
			Capabilities []string `json:"capabilities"`
		}
		if !decode(w, r, &input) {
			return
		}
		role, err := store.CreateRole(r.Context(), requestSession(r).principalID, org, input.Name, input.Description, input.Capabilities)
		if err != nil {
			handleRoleError(w, err)
			return
		}
		reply(w, 201, role)
	})

	// GET single role
	mux.HandleFunc("GET "+base+"/roles/{role}", func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		roleID := r.PathValue("role")
		if !uuid(org) || !uuid(roleID) {
			reply(w, 400, map[string]string{"error": "Valid organization and role selectors required"})
			return
		}
		role, err := store.GetRole(r.Context(), requestSession(r).principalID, org, roleID)
		if err != nil {
			handleRoleError(w, err)
			return
		}
		reply(w, 200, role)
	})

	// PUT update role
	mux.HandleFunc("PUT "+base+"/roles/{role}", func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		roleID := r.PathValue("role")
		if !uuid(org) || !uuid(roleID) {
			reply(w, 400, map[string]string{"error": "Valid organization and role selectors required"})
			return
		}
		var input struct {
			Name         string   `json:"name"`
			Description  string   `json:"description"`
			Capabilities []string `json:"capabilities"`
			Version      int      `json:"version"`
		}
		if !decode(w, r, &input) {
			return
		}
		role, err := store.UpdateRole(r.Context(), requestSession(r).principalID, org, roleID, input.Name, input.Description, input.Capabilities, input.Version)
		if err != nil {
			handleRoleError(w, err)
			return
		}
		reply(w, 200, role)
	})

	// DELETE role
	mux.HandleFunc("DELETE "+base+"/roles/{role}", func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		roleID := r.PathValue("role")
		if !uuid(org) || !uuid(roleID) {
			reply(w, 400, map[string]string{"error": "Valid organization and role selectors required"})
			return
		}
		version := 0
		if vStr := r.URL.Query().Get("version"); vStr != "" {
			if v, err := strconv.Atoi(vStr); err == nil && v > 0 {
				version = v
			}
		}
		err := store.DeleteRole(r.Context(), requestSession(r).principalID, org, roleID, version)
		if err != nil {
			handleRoleError(w, err)
			return
		}
		reply(w, 204, nil)
	})

	// GET role assignments
	mux.HandleFunc("GET "+base+"/role-assignments", func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return
		}
		assignments, err := store.ListRoleAssignments(r.Context(), requestSession(r).principalID, org)
		if err != nil {
			handleRoleError(w, err)
			return
		}
		reply(w, 200, assignments)
	})

	// POST assign role
	mux.HandleFunc("POST "+base+"/role-assignments", func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return
		}
		var input struct {
			PrincipalID string `json:"principalId"`
			RoleID      string `json:"roleId"`
			ScopeType   string `json:"scopeType"`
			ScopeID     string `json:"scopeId"`
		}
		if !decode(w, r, &input) {
			return
		}
		if !uuid(input.PrincipalID) || !uuid(input.RoleID) || !uuid(input.ScopeID) {
			reply(w, 422, map[string]string{"error": "Valid principal, role, and scope selectors required"})
			return
		}
		assignment, err := store.AssignMemberRole(r.Context(), requestSession(r).principalID, org, input.PrincipalID, input.RoleID, input.ScopeType, input.ScopeID)
		if err != nil {
			handleRoleError(w, err)
			return
		}
		reply(w, 201, assignment)
	})

	// DELETE revoke assignment
	mux.HandleFunc("DELETE "+base+"/role-assignments/{assignment}", func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		assignmentID := r.PathValue("assignment")
		if !uuid(org) || !uuid(assignmentID) {
			reply(w, 400, map[string]string{"error": "Valid organization and assignment selectors required"})
			return
		}
		version := 0
		if vStr := r.URL.Query().Get("version"); vStr != "" {
			if v, err := strconv.Atoi(vStr); err == nil && v > 0 {
				version = v
			}
		}
		err := store.RevokeMemberRoleAssignment(r.Context(), requestSession(r).principalID, org, assignmentID, version)
		if err != nil {
			handleRoleError(w, err)
			return
		}
		reply(w, 204, nil)
	})
}

func handleRoleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, postgres.ErrForbidden) || errors.Is(err, postgres.ErrScope):
		reply(w, 403, map[string]string{"error": "Access denied for this organization or operation"})
	case errors.Is(err, postgres.ErrSelfEscalation):
		reply(w, 403, map[string]string{"error": "Cannot assign capabilities or roles exceeding actor authority"})
	case errors.Is(err, postgres.ErrRoleNotFound):
		reply(w, 404, map[string]string{"error": "Role not found"})
	case errors.Is(err, postgres.ErrAssignmentNotFound):
		reply(w, 404, map[string]string{"error": "Role assignment not found"})
	case errors.Is(err, postgres.ErrRoleNameConflict):
		reply(w, 409, map[string]string{"error": "A role with this name already exists in this organization"})
	case errors.Is(err, postgres.ErrRoleStaleVersion):
		reply(w, 409, map[string]string{"error": "Role version conflict; record was modified by another request"})
	case errors.Is(err, postgres.ErrRoleInUse):
		reply(w, 409, map[string]string{"error": "Cannot delete role with active member assignments"})
	case errors.Is(err, postgres.ErrAssignmentConflict):
		reply(w, 409, map[string]string{"error": "Member already has this role at the specified scope"})
	case errors.Is(err, postgres.ErrLastAdminLockout):
		reply(w, 409, map[string]string{"error": "Operation would revoke or remove the last active organization administrator"})
	case errors.Is(err, postgres.ErrInactiveMember):
		reply(w, 422, map[string]string{"error": "Cannot assign roles to an inactive member"})
	case errors.Is(err, postgres.ErrInvalidScope):
		reply(w, 422, map[string]string{"error": "Invalid scope target or scope type for this organization"})
	case errors.Is(err, postgres.ErrInvalidCapability):
		reply(w, 422, map[string]string{"error": err.Error()})
	default:
		failure(w, err)
	}
}

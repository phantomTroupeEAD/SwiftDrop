package transport

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/swiftdrop/core-api/internal/identity/application"
	"github.com/swiftdrop/core-api/internal/identity/domain"
)

// AdminHandler handles HTTP requests for user governance and administrative operations.
type AdminHandler struct {
	userService *application.UserService
}

// NewAdminHandler constructs an AdminHandler.
func NewAdminHandler(userService *application.UserService) *AdminHandler {
	return &AdminHandler{
		userService: userService,
	}
}

// ListUsers handles GET /api/v1/admin/users
// Supports query params: role (ADMIN, COURIER, etc.), limit (default 20), offset (default 0).
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	var roleFilter *domain.Role
	roleQuery := strings.TrimSpace(r.URL.Query().Get("role"))
	if roleQuery != "" {
		parsedRole, err := domain.ParseRole(roleQuery)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ROLE", "Unrecognized role filter parameter")
			return
		}
		roleFilter = &parsedRole
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			offset = val
		}
	}

	users, total, err := h.userService.ListUsers(r.Context(), roleFilter, limit, offset)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve user directory")
		return
	}

	userDTOs := make([]UserDTO, 0, len(users))
	for _, u := range users {
		userDTOs = append(userDTOs, toUserDTO(u))
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(SingleDataEnvelope{
		Data: map[string]interface{}{
			"items":  userDTOs,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	})
}

// UpdateRole handles PUT /api/v1/admin/users/{id}/role
// Promotes or reassigns a user's role and optional merchant tenant context.
func (h *AdminHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	targetUserID := strings.TrimSpace(r.PathValue("id"))
	if targetUserID == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_USER_ID", "Target user ID parameter is required")
		return
	}

	principal, ok := GetPrincipal(r.Context())
	if !ok || principal == nil {
		writeJSONError(w, http.StatusUnauthorized, "AUTH_UNAUTHORIZED", "Authentication required")
		return
	}

	var req UpdateUserRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST_PAYLOAD", "Malformed JSON body")
		return
	}

	if !domain.IsValidRole(req.Role) {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ROLE", "Target role is unrecognized or invalid")
		return
	}

	// If promoting to MERCHANT, require a tenant_id
	if req.Role == domain.RoleMerchant && (req.TenantID == nil || strings.TrimSpace(*req.TenantID) == "") {
		writeJSONError(w, http.StatusBadRequest, "TENANT_REQUIRED", "tenant_id is required when assigning the MERCHANT role")
		return
	}

	err := h.userService.UpdateUserRole(r.Context(), *principal, targetUserID, req.Role, req.TenantID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrUserNotFound):
			writeJSONError(w, http.StatusNotFound, "USER_NOT_FOUND", "Target user does not exist")
		case errors.Is(err, domain.ErrForbidden):
			writeJSONError(w, http.StatusForbidden, "AUTH_FORBIDDEN", "Only Administrators can modify user roles")
		default:
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update user role")
		}
		return
	}

	updatedUser, err := h.userService.GetProfile(r.Context(), targetUserID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load updated profile")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(SingleDataEnvelope{
		Data: toUserDTO(updatedUser),
	})
}

// UpdateStatus handles PUT /api/v1/admin/users/{id}/status
// Suspends or reactivates a user's account.
func (h *AdminHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	targetUserID := strings.TrimSpace(r.PathValue("id"))
	if targetUserID == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_USER_ID", "Target user ID parameter is required")
		return
	}

	principal, ok := GetPrincipal(r.Context())
	if !ok || principal == nil {
		writeJSONError(w, http.StatusUnauthorized, "AUTH_UNAUTHORIZED", "Authentication required")
		return
	}

	var req UpdateUserStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST_PAYLOAD", "Malformed JSON body")
		return
	}

	if req.Status != domain.StatusActive && req.Status != domain.StatusSuspended {
		writeJSONError(w, http.StatusBadRequest, "INVALID_STATUS", "Status must be ACTIVE or SUSPENDED")
		return
	}

	err := h.userService.UpdateUserStatus(r.Context(), *principal, targetUserID, req.Status)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrUserNotFound):
			writeJSONError(w, http.StatusNotFound, "USER_NOT_FOUND", "Target user does not exist")
		case errors.Is(err, domain.ErrForbidden):
			writeJSONError(w, http.StatusForbidden, "AUTH_FORBIDDEN", "Only Administrators can modify user status")
		default:
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update user status")
		}
		return
	}

	updatedUser, err := h.userService.GetProfile(r.Context(), targetUserID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load updated profile")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(SingleDataEnvelope{
		Data: toUserDTO(updatedUser),
	})
}

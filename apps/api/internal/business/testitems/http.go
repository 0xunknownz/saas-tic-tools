package testitems

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/0xunknownz/saas.starter/foundation/identity"
	"github.com/0xunknownz/saas.starter/foundation/plugin"
	apihttp "github.com/0xunknownz/saas.starter/foundation/http"
)

func RegisterHTTP(r plugin.Router, svc *Service, deps HTTPDeps) error {
	// This sample Product module declares its list endpoint as a licensed
	// feature: removing "advanced" from the licence stops the endpoint while
	// reads elsewhere stay available. Real Products declare their own features.
	apihttp.RequireFeature("/api/product-admin/v1/test-items", "advanced")
	r.Handle("GET /api/product-admin/v1/test-items", deps.Identity.RequirePlatform(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !deps.Identity.PlatformCan(req.Context(), PermissionRead) {
			writeError(w, http.StatusForbidden, "PERMISSION_DENIED")
			return
		}
		limit := 50
		if raw := req.URL.Query().Get("limit"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 || n > 100 {
				writeError(w, http.StatusBadRequest, "BAD_REQUEST")
				return
			}
			limit = n
		}
		items, total, err := svc.List(req.Context(), strings.TrimSpace(req.URL.Query().Get("q")), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
	})))
	r.Handle("GET /api/product-admin/v1/test-items/{id}", deps.Identity.RequirePlatform(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !deps.Identity.PlatformCan(req.Context(), PermissionRead) {
			writeError(w, http.StatusForbidden, "PERMISSION_DENIED")
			return
		}
		item, err := svc.Get(req.Context(), req.PathValue("id"))
		if err == ErrNotFound {
			writeError(w, http.StatusNotFound, "TEST_ITEM_NOT_FOUND")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL")
			return
		}
		writeJSON(w, http.StatusOK, item)
	})))
	r.Handle("PUT /api/product-admin/v1/test-items/{id}", deps.Identity.RequirePlatform(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !deps.Identity.PlatformCan(req.Context(), PermissionManage) {
			writeError(w, http.StatusForbidden, "PERMISSION_DENIED")
			return
		}
		var input UpdateInput
		if !deps.Identity.GuardJSON(w, req, &input) {
			return
		}
		principal := identity.PrincipalFrom(req.Context())
		if principal == nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED")
			return
		}
		item, err := svc.Update(req.Context(), "account:"+principal.AccountID, req.PathValue("id"), input)
		switch {
		case errors.Is(err, ErrInvalidInput):
			writeErrorMessage(w, http.StatusBadRequest, "TEST_ITEM_INVALID", err.Error())
		case errors.Is(err, ErrNotFound):
			writeError(w, http.StatusNotFound, "TEST_ITEM_NOT_FOUND")
		case errors.Is(err, ErrConflict):
			writeErrorMessage(w, http.StatusConflict, "REVISION_CONFLICT", err.Error())
		case err != nil:
			writeError(w, http.StatusInternalServerError, "INTERNAL")
		default:
			writeJSON(w, http.StatusOK, item)
		}
	})))
	return nil
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, code string) {
	writeErrorMessage(w, status, code, http.StatusText(status))
}
func writeErrorMessage(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}

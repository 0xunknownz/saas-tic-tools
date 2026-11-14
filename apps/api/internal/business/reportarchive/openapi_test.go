package reportarchive

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/0xunknownz/saas.starter/foundation/identity"
	"github.com/0xunknownz/saas.starter/foundation/tenant"
)

type contractGuard struct{ identity.PlatformGuard }

func (contractGuard) RequireTenant(next http.Handler) http.Handler           { return next }
func (contractGuard) RequirePlatform(next http.Handler) http.Handler         { return next }
func (contractGuard) GuardJSON(http.ResponseWriter, *http.Request, any) bool { return true }
func (contractGuard) PlatformCan(context.Context, string) bool               { return true }

type contractMembership struct{ tenant.Membership }

func (contractMembership) Role(context.Context, string, string) (string, error) {
	return tenant.RoleTenantAdmin, nil
}

type contractRouter struct{ patterns map[string]bool }

func (r *contractRouter) Handle(pattern string, _ http.Handler) { r.patterns[pattern] = true }

// The shipped OpenAPI contract must declare exactly the Product routes mounted by this plugin.
func TestReportArchiveRoutesMatchOpenAPI(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "openapi", "product-reportarchive.json"))
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		Paths map[string]map[string]struct {
			OperationID  string `json:"operationId"`
			Permission   string `json:"x-permission"`
			TenantScoped bool   `json:"x-tenant-scoped"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatal(err)
	}
	router := &contractRouter{patterns: map[string]bool{}}
	if err := RegisterHTTP(router, &Service{}, HTTPDeps{Identity: contractGuard{}, Tenant: contractMembership{}}); err != nil {
		t.Fatal(err)
	}
	operations := 0
	tenantRoutes := 0
	for path, methods := range contract.Paths {
		for method, operation := range methods {
			operations++
			pattern := strings.ToUpper(method) + " " + path
			if !router.patterns[pattern] {
				t.Errorf("contract declares route not mounted: %s", pattern)
			}
			if operation.TenantScoped {
				tenantRoutes++
				if operation.Permission != "tenant.data.read" && operation.Permission != "tenant.data.write" {
					t.Errorf("%s: tenant route permission %q", pattern, operation.Permission)
				}
			} else if operation.Permission != platformTemplatesPermission {
				t.Errorf("%s: platform permission %q", pattern, operation.Permission)
			}
		}
	}
	if len(contract.Paths) == 0 || operations != 25 {
		t.Fatalf("contract operations: got %d, want 25", operations)
	}
	if tenantRoutes != 22 {
		t.Fatalf("tenant-scoped operations: got %d, want 22", tenantRoutes)
	}
	if len(router.patterns) != operations {
		var mounted []string
		for pattern := range router.patterns {
			mounted = append(mounted, pattern)
		}
		sort.Strings(mounted)
		t.Fatalf("mounted routes (%d) do not match contract (%d): %v", len(router.patterns), operations, mounted)
	}
}

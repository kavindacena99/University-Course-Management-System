package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/gin-gonic/gin"
)

// This contract includes deliberately opt-in permissions, not just default role grants.
var permissionContract = []string{
	"users:read", "users:manage", "roles:manage", "sessions:read", "sessions:revoke", "account:unlock", "audit:read",
	"courses:manage", "offerings:manage", "enrollments:read", "enrollments:manage",
	"assessments:read", "assessments:manage", "assessments:grade", "assessments:release", "assessments:submit",
	"assessments:student:read", "assessments:results:import", "analytics:student:read", "analytics:lecturer:read",
	"analytics:admin:read", "analytics:admin:student:read", "analytics:manage",
}

func TestPermissionCatalogueContract(t *testing.T) {
	seen := make(map[string]bool)
	for _, code := range AllPermissions {
		if seen[code] {
			t.Fatalf("duplicate catalogue permission: %s", code)
		}
		seen[code] = true
	}
	want, got := slices.Clone(permissionContract), slices.Clone(AllPermissions)
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("catalogue %v differs from contract %v", got, want)
	}
	for _, code := range permissionContract {
		if !IsValidPermission(code) {
			t.Errorf("legitimate permission rejected: %s", code)
		}
	}
	for _, invalid := range []string{"", "analytics:*", "Analytics:manage", "assessments:manage ", "not:a:permission"} {
		if IsValidPermission(invalid) {
			t.Errorf("invalid permission accepted: %q", invalid)
		}
	}
}

func TestSeededPermissionRoleBoundaries(t *testing.T) {
	service, _, _, _ := newTestAuthService(t)
	ctx := context.Background()
	wantAdmin := slices.DeleteFunc(slices.Clone(permissionContract), func(code string) bool {
		return slices.Contains([]string{PermAssessmentsStudentRead, PermAnalyticsAdminStudentRead,
			PermAnalyticsStudentRead, PermAnalyticsLecturerRead}, code)
	})
	roles := map[string][]string{
		RoleAdmin: wantAdmin,
		RoleStaff: {PermUsersRead, PermSessionsRead, PermSessionsRevoke, PermAccountUnlock},
		RoleLecturer: {PermUsersRead, PermOfferingsManage, PermEnrollmentsRead,
			PermAssessmentsRead, PermAssessmentsManage, PermAssessmentsGrade, PermAssessmentsRelease, PermAnalyticsLecturerRead},
		RoleStudent: {PermAssessmentsSubmit, PermAnalyticsStudentRead},
	}
	for role, want := range roles {
		actual, err := service.GetRolePermissions(ctx, role)
		if err != nil {
			t.Fatal(err)
		}
		slices.Sort(want)
		if !slices.Equal(actual, want) {
			t.Errorf("%s default grants %v, want %v", role, actual, want)
		}
		for _, code := range actual {
			if !IsValidPermission(code) {
				t.Errorf("%s has unrecognized grant %s", role, code)
			}
		}
	}
}

func TestPermissionMiddlewareCatalogueBoundaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, required := range permissionContract {
		for _, tc := range []struct {
			name   string
			claims *AccessClaims
			status int
		}{
			{"anonymous", nil, http.StatusUnauthorized},
			{"role-alone", &AccessClaims{Roles: []string{RoleAdmin}}, http.StatusForbidden},
			{"unrelated-permission", &AccessClaims{Permissions: []string{"unrelated:read"}}, http.StatusForbidden},
			{"exact-permission", &AccessClaims{Permissions: []string{required}}, http.StatusNoContent},
		} {
			t.Run(required+"/"+tc.name, func(t *testing.T) {
				router := gin.New()
				router.GET("/protected", func(c *gin.Context) {
					if tc.claims != nil {
						c.Set(claimsContextKey, tc.claims)
					}
					c.Next()
				}, RequirePermissions(required), func(c *gin.Context) { c.Status(http.StatusNoContent) })
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/protected", nil))
				if response.Code != tc.status {
					t.Fatalf("got %d, want %d", response.Code, tc.status)
				}
			})
		}
	}
}

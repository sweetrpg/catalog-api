package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/catalog-api/authz"
)

// newVocabTestRouter mirrors newTestRouter but wires setupVocabularyHandlers instead of the
// volume handlers.
func newVocabTestRouter(t *testing.T, roles []string) *gin.Engine {
	t.Helper()

	authAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(authz.CheckResponse{Allowed: true, Roles: roles, Sub: "auth0|test-reviewer"})
	}))
	t.Cleanup(authAPI.Close)

	r := gin.New()
	setupVocabularyHandlers(r, authz.NewClient(authAPI.URL))
	return r
}

func doGet(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer some-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestListVocabularyAvailableToAnyEditCapableRole(t *testing.T) {
	for _, role := range []string{authz.RoleSubmitter, authz.RoleEditor, authz.RoleAdmin} {
		t.Run(role, func(t *testing.T) {
			r := newVocabTestRouter(t, []string{role})
			rec := doGet(t, r, "/vocabularies/contribution-type")
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
			}
		})
	}
}

func TestListVocabularyUnknownType404s(t *testing.T) {
	r := newVocabTestRouter(t, []string{authz.RoleEditor})
	rec := doGet(t, r, "/vocabularies/nonsense")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestListFormatRestrictedToEditorAdmin(t *testing.T) {
	r := newVocabTestRouter(t, []string{authz.RoleSubmitter})
	rec := doGet(t, r, "/vocabularies/format")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestListFormatAllowedForEditor(t *testing.T) {
	r := newVocabTestRouter(t, []string{authz.RoleEditor})
	rec := doGet(t, r, "/vocabularies/format")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestAddVocabularyValueEditorOnly(t *testing.T) {
	r := newVocabTestRouter(t, []string{authz.RoleSubmitter})
	rec := doPost(t, r, "/vocabularies/property-name", map[string]string{"value": "Submitter Attempt"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestAddVocabularyValueByEditorBecomesListable(t *testing.T) {
	r := newVocabTestRouter(t, []string{authz.RoleEditor})

	rec := doPost(t, r, "/vocabularies/property-name", map[string]string{"value": "Page Count"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST status = %d, want %d, body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var created vocabularyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !contains(created.Values, "Page Count") {
		t.Errorf("POST response values = %v, want to contain %q", created.Values, "Page Count")
	}

	getRec := doGet(t, r, "/vocabularies/property-name")
	var listed vocabularyResponse
	if err := json.Unmarshal(getRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !contains(listed.Values, "Page Count") {
		t.Errorf("GET response values = %v, want to contain %q", listed.Values, "Page Count")
	}
}

func TestAddVocabularyValueRequiresValue(t *testing.T) {
	r := newVocabTestRouter(t, []string{authz.RoleEditor})
	rec := doPost(t, r, "/vocabularies/property-name", map[string]string{"value": ""})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

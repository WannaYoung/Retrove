package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"retrove/internal/config"
	"retrove/internal/platform"
	"retrove/internal/store"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "retrove.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return New(config.Config{}, db)
}

func TestRegisterLoginAndHideSecondUser(t *testing.T) {
	srv := testServer(t)

	status := getJSON(t, srv, "/api/v1/auth/status")
	if status["has_user"] != false {
		t.Fatalf("status %#v", status)
	}

	rec := postJSON(t, srv, "/api/v1/auth/register", `{"username":"yang","email":"a@b.c","password":"short","password_confirm":"short"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("short password status %d", rec.Code)
	}
	rec = postJSON(t, srv, "/api/v1/auth/register", `{"username":"yang","email":"a@b.c","password":"password1","password_confirm":"password2"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("mismatch status %d", rec.Code)
	}

	rec = postJSON(t, srv, "/api/v1/auth/register", `{"username":"yang","email":"a@b.c","password":"password1","password_confirm":"password1"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register %d %s", rec.Code, rec.Body.String())
	}
	cookie := rec.Result().Cookies()[0]

	status = getJSON(t, srv, "/api/v1/auth/status")
	if status["has_user"] != true {
		t.Fatalf("status after register %#v", status)
	}
	rec = postJSON(t, srv, "/api/v1/auth/register", `{"username":"other","email":"c@d.e","password":"password1","password_confirm":"password1"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("second register %d", rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(cookie)
	meRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(meRec, req)
	if meRec.Code != http.StatusOK {
		t.Fatalf("me %d", meRec.Code)
	}

	rec = postJSON(t, srv, "/api/v1/auth/login", `{"username":"yang","password":"wrongpass"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad login %d", rec.Code)
	}
	rec = postJSON(t, srv, "/api/v1/auth/login", `{"username":"yang","password":"password1"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login %d %s", rec.Code, rec.Body.String())
	}
}

func getJSON(t *testing.T, srv *Server, path string) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func postJSON(t *testing.T, srv *Server, path, raw string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

func TestHealth(t *testing.T) {
	srv := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), `"ok"`) {
		t.Fatal(string(body))
	}
}

func TestPlatformsRequireAuth(t *testing.T) {
	srv := testServer(t)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/platforms", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}

	reg := postJSON(t, srv, "/api/v1/auth/register", `{"username":"yang","email":"a@b.c","password":"password1","password_confirm":"password1"}`)
	if reg.Code != http.StatusCreated {
		t.Fatal(reg.Body.String())
	}
	cookie := reg.Result().Cookies()[0]
	if err := platform.Sync(srv.db, []platform.Definition{{
		ID: "sfc", Name: "SFC", Directory: "SFC", Extensions: []string{"sfc"}, HashMode: "content", RenamePolicy: "allow", SortOrder: 1,
	}}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/platforms", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

package network

import (
	"database/sql"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"locafy.dev/desktop/internal/db"
)

func openDatabase(t *testing.T) *sql.DB {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "locafy.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return database
}

// projectWithApplication creates a project and one application in it.
func projectWithApplication(t *testing.T, database *sql.DB, name string) (string, string) {
	t.Helper()

	projectID := db.NewID()
	if _, err := database.Exec(`INSERT INTO projects (id, name, root_path, kind) VALUES (?, ?, ?, 'managed')`,
		projectID, name, t.TempDir()); err != nil {
		t.Fatalf("create project: %v", err)
	}
	applicationID := db.NewID()
	if _, err := database.Exec(`INSERT INTO applications (id, project_id, name, directory) VALUES (?, ?, ?, ?)`,
		applicationID, projectID, name, name); err != nil {
		t.Fatalf("create application: %v", err)
	}
	return projectID, applicationID
}

func TestNormalizeHostname(t *testing.T) {
	cases := []struct {
		raw  string
		want string
		err  error
	}{
		{"api.etugora.test", "api.etugora.test", nil},
		{"  API.Etugora.TEST. ", "api.etugora.test", nil},
		{"nestspot.test", "nestspot.test", nil},
		{"api.locafy.dev", "", ErrReservedHostname},
		{"locafy.dev", "", ErrReservedHostname},
		{"api.etugora.local", "", ErrInvalidHostname},
		{"api..test", "", ErrInvalidHostname},
		{"-api.etugora.test", "", ErrInvalidHostname},
		{"api.etugora.test:3000", "", ErrInvalidHostname},
		{"api/etugora.test", "", ErrInvalidHostname},
		{"", "", ErrInvalidHostname},
	}

	for _, test := range cases {
		got, err := NormalizeHostname(test.raw)
		if !errors.Is(err, test.err) {
			t.Errorf("NormalizeHostname(%q) error = %v, want %v", test.raw, err, test.err)
			continue
		}
		if got != test.want {
			t.Errorf("NormalizeHostname(%q) = %q, want %q", test.raw, got, test.want)
		}
	}
}

func TestAssignStoresTheNormalisedNameAgainstItsApplication(t *testing.T) {
	database := openDatabase(t)
	projectID, applicationID := projectWithApplication(t, database, "Etugora")

	assigned, err := Assign(database, projectID, applicationID, "  API.etugora.TEST ")
	if err != nil {
		t.Fatalf("assign: %v", err)
	}
	if assigned.Hostname != "api.etugora.test" {
		t.Fatalf("hostname = %q, want the normalised name", assigned.Hostname)
	}

	domains, err := List(database, projectID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(domains) != 1 {
		t.Fatalf("domains = %+v, want one", domains)
	}
	if domains[0].ApplicationName != "Etugora" || domains[0].Port != nil {
		t.Fatalf("domain = %+v, want the application named and no port before a start", domains[0])
	}
}

func TestAssignRefusesAHostnameAnotherProjectHolds(t *testing.T) {
	database := openDatabase(t)
	projectID, applicationID := projectWithApplication(t, database, "Etugora")
	otherProject, otherApplication := projectWithApplication(t, database, "Nestspot")

	if _, err := Assign(database, projectID, applicationID, "api.etugora.test"); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if _, err := Assign(database, otherProject, otherApplication, "API.Etugora.test."); !errors.Is(err, ErrHostnameClaimed) {
		t.Fatalf("second assign error = %v, want the hostname claim refused", err)
	}

	// A released hostname is available again.
	domains, err := List(database, projectID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if err := Remove(database, domains[0].ID); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := Assign(database, otherProject, otherApplication, "api.etugora.test"); err != nil {
		t.Fatalf("assign after release: %v", err)
	}
}

func TestAssignRefusesAnApplicationFromAnotherProject(t *testing.T) {
	database := openDatabase(t)
	projectID, _ := projectWithApplication(t, database, "Etugora")
	_, foreignApplication := projectWithApplication(t, database, "Nestspot")

	if _, err := Assign(database, projectID, foreignApplication, "api.etugora.test"); !errors.Is(err, ErrUnknownApplication) {
		t.Fatalf("error = %v, want the foreign application refused", err)
	}
}

// testProxy starts the proxy on an ephemeral port and answers its address.
func testProxy(t *testing.T, database *sql.DB) (*Proxy, string) {
	t.Helper()

	proxy := NewProxy(database, nil)
	status, err := proxy.Start("127.0.0.1:0")
	if err != nil {
		t.Fatalf("start proxy: %v", err)
	}
	t.Cleanup(func() { proxy.Stop() })
	return proxy, status.Address
}

func get(t *testing.T, address string, host string) (*http.Response, string) {
	t.Helper()

	request, err := http.NewRequest(http.MethodGet, "http://"+address+"/", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	request.Host = host
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("get %s: %v", host, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return response, string(body)
}

func TestProxyRoutesByHostname(t *testing.T) {
	database := openDatabase(t)
	projectID, applicationID := projectWithApplication(t, database, "Etugora")
	if _, err := Assign(database, projectID, applicationID, "api.etugora.test"); err != nil {
		t.Fatalf("assign: %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "api.etugora.test" {
			t.Errorf("upstream Host = %q, want the domain the browser asked for", r.Host)
		}
		if forwarded := r.Header.Get("X-Forwarded-Host"); forwarded != "api.etugora.test" {
			t.Errorf("X-Forwarded-Host = %q, want the original host", forwarded)
		}
		_, _ = io.WriteString(w, "hello from the application")
	}))
	defer upstream.Close()

	parsed, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream: %v", err)
	}
	_, port, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatalf("upstream port: %v", err)
	}
	if _, err := database.Exec(`UPDATE applications SET port = ? WHERE id = ?`, port, applicationID); err != nil {
		t.Fatalf("set port: %v", err)
	}

	proxy, address := testProxy(t, database)
	proxy.Reload()

	response, body := get(t, address, "api.etugora.test")
	if response.StatusCode != http.StatusOK || !strings.Contains(body, "hello from the application") {
		t.Fatalf("response = %d %q, want the upstream's answer", response.StatusCode, body)
	}

	missing, body := get(t, address, "unknown.etugora.test")
	if missing.StatusCode != http.StatusNotFound || !strings.Contains(body, "No running application is assigned") {
		t.Fatalf("unknown host = %d %q, want a clear 404", missing.StatusCode, body)
	}
}

func TestProxyReportsAnApplicationThatIsNotAnswering(t *testing.T) {
	database := openDatabase(t)
	projectID, applicationID := projectWithApplication(t, database, "Etugora")
	if _, err := Assign(database, projectID, applicationID, "api.etugora.test"); err != nil {
		t.Fatalf("assign: %v", err)
	}

	// A port nobody listens on any more: bound, read, released.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("port: %v", err)
	}
	listener.Close()
	if _, err := database.Exec(`UPDATE applications SET port = ? WHERE id = ?`, port, applicationID); err != nil {
		t.Fatalf("set port: %v", err)
	}

	proxy, address := testProxy(t, database)
	proxy.Reload()

	response, body := get(t, address, "api.etugora.test")
	if response.StatusCode != http.StatusBadGateway || !strings.Contains(body, "could not reach the application") {
		t.Fatalf("response = %d %q, want a 502 naming the failure", response.StatusCode, body)
	}
}

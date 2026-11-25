package network

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
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

	proxy := NewProxy(database, NewAuthority(t.TempDir()), nil)
	status, err := proxy.Start([]string{"127.0.0.1:0"}, nil)
	if err != nil {
		t.Fatalf("start proxy: %v", err)
	}
	t.Cleanup(func() { proxy.Stop() })
	return proxy, status.HTTPAddress
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

func TestProxyFallsBackWhenTheFirstPortIsTaken(t *testing.T) {
	database := openDatabase(t)

	// Something is already on the preferred address.
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer occupied.Close()

	proxy := NewProxy(database, NewAuthority(t.TempDir()), nil)
	status, err := proxy.Start([]string{occupied.Addr().String(), "127.0.0.1:0"}, nil)
	if err != nil {
		t.Fatalf("start with a taken port: %v", err)
	}
	t.Cleanup(func() { proxy.Stop() })

	if !status.Running || status.HTTPAddress == occupied.Addr().String() || status.HTTPAddress == "" {
		t.Fatalf("status = %+v, want the fallback address bound and reported", status)
	}
}

func TestProxyServesHTTPSWithItsOwnAuthority(t *testing.T) {
	database := openDatabase(t)
	projectID, applicationID := projectWithApplication(t, database, "Etugora")
	if _, err := Assign(database, projectID, applicationID, "api.etugora.test"); err != nil {
		t.Fatalf("assign: %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "secure hello")
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

	authority := NewAuthority(t.TempDir())
	proxy := NewProxy(database, authority, nil)
	status, err := proxy.Start([]string{"127.0.0.1:0"}, []string{"127.0.0.1:0"})
	if err != nil {
		t.Fatalf("start proxy: %v", err)
	}
	t.Cleanup(func() { proxy.Stop() })
	if status.HTTPSAddress == "" {
		t.Fatalf("status = %+v, want an HTTPS address", status)
	}

	rawCA, err := os.ReadFile(authority.CAPath())
	if err != nil {
		t.Fatalf("read authority: %v", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(rawCA) {
		t.Fatal("the authority certificate did not parse")
	}

	request, err := http.NewRequest(http.MethodGet, "https://"+status.HTTPSAddress+"/", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	request.Host = "api.etugora.test"
	// The address is a loopback IP, so the certificate does not name it: the
	// browser reaches the same listener through the hostname, and so does this
	// client by asking TLS to verify that name.
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{
			RootCAs:    roots,
			ServerName: "api.etugora.test",
		}},
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("https request: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "secure hello") {
		t.Fatalf("response = %d %q, want the upstream over TLS", response.StatusCode, body)
	}
}

func TestProxyAnnouncesOnlyWhenTheTableChanges(t *testing.T) {
	database := openDatabase(t)
	projectID, applicationID := projectWithApplication(t, database, "Announce")
	if _, err := Assign(database, projectID, applicationID, "api.announce.test"); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if _, err := database.Exec(`UPDATE applications SET port = 41000 WHERE id = ?`, applicationID); err != nil {
		t.Fatalf("set port: %v", err)
	}

	events := 0
	proxy := NewProxy(database, NewAuthority(t.TempDir()), func(string, any) { events++ })
	if _, err := proxy.Start([]string{"127.0.0.1:0"}, nil); err != nil {
		t.Fatalf("start proxy: %v", err)
	}
	t.Cleanup(func() { proxy.Stop() })

	first := events
	if first == 0 {
		t.Fatal("starting the proxy announced nothing")
	}

	// The refresh runs every second; an unchanged table must stay silent.
	proxy.Reload()
	proxy.Reload()
	if events != first {
		t.Fatalf("events = %d after two unchanged reloads, want %d", events, first)
	}

	if _, err := database.Exec(`UPDATE applications SET port = 41001 WHERE id = ?`, applicationID); err != nil {
		t.Fatalf("move port: %v", err)
	}
	proxy.Reload()
	if events != first+1 {
		t.Fatalf("events = %d after a changed table, want %d", events, first+1)
	}
}

func TestProxyKeepsItsCertificateAcrossUnchangedReloads(t *testing.T) {
	database := openDatabase(t)
	projectID, applicationID := projectWithApplication(t, database, "Stable")
	if _, err := Assign(database, projectID, applicationID, "api.stable.test"); err != nil {
		t.Fatalf("assign: %v", err)
	}

	authority := NewAuthority(t.TempDir())
	proxy := NewProxy(database, authority, nil)
	if _, err := proxy.Start([]string{"127.0.0.1:0"}, []string{"127.0.0.1:0"}); err != nil {
		t.Fatalf("start proxy: %v", err)
	}
	t.Cleanup(func() { proxy.Stop() })

	issued, err := os.ReadFile(authority.leafCertPath())
	if err != nil {
		t.Fatalf("read served certificate: %v", err)
	}
	proxy.Reload()
	again, err := os.ReadFile(authority.leafCertPath())
	if err != nil {
		t.Fatalf("read served certificate: %v", err)
	}
	if !bytes.Equal(issued, again) {
		t.Error("an unchanged reload reissued the certificate")
	}
}

func TestProxyServesTheRevocationListItsCertificatePointsAt(t *testing.T) {
	database := openDatabase(t)
	projectID, applicationID := projectWithApplication(t, database, "Revoked")
	if _, err := Assign(database, projectID, applicationID, "api.revoked.test"); err != nil {
		t.Fatalf("assign: %v", err)
	}

	authority := NewAuthority(t.TempDir())
	proxy := NewProxy(database, authority, nil)
	status, err := proxy.Start([]string{"127.0.0.1:0"}, []string{"127.0.0.1:0"})
	if err != nil {
		t.Fatalf("start proxy: %v", err)
	}
	t.Cleanup(func() { proxy.Stop() })

	// The leaf points at the address the proxy actually bound, not at a port it
	// would like to have.
	wantURL := "http://" + status.HTTPAddress + RevocationListPath
	served := proxy.certificate.Load()
	if served == nil {
		t.Fatal("no certificate was issued")
	}
	leaf, err := x509.ParseCertificate(served.Certificate[0])
	if err != nil {
		t.Fatalf("parse served certificate: %v", err)
	}
	if len(leaf.CRLDistributionPoints) != 1 || leaf.CRLDistributionPoints[0] != wantURL {
		t.Fatalf("CRL distribution points = %v, want [%s]", leaf.CRLDistributionPoints, wantURL)
	}

	response, err := http.Get(wantURL)
	if err != nil {
		t.Fatalf("fetch revocation list: %v", err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read revocation list: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	if got := response.Header.Get("Content-Type"); got != "application/pkix-crl" {
		t.Errorf("content type = %q, want application/pkix-crl", got)
	}

	list, err := x509.ParseRevocationList(raw)
	if err != nil {
		t.Fatalf("parse revocation list: %v", err)
	}
	ca, err := authority.Ensure()
	if err != nil {
		t.Fatalf("read authority: %v", err)
	}
	if err := list.CheckSignatureFrom(ca); err != nil {
		t.Fatalf("the revocation list is not signed by the authority: %v", err)
	}
	if time.Now().After(list.NextUpdate) {
		t.Errorf("the revocation list expired at %s", list.NextUpdate)
	}
	if len(list.RevokedCertificateEntries) != 0 {
		t.Errorf("revoked entries = %d, want none", len(list.RevokedCertificateEntries))
	}
}

func TestProxyReissuesWhenTheRevocationListMoves(t *testing.T) {
	database := openDatabase(t)
	projectID, applicationID := projectWithApplication(t, database, "Moved")
	if _, err := Assign(database, projectID, applicationID, "api.moved.test"); err != nil {
		t.Fatalf("assign: %v", err)
	}

	authority := NewAuthority(t.TempDir())
	proxy := NewProxy(database, authority, nil)
	if _, err := proxy.Start([]string{"127.0.0.1:0"}, []string{"127.0.0.1:0"}); err != nil {
		t.Fatalf("start proxy: %v", err)
	}
	t.Cleanup(func() { proxy.Stop() })

	before := proxy.certificate.Load()
	// A different listener means a different CRL address, and a certificate that
	// still pointed at the old one would fail revocation checks.
	proxy.mu.Lock()
	proxy.crlURL = "http://127.0.0.1:1" + RevocationListPath
	proxy.mu.Unlock()
	proxy.Reload()

	after := proxy.certificate.Load()
	if before == nil || after == nil {
		t.Fatal("no certificate was issued")
	}
	if bytes.Equal(before.Certificate[0], after.Certificate[0]) {
		t.Error("the certificate was not reissued for the new revocation-list address")
	}
	leaf, err := x509.ParseCertificate(after.Certificate[0])
	if err != nil {
		t.Fatalf("parse served certificate: %v", err)
	}
	if len(leaf.CRLDistributionPoints) != 1 || leaf.CRLDistributionPoints[0] != proxy.crlURL {
		t.Fatalf("CRL distribution points = %v, want [%s]", leaf.CRLDistributionPoints, proxy.crlURL)
	}
}

func TestProxyStopsRoutingARemovedProjectsDomains(t *testing.T) {
	database := openDatabase(t)
	projectID, applicationID := projectWithApplication(t, database, "Gone")
	if _, err := Assign(database, projectID, applicationID, "api.gone.test"); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if _, err := database.Exec(`UPDATE applications SET port = 41998 WHERE id = ?`, applicationID); err != nil {
		t.Fatalf("set port: %v", err)
	}

	proxy, address := testProxy(t, database)
	request, err := http.NewRequest(http.MethodGet, "http://"+address+"/", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	request.Host = "api.gone.test"
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d before removal, want a route to the application", response.StatusCode)
	}

	if _, err := database.Exec(`UPDATE projects SET deleted_at = strftime('%Y-%m-%dT%H:%M:%SZ','now') WHERE id = ?`, projectID); err != nil {
		t.Fatalf("remove project: %v", err)
	}
	if status := proxy.Reload(); status.Routes != 0 {
		t.Fatalf("routes = %d after removal, want 0", status.Routes)
	}

	again, err := http.NewRequest(http.MethodGet, "http://"+address+"/", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	again.Host = "api.gone.test"
	removed, err := http.DefaultClient.Do(again)
	if err != nil {
		t.Fatalf("request after removal: %v", err)
	}
	removed.Body.Close()
	if removed.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d after removal, want 404", removed.StatusCode)
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

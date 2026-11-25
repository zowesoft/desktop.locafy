package network

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// The listener candidates, in order. Nothing privileged is assumed: 80 and 443
// come first because they make api.project.test work without a port suffix, and
// an unprivileged port is used when the machine refuses them. The interface
// states the address that was actually bound, because a URL with :8080 is not
// the same promise as one without.
var (
	DefaultHTTP  = []string{"127.0.0.1:80", "127.0.0.1:8080"}
	DefaultHTTPS = []string{"127.0.0.1:443", "127.0.0.1:8443"}
)

// EventProxy announces a change of proxy state or routing table.
const EventProxy = "network:proxy"

// refreshInterval is how often the routing table is rebuilt while the proxy
// runs, so an application that binds its port after a domain was assigned starts
// routing without the operator doing anything.
const refreshInterval = time.Second

// ProxyStatus is the proxy's own state, independent of any project.
type ProxyStatus struct {
	Running      bool   `json:"running"`
	HTTPAddress  string `json:"httpAddress"`
	HTTPSAddress string `json:"httpsAddress"`
	Routes       int    `json:"routes"`
	Error        string `json:"error"`
}

// Proxy is Locafy's local reverse proxy: loopback listeners, a hostname table
// rebuilt from the database, and an atomic swap when the table changes. There is
// no configuration file to write or reload: the table is the configuration, and
// it is built only from validated hostnames and live application ports.
type Proxy struct {
	database  *sql.DB
	authority *Authority
	emit      func(string, any)

	mu           sync.RWMutex
	routes       map[string]*httputil.ReverseProxy
	targets      map[string]string
	loaded       bool
	hosts        []string
	server       *http.Server
	tlsServer    *http.Server
	listener     net.Listener
	tlsListener  net.Listener
	httpAddress  string
	httpsAddress string
	err          string
	stop         chan struct{}
	certificate  atomic.Pointer[tls.Certificate]
}

// NewProxy returns a proxy that routes the domains stored in database. The
// authority issues the certificates for the TLS listener; it is created lazily
// the first time HTTPS starts.
func NewProxy(database *sql.DB, authority *Authority, emit func(string, any)) *Proxy {
	return &Proxy{
		database:     database,
		authority:    authority,
		emit:         emit,
		routes:       map[string]*httputil.ReverseProxy{},
		httpAddress:  "",
		httpsAddress: "",
	}
}

// Start binds the first available HTTP address and, when httpsCandidates are
// given, the first available HTTPS address. A listener that is already open is
// left alone, so a second start is not an error.
func (p *Proxy) Start(httpCandidates []string, httpsCandidates []string) (ProxyStatus, error) {
	if len(httpCandidates) == 0 {
		httpCandidates = DefaultHTTP
	}

	p.mu.Lock()
	if p.server != nil {
		status := p.statusLocked()
		p.mu.Unlock()
		return status, nil
	}

	listener, httpAddress, err := listenFirst(httpCandidates)
	if err != nil {
		p.err = err.Error()
		status := p.statusLocked()
		p.mu.Unlock()
		p.announce()
		return status, err
	}

	stop := make(chan struct{})
	p.listener = listener
	p.httpAddress = httpAddress
	p.err = ""
	p.server = &http.Server{
		Handler:           http.HandlerFunc(p.serve),
		ReadHeaderTimeout: 5 * time.Second,
	}
	p.stop = stop
	server := p.server
	status := p.statusLocked()
	p.mu.Unlock()

	go func() { _ = server.Serve(listener) }()
	go p.refresh(stop)

	if len(httpsCandidates) > 0 {
		p.startTLS(httpsCandidates)
	}

	// Reload announces the first table itself; announcing again here would
	// send the interface the same state twice.
	status = p.Reload()
	return status, nil
}

// startTLS binds the HTTPS listener and issues the certificate it serves. A
// failure here is reported but does not take the HTTP listener down: routing
// over HTTP still works while the certificate problem is visible.
func (p *Proxy) startTLS(candidates []string) {
	if p.authority == nil {
		return
	}

	listener, address, err := listenFirst(candidates)
	if err != nil {
		p.mu.Lock()
		p.err = fmt.Sprintf("HTTPS is not listening: %v", err)
		p.mu.Unlock()
		return
	}

	server := &http.Server{
		Handler:           http.HandlerFunc(p.serve),
		ReadHeaderTimeout: 5 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
				certificate := p.certificate.Load()
				if certificate == nil {
					return nil, fmt.Errorf("no domain certificate is issued yet")
				}
				return certificate, nil
			},
		},
	}

	p.mu.Lock()
	p.tlsServer = server
	p.tlsListener = listener
	p.httpsAddress = address
	p.mu.Unlock()

	go func() { _ = server.ServeTLS(listener, "", "") }()
}

// Stop closes both listeners.
func (p *Proxy) Stop() ProxyStatus {
	p.mu.Lock()
	httpServer, tlsServer := p.server, p.tlsServer
	stop := p.stop
	p.server, p.tlsServer = nil, nil
	p.listener, p.tlsListener = nil, nil
	p.stop = nil
	p.routes = map[string]*httputil.ReverseProxy{}
	p.targets = nil
	p.loaded = false
	p.certificate.Store(nil)
	if stop != nil {
		close(stop)
	}
	status := p.statusLocked()
	p.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if httpServer != nil {
		_ = httpServer.Shutdown(ctx)
	}
	if tlsServer != nil {
		_ = tlsServer.Shutdown(ctx)
	}
	p.announce()
	return status
}

// Status reports the listeners, the route count and the last failure.
func (p *Proxy) Status() ProxyStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.statusLocked()
}

// Reload rebuilds the routing table from the database and swaps it in one step,
// so a request is served by either the old table or the new one. When the set of
// hostnames changed, the TLS certificate is reissued for exactly that set.
//
// It runs once a second, so it is also quiet: an unchanged table is not swapped
// and not announced. Rebuilding new route objects and emitting them every second
// would make the interface re-render forever for no reason.
func (p *Proxy) Reload() ProxyStatus {
	routes, hosts, targets, err := p.buildRoutes()

	p.mu.RLock()
	unchanged := p.loaded && targetsEqual(p.targets, targets)
	tlsOn := p.tlsListener != nil
	haveCertificate := p.certificate.Load() != nil
	p.mu.RUnlock()

	// The certificate is reused when it already names this host set: the
	// authority reissues only when the names differ or it is close to expiry.
	if err == nil && tlsOn && p.authority != nil && (!unchanged || !haveCertificate) {
		if certificate, certErr := p.authority.Leaf(hosts); certErr != nil {
			err = fmt.Errorf("HTTPS certificate: %w", certErr)
		} else {
			p.certificate.Store(&certificate)
		}
	}

	p.mu.Lock()
	changed := false
	if err != nil {
		if p.err != err.Error() {
			p.err = err.Error()
			changed = true
		}
	} else {
		if p.err != "" {
			p.err = ""
			changed = true
		}
		if !unchanged {
			p.routes = routes
			p.targets = targets
			p.hosts = hosts
			p.loaded = true
			changed = true
		}
	}
	status := p.statusLocked()
	running := p.server != nil
	p.mu.Unlock()

	if running && changed {
		p.announce()
	}
	return status
}

// TrustCommand returns the platform command that trusts Locafy's CA, so the
// interface can show exactly what it will run before it runs it.
func (p *Proxy) TrustCommand() (string, []string, string, error) {
	if p.authority == nil {
		return "", nil, "", fmt.Errorf("no certificate authority is configured")
	}
	return p.authority.TrustCommand()
}

// CAPath is the certificate a user trusts once.
func (p *Proxy) CAPath() string {
	if p.authority == nil {
		return ""
	}
	return p.authority.CAPath()
}

// serve routes one request by hostname. An unknown host is answered here rather
// than proxied anywhere.
func (p *Proxy) serve(w http.ResponseWriter, r *http.Request) {
	host := strings.ToLower(strings.TrimSuffix(r.Host, "."))
	if name, _, err := net.SplitHostPort(host); err == nil {
		host = name
	}

	p.mu.RLock()
	target, ok := p.routes[host]
	p.mu.RUnlock()
	if !ok {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "No running application is assigned to %s.\n", host)
		return
	}
	target.ServeHTTP(w, r)
}

// refresh keeps the routing table and the certificate current while the proxy
// runs: an application binds its port after its domain was assigned, and a
// domain added later must be in the certificate.
func (p *Proxy) refresh(stop <-chan struct{}) {
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			p.Reload()
		}
	}
}

// buildRoutes reads every live domain whose application has a discovered port.
// A domain whose application has not bound a port yet has no route: it is a
// valid record that cannot route until the application runs.
func (p *Proxy) buildRoutes() (map[string]*httputil.ReverseProxy, []string, map[string]string, error) {
	routes := make(map[string]*httputil.ReverseProxy)
	targets := make(map[string]string)
	if p.database == nil {
		return routes, nil, targets, nil
	}

	rows, err := p.database.Query(`SELECT d.hostname, a.port
		FROM domains d
		JOIN applications a ON a.id = d.application_id
		WHERE d.deleted_at IS NULL AND a.deleted_at IS NULL AND a.port IS NOT NULL`)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()

	hosts := make([]string, 0)
	for rows.Next() {
		var hostname string
		var port int
		if err := rows.Scan(&hostname, &port); err != nil {
			return nil, nil, nil, err
		}
		target := fmt.Sprintf("127.0.0.1:%d", port)
		routes[hostname] = reverseProxy(target)
		targets[hostname] = target
		hosts = append(hosts, hostname)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, err
	}
	slices.Sort(hosts)
	hosts = slices.Compact(hosts)
	return routes, hosts, targets, nil
}

// targetsEqual compares the hostname-to-port table, not the route objects: two
// builds of the same table hold different pointers and mean the same thing.
func targetsEqual(a map[string]string, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for hostname, target := range a {
		if b[hostname] != target {
			return false
		}
	}
	return true
}

// reverseProxy forwards one hostname to one loopback port. ReverseProxy passes
// WebSocket upgrades through, so a development WebSocket server needs no
// separate route.
func reverseProxy(target string) *httputil.ReverseProxy {
	upstream := &url.URL{Scheme: "http", Host: target}
	return &httputil.ReverseProxy{
		Rewrite: func(request *httputil.ProxyRequest) {
			request.SetURL(upstream)
			request.SetXForwarded()
			// Keep the browser's Host: frameworks build absolute URLs and cookie
			// domains from the name the developer typed.
			request.Out.Host = request.In.Host
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, "Locafy could not reach the application behind %s: %v\n", r.Host, err)
		},
	}
}

func (p *Proxy) statusLocked() ProxyStatus {
	return ProxyStatus{
		Running:      p.server != nil,
		HTTPAddress:  p.httpAddress,
		HTTPSAddress: p.httpsAddress,
		Routes:       len(p.routes),
		Error:        p.err,
	}
}

func (p *Proxy) announce() {
	if p.emit != nil {
		p.emit(EventProxy, p.Status())
	}
}

// listenFirst binds the first candidate that is free, so a privileged or busy
// port does not stop the proxy from serving.
func listenFirst(candidates []string) (net.Listener, string, error) {
	var failures []string
	for _, candidate := range candidates {
		listener, err := net.Listen("tcp", candidate)
		if err == nil {
			return listener, listener.Addr().String(), nil
		}
		failures = append(failures, fmt.Sprintf("%s: %v", candidate, err))
	}
	return nil, "", fmt.Errorf("no local port could be bound (%s)", strings.Join(failures, "; "))
}

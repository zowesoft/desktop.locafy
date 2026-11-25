package network

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

// DefaultAddress is where the proxy listens. Port 80 is what lets
// api.project.test work without a port suffix; a refused bind is reported as the
// proxy's status rather than worked around.
const DefaultAddress = "127.0.0.1:80"

// EventProxy announces a change of proxy state or routing table.
const EventProxy = "network:proxy"

// refreshInterval is how often the routing table is rebuilt while the proxy
// runs, so an application that binds its port after a domain was assigned starts
// routing without the operator doing anything.
const refreshInterval = time.Second

// ProxyStatus is the proxy's own state, independent of any project.
type ProxyStatus struct {
	Running bool   `json:"running"`
	Address string `json:"address"`
	Routes  int    `json:"routes"`
	Error   string `json:"error"`
}

// Proxy is Locafy's local reverse proxy: one loopback listener, a hostname table
// rebuilt from the database, and an atomic swap when the table changes. There is
// no configuration file to write or reload: the table is the configuration, and
// it is built only from validated hostnames and live application ports.
type Proxy struct {
	database *sql.DB
	emit     func(string, any)

	mu       sync.RWMutex
	routes   map[string]*httputil.ReverseProxy
	server   *http.Server
	listener net.Listener
	address  string
	err      string
	stop     chan struct{}
}

// NewProxy returns a proxy that routes the domains stored in database.
func NewProxy(database *sql.DB, emit func(string, any)) *Proxy {
	return &Proxy{
		database: database,
		emit:     emit,
		routes:   map[string]*httputil.ReverseProxy{},
		address:  DefaultAddress,
	}
}

// Start binds the listener and begins routing. A listener that is already open
// is left alone, so a second start is not an error.
func (p *Proxy) Start(address string) (ProxyStatus, error) {
	if strings.TrimSpace(address) == "" {
		address = DefaultAddress
	}

	p.mu.Lock()
	if p.server != nil {
		status := p.statusLocked()
		p.mu.Unlock()
		return status, nil
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		p.address = address
		p.err = err.Error()
		status := p.statusLocked()
		p.mu.Unlock()
		p.announce()
		return status, err
	}

	stop := make(chan struct{})
	p.listener = listener
	p.address = listener.Addr().String()
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

	status = p.Reload()
	p.announce()
	return status, nil
}

// Stop closes the listener. Routing stops, and the last status keeps the address
// it had so the interface can still explain what was running.
func (p *Proxy) Stop() ProxyStatus {
	p.mu.Lock()
	server := p.server
	stop := p.stop
	p.server = nil
	p.listener = nil
	p.stop = nil
	p.routes = map[string]*httputil.ReverseProxy{}
	if stop != nil {
		close(stop)
	}
	status := p.statusLocked()
	p.mu.Unlock()

	if server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}
	p.announce()
	return status
}

// Status reports the listener, the route count and the last bind or refresh
// failure.
func (p *Proxy) Status() ProxyStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.statusLocked()
}

// Reload rebuilds the routing table from the database and swaps it in one step,
// so a request is served by either the old table or the new one.
func (p *Proxy) Reload() ProxyStatus {
	routes, err := p.buildRoutes()

	p.mu.Lock()
	if err != nil {
		p.err = err.Error()
	} else {
		p.err = ""
		p.routes = routes
	}
	status := p.statusLocked()
	running := p.server != nil
	p.mu.Unlock()

	if running {
		p.announce()
	}
	return status
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

// refresh keeps the routing table current while the proxy runs: an application
// binds its port after its domain was assigned, and its route must appear
// without another action.
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
func (p *Proxy) buildRoutes() (map[string]*httputil.ReverseProxy, error) {
	if p.database == nil {
		return map[string]*httputil.ReverseProxy{}, nil
	}

	rows, err := p.database.Query(`SELECT d.hostname, a.port
		FROM domains d
		JOIN applications a ON a.id = d.application_id
		WHERE d.deleted_at IS NULL AND a.deleted_at IS NULL AND a.port IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	routes := make(map[string]*httputil.ReverseProxy)
	for rows.Next() {
		var hostname string
		var port int
		if err := rows.Scan(&hostname, &port); err != nil {
			return nil, err
		}
		target := fmt.Sprintf("127.0.0.1:%d", port)
		routes[hostname] = reverseProxy(target)
	}
	return routes, rows.Err()
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
		Running: p.server != nil,
		Address: p.address,
		Routes:  len(p.routes),
		Error:   p.err,
	}
}

func (p *Proxy) announce() {
	if p.emit != nil {
		p.emit(EventProxy, p.Status())
	}
}

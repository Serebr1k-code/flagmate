package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"gopkg.in/yaml.v3"
	_ "modernc.org/sqlite"
)

type Config struct {
	UI             string
	JWT            string
	EveSocket      string
	DBPath         string
	ListenAddr     string
	StableN        int
	AutoHook       bool
	SuricataY      string
	SuricataPID    int
	GateEnabled    bool
	GateListen     string
	GateUpstream   string
	PoisonImageDir string
}

type Service struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Port      int    `json:"port"`
	Protocol  string `json:"protocol"`
	CreatedAt string `json:"created_at"`
}

type Pattern struct {
	ID          int    `json:"id"`
	ServiceID   *int   `json:"service_id"`
	Pattern     string `json:"pattern"`
	Description string `json:"description"`
	Mode        string `json:"mode"`
	Active      bool   `json:"active"`
	MatchCount  int    `json:"match_count"`
	CreatedAt   string `json:"created_at"`
}

type Flow struct {
	ID           string         `json:"id"`
	ServiceID    *int           `json:"service_id"`
	Direction    string         `json:"direction"`
	StartTS      *string        `json:"start_ts"`
	EndTS        *string        `json:"end_ts"`
	RawRequest   map[string]any `json:"raw_request"`
	RawResponse  map[string]any `json:"raw_response"`
	Hash         string         `json:"hash"`
	Stable       bool           `json:"stable"`
	StabilityPct int            `json:"stability_pct"`
	AvgInterval  float64        `json:"avg_interval"`
	Destination  string         `json:"destination"`
	Checker      bool           `json:"checker"`
	Banned       bool           `json:"banned"`
	Mirrored     bool           `json:"mirrored"`
	GroupName    string         `json:"group_name"`
	GroupCount   int            `json:"group_count"`
	ResponseCode int            `json:"response_code"`
	FlowID       int64          `json:"flow_id"`
	SrcIP        string         `json:"src_ip"`
	DstIP        string         `json:"dst_ip"`
	SrcPort      int            `json:"src_port"`
	DstPort      int            `json:"dst_port"`
	Proto        string         `json:"proto"`
	PktCount     int            `json:"pkt_count"`
	BytesIn      int            `json:"bytes_in"`
	BytesOut     int            `json:"bytes_out"`
	CreatedAt    string         `json:"created_at"`
}

type WS struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]struct{}
}

type App struct {
	cfg              Config
	db               *sql.DB
	ws               *WS
	upgrader         websocket.Upgrader
	mirrorMu         sync.RWMutex
	mirroring        MirroringConfig
	mirrorDue        map[int]time.Time
	poisonMu         sync.Mutex
	poisonHits       map[string][]time.Time
	poisonFlagMinute int64
	poisonFlag       string
}

type MirroringConfig struct {
	Enabled  bool                  `json:"enabled"`
	Targets  []MirrorTarget        `json:"targets"`
	Services []ServiceMirrorConfig `json:"services"`
}

type ServiceMirrorConfig struct {
	ServiceID       int            `json:"service_id"`
	Enabled         bool           `json:"enabled"`
	IntervalSeconds int            `json:"interval_seconds"`
	Targets         []MirrorTarget `json:"targets"`
}

type FlowGroupMeta struct {
	Hash          string `json:"hash"`
	Name          string `json:"name"`
	Checker       bool   `json:"checker"`
	Count         int    `json:"count"`
	ExampleFlowID string `json:"example_flow_id"`
	FirstSeen     string `json:"first_seen"`
	LastSeen      string `json:"last_seen"`
	Destination   string `json:"destination"`
	Method        string `json:"method"`
	URI           string `json:"uri"`
	ResponseCode  int    `json:"response_code"`
	ServiceID     *int   `json:"service_id"`
	Mirrored      bool   `json:"mirrored"`
	LatestFlow    *Flow  `json:"latest_flow,omitempty"`
}

type MirrorTarget struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

type MirrorAttemptStat struct {
	ServiceID int
	Hash      string
	TargetIP  string
	Success   bool
	Flag      string
	CreatedAt time.Time
}

func main() {
	cfg := loadConfig()
	if cfg.AutoHook {
		if err := ensureSuricataUnixHook(cfg.SuricataY, cfg.EveSocket); err != nil {
			log.Printf("suricata hook reconcile warning: %v", err)
		}
		if cfg.SuricataPID > 0 {
			if err := syscall.Kill(cfg.SuricataPID, syscall.SIGHUP); err != nil {
				log.Printf("suricata reload warning: %v", err)
			}
		}
	}

	db, err := sql.Open("sqlite", cfg.DBPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := initSchema(db); err != nil {
		log.Fatalf("init schema: %v", err)
	}

	app := &App{
		cfg: cfg,
		db:  db,
		ws:  &WS{clients: map[*websocket.Conn]struct{}{}},
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(_ *http.Request) bool { return true },
		},
		poisonHits: map[string][]time.Time{},
		mirrorDue:  map[int]time.Time{},
	}
	app.loadMirroring()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go app.startSuricataListener(ctx)
	go app.startMirrorScheduler(ctx)
	if cfg.GateEnabled {
		go app.startHTTPGate(ctx)
	}

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	r.Get("/ws", app.handleWS)
	r.Post("/login", app.login)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Group(func(pr chi.Router) {
		pr.Use(app.auth)
		pr.Get("/services", app.listServices)
		pr.Post("/services", app.createService)
		pr.Delete("/services/{id}", app.deleteService)

		pr.Get("/patterns", app.listPatterns)
		pr.Post("/patterns", app.createPattern)
		pr.Delete("/patterns/{id}", app.deletePattern)
		pr.Post("/patterns/{id}/toggle", app.togglePattern)

		pr.Get("/flows", app.listFlows)
		pr.Get("/flows/history", app.flowHistory)
		pr.Get("/flows/{id}", app.getFlow)
		pr.Get("/flows/{id}/unique-words", app.uniqueWords)
		pr.Post("/flows/{id}/label", app.labelFlow)
		pr.Post("/flows/{id}/mirror", app.mirrorFlowGroup)
		pr.Post("/flows/{id}/unban", app.unbanFlow)
		pr.Get("/flows/{id}/matching-patterns", app.matchingPatternsForFlow)
		pr.Post("/flows/{id}/remove-matching-patterns", app.removeMatchingPatternsForFlow)

		pr.Get("/flow-groups", app.flowGroups)
		pr.Post("/flow-groups/{hash}/name", app.renameFlowGroup)
		pr.Post("/flow-groups/{hash}/checker", app.markFlowGroupChecker)

		pr.Get("/mirroring", app.getMirroring)
		pr.Get("/mirroring/groups", app.mirroredGroups)
		pr.Get("/mirroring/stats", app.mirroringStats)
		pr.Get("/mirroring/attempts", app.mirroringAttempts)
		pr.Post("/mirroring", app.setMirroring)
		pr.Get("/settings", app.getSettings)
		pr.Post("/settings", app.setSettings)
	})

	log.Printf("backend listening on %s", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, r); err != nil {
		log.Fatalf("http server: %v", err)
	}
}

func loadConfig() Config {
	stable := getEnvInt("STABLE_THRESHOLD", 5)
	if stable < 1 {
		stable = 1
	}
	return Config{
		UI:             getenv("UI_PASSWORD", "admin"),
		JWT:            getenv("JWT_SECRET", "supersecretjwtkey"),
		EveSocket:      getenv("EVE_SOCKET", "/var/run/suricata/eve.sock"),
		DBPath:         getenv("DB_PATH", "/data/flagmate.db"),
		ListenAddr:     getenv("LISTEN_ADDR", ":8080"),
		StableN:        stable,
		AutoHook:       strings.ToLower(getenv("AUTO_HOOK_SURICATA", "true")) == "true",
		SuricataY:      getenv("SURICATA_YAML_PATH", "/etc/suricata/suricata.yaml"),
		SuricataPID:    getEnvInt("SURICATA_PID", 0),
		GateEnabled:    strings.ToLower(getenv("GATE_ENABLED", "true")) == "true",
		GateListen:     getenv("GATE_LISTEN", ":18080"),
		GateUpstream:   strings.TrimRight(getenv("GATE_UPSTREAM", "http://testservice:18080"), "/"),
		PoisonImageDir: getenv("POISON_IMAGE_DIR", "/app/poison-images"),
	}
}

func getenv(k, def string) string {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	return v
}

func getEnvInt(k string, def int) int {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return i
}

func initSchema(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS services (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, port INTEGER NOT NULL UNIQUE, protocol TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);`,
		`CREATE TABLE IF NOT EXISTS patterns (id INTEGER PRIMARY KEY AUTOINCREMENT, service_id INTEGER NULL, pattern TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', mode TEXT NOT NULL DEFAULT 'B', active INTEGER NOT NULL DEFAULT 1, match_count INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);`,
		`CREATE TABLE IF NOT EXISTS flows (id TEXT PRIMARY KEY, service_id INTEGER NULL, direction TEXT NOT NULL, start_ts TEXT NULL, end_ts TEXT NULL, raw_request TEXT NOT NULL, raw_response TEXT NOT NULL, hash TEXT NOT NULL, stable INTEGER NOT NULL DEFAULT 0, checker INTEGER NOT NULL DEFAULT 0, banned INTEGER NOT NULL DEFAULT 0, response_code INTEGER NOT NULL DEFAULT 0, flow_id INTEGER NOT NULL DEFAULT 0, src_ip TEXT NOT NULL, dst_ip TEXT NOT NULL, src_port INTEGER NOT NULL, dst_port INTEGER NOT NULL, proto TEXT NOT NULL, pkt_count INTEGER NOT NULL DEFAULT 0, bytes_in INTEGER NOT NULL DEFAULT 0, bytes_out INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);`,
		`CREATE TABLE IF NOT EXISTS flow_payloads (hash TEXT PRIMARY KEY, payload TEXT NOT NULL, bytes INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);`,
		`CREATE INDEX IF NOT EXISTS idx_flows_created_at ON flows(created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_flows_hash ON flows(hash);`,
		`CREATE TABLE IF NOT EXISTS mirroring (id INTEGER PRIMARY KEY CHECK(id=1), enabled INTEGER NOT NULL DEFAULT 0, targets TEXT NOT NULL DEFAULT '[]');`,
		`CREATE TABLE IF NOT EXISTS mirror_groups (hash TEXT PRIMARY KEY, service_id INTEGER NOT NULL, enabled INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);`,
		`CREATE TABLE IF NOT EXISTS flow_group_meta (hash TEXT PRIMARY KEY, name TEXT NOT NULL DEFAULT '', checker INTEGER NOT NULL DEFAULT 0, updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);`,
		`CREATE TABLE IF NOT EXISTS mirror_attempts (id INTEGER PRIMARY KEY AUTOINCREMENT, service_id INTEGER NOT NULL, hash TEXT NOT NULL, flow_id TEXT NOT NULL, target_ip TEXT NOT NULL, target_port INTEGER NOT NULL, success INTEGER NOT NULL DEFAULT 0, flag TEXT NOT NULL DEFAULT '', response TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);`,
		`CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);`,
		`INSERT OR IGNORE INTO mirroring(id, enabled, targets) VALUES (1, 0, '[]');`,
		`INSERT OR IGNORE INTO settings(key, value) VALUES ('poison_mode', 'media');`,
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	_, _ = db.Exec(`ALTER TABLE patterns ADD COLUMN service_id INTEGER NULL`)
	_, _ = db.Exec(`ALTER TABLE mirroring ADD COLUMN services TEXT NOT NULL DEFAULT '[]'`)
	_, _ = db.Exec(`ALTER TABLE mirror_groups ADD COLUMN name TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE flow_group_meta ADD COLUMN name TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE flow_group_meta ADD COLUMN checker INTEGER NOT NULL DEFAULT 0`)
	_, _ = db.Exec(`ALTER TABLE flows ADD COLUMN req_hash TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE flows ADD COLUMN resp_hash TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_patterns_service ON patterns(service_id)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_mirror_groups_service ON mirror_groups(service_id)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_flows_service_created ON flows(service_id, created_at DESC)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_flows_hash_created ON flows(hash, created_at DESC)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_flows_banned ON flows(banned)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_flows_checker ON flows(checker)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_mirror_attempts_group ON mirror_attempts(hash, target_ip, created_at DESC)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_mirror_attempts_created ON mirror_attempts(created_at DESC)`)
	return nil
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}
	if payload.Password != a.cfg.UI {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	claims := jwt.MapClaims{"exp": time.Now().Add(24 * time.Hour).Unix(), "iat": time.Now().Unix(), "sub": "ui"}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString([]byte(a.cfg.JWT))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token generation failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": signed})
}

func (a *App) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing token"})
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")
		_, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
			if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, errors.New("invalid token algorithm")
			}
			return []byte(a.cfg.JWT), nil
		})
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) handleWS(w http.ResponseWriter, r *http.Request) {
	c, err := a.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	a.ws.mu.Lock()
	a.ws.clients[c] = struct{}{}
	a.ws.mu.Unlock()

	for {
		if _, _, err := c.ReadMessage(); err != nil {
			break
		}
	}

	a.ws.mu.Lock()
	delete(a.ws.clients, c)
	a.ws.mu.Unlock()
	_ = c.Close()
}

func (a *App) broadcastFlow(flow Flow) {
	data, err := json.Marshal(flow)
	if err != nil {
		return
	}
	a.ws.mu.RLock()
	clients := make([]*websocket.Conn, 0, len(a.ws.clients))
	for c := range a.ws.clients {
		clients = append(clients, c)
	}
	a.ws.mu.RUnlock()

	for _, c := range clients {
		_ = c.SetWriteDeadline(time.Now().Add(2 * time.Second))
		if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
			a.ws.mu.Lock()
			delete(a.ws.clients, c)
			a.ws.mu.Unlock()
			_ = c.Close()
		}
	}
}

func (a *App) startSuricataListener(ctx context.Context) {
	if err := os.MkdirAll(filepath.Dir(a.cfg.EveSocket), 0o755); err != nil {
		log.Printf("socket directory error: %v", err)
		return
	}
	_ = os.Remove(a.cfg.EveSocket)
	l, err := net.Listen("unix", a.cfg.EveSocket)
	if err != nil {
		log.Printf("unix socket listen error: %v", err)
		return
	}
	defer l.Close()
	_ = os.Chmod(a.cfg.EveSocket, 0o666)
	log.Printf("suricata listener ready: %s", a.cfg.EveSocket)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		conn, err := l.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			time.Sleep(250 * time.Millisecond)
			continue
		}
		go a.readConn(conn)
	}
}

func (a *App) startHTTPGate(ctx context.Context) {
	upstream, err := url.Parse(a.cfg.GateUpstream)
	if err != nil {
		log.Printf("gate upstream parse error: %v", err)
		return
	}
	server := &http.Server{
		Addr: a.cfg.GateListen,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			a.handleGateRequest(w, r, upstream)
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("http gate listening on %s -> %s", a.cfg.GateListen, a.cfg.GateUpstream)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("gate server error: %v", err)
	}
}

func (a *App) handleGateRequest(w http.ResponseWriter, r *http.Request, upstream *url.URL) {
	if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		a.handleGateWebSocket(w, r, upstream)
		return
	}

	target := *upstream
	target.Path = r.URL.Path
	target.RawPath = r.URL.RawPath
	target.RawQuery = r.URL.RawQuery

	reqBody, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()

	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), strings.NewReader(string(reqBody)))
	if err != nil {
		http.Error(w, "gate request error", http.StatusBadGateway)
		return
	}
	copyHeaders(proxyReq.Header, r.Header)
	proxyReq.Header.Set("X-Forwarded-For", r.RemoteAddr)

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	reqMeta := map[string]any{
		"method":  r.Method,
		"uri":     r.URL.Path,
		"query":   r.URL.RawQuery,
		"headers": r.Header,
		"body":    string(reqBody),
	}
	respMeta := map[string]any{
		"status":  resp.StatusCode,
		"headers": resp.Header,
		"body":    string(respBody),
	}
	_, svcID := a.lookupService(listenPortFromAddr(a.cfg.GateListen), listenPortFromAddr(a.cfg.GateListen))
	banned := a.isBanned(reqMeta, respMeta, resp.StatusCode, svcID)

	statusToSend := resp.StatusCode
	bodyToSend := respBody
	if banned {
		poisonedBody, contentType, limited := a.buildPoisonResponse(r)
		statusToSend = http.StatusOK
		if limited {
			statusToSend = http.StatusTooManyRequests
		}
		bodyToSend = poisonedBody
		resp.Header = http.Header{}
		resp.Header.Set("Content-Type", contentType)
		resp.Header.Set("X-FlagMate-Poisoned", "1")
		if limited {
			resp.Header.Set("Retry-After", "60")
		}
	}

	for k := range w.Header() {
		w.Header().Del(k)
	}
	copyHeaders(w.Header(), resp.Header)
	w.Header().Set("Content-Length", strconv.Itoa(len(bodyToSend)))
	w.WriteHeader(statusToSend)
	_, _ = w.Write(bodyToSend)

	a.storeInlineFlow(r, reqMeta, respMeta, banned)
}

func (a *App) handleGateWebSocket(w http.ResponseWriter, r *http.Request, upstream *url.URL) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "websocket unsupported", http.StatusInternalServerError)
		return
	}
	upstreamAddr := upstream.Host
	if !strings.Contains(upstreamAddr, ":") {
		upstreamAddr = net.JoinHostPort(upstreamAddr, "80")
	}
	upConn, err := net.DialTimeout("tcp", upstreamAddr, 3*time.Second)
	if err != nil {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
		return
	}
	defer func() { _ = upConn.Close() }()

	target := *upstream
	target.Path = r.URL.Path
	target.RawQuery = r.URL.RawQuery
	proxyReq := r.Clone(r.Context())
	proxyReq.URL = &target
	proxyReq.RequestURI = ""
	proxyReq.Host = upstream.Host
	if err := proxyReq.Write(upConn); err != nil {
		http.Error(w, "websocket upstream write failed", http.StatusBadGateway)
		return
	}
	br := bufio.NewReader(upConn)
	resp, err := http.ReadResponse(br, proxyReq)
	if err != nil {
		http.Error(w, "websocket upstream handshake failed", http.StatusBadGateway)
		return
	}

	clientConn, rw, err := hj.Hijack()
	if err != nil {
		return
	}
	defer func() { _ = clientConn.Close() }()
	_ = resp.Write(rw)
	_ = rw.Flush()

	reqMeta := map[string]any{"method": r.Method, "uri": r.URL.Path, "query": r.URL.RawQuery, "headers": r.Header, "body": ""}
	respMeta := map[string]any{"status": resp.StatusCode, "headers": resp.Header, "body": "websocket upgrade"}
	a.storeInlineFlow(r, reqMeta, respMeta, false)

	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(upConn, rw); done <- struct{}{} }()
	go func() { _, _ = io.Copy(clientConn, br); done <- struct{}{} }()
	<-done
}

func (a *App) storeInlineFlow(r *http.Request, reqMeta, respMeta map[string]any, banned bool) {
	_, svcID := a.lookupService(listenPortFromAddr(a.cfg.GateListen), listenPortFromAddr(a.cfg.GateListen))
	if svcID == 0 {
		return
	}
	clientIP, clientPort := parseHostPortDefault(r.RemoteAddr)
	status := asInt(respMeta["status"])
	hash := flowHash(reqMeta, respMeta, svcID)

	flow := Flow{
		ID:           newFlowID(),
		ServiceID:    intPtr(svcID),
		Direction:    fmt.Sprintf("%s:%d -> gate:%d", clientIP, clientPort, listenPortFromAddr(a.cfg.GateListen)),
		RawRequest:   reqMeta,
		RawResponse:  respMeta,
		Hash:         hash,
		Stable:       a.isStable(hash),
		Checker:      false,
		Banned:       banned,
		ResponseCode: status,
		FlowID:       time.Now().UnixNano(),
		SrcIP:        clientIP,
		DstIP:        "gate",
		SrcPort:      clientPort,
		DstPort:      listenPortFromAddr(a.cfg.GateListen),
		Proto:        "tcp",
		PktCount:     1,
		BytesIn:      len(jsonString(reqMeta)),
		BytesOut:     len(jsonString(respMeta)),
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if err := a.insertFlow(flow); err == nil {
		a.enrichFlow(&flow)
		a.broadcastFlow(flow)
	}
}

func buildGarbageResponse(req map[string]any, upstreamStatus int) string {
	seed := sha256.Sum256([]byte(fmt.Sprintf("%d|%s|%s", time.Now().UnixNano(), asString(req["method"]), asString(req["uri"]))))
	head := hex.EncodeToString(seed[:])
	b := strings.Builder{}
	b.WriteString("FLAGMATE GARBAGE WALL\n")
	b.WriteString("upstream_status=")
	b.WriteString(strconv.Itoa(upstreamStatus))
	b.WriteString("\n")
	for i := 0; i < 90; i++ {
		tok := fakeFlagToken(head, i)
		noise := fakeNoise(head, i)
		b.WriteString(tok)
		b.WriteString(" :: ")
		b.WriteString(noise)
		b.WriteString("\n")
	}
	return b.String()
}

func (a *App) buildPoisonResponse(r *http.Request) ([]byte, string, bool) {
	if a.poisonMode() == "flag" {
		return []byte(a.currentFlagLine() + "\n"), "text/plain; charset=utf-8", false
	}
	if !isBrowserLike(r) {
		return []byte(a.currentFlagLine() + "\n"), "text/plain; charset=utf-8", false
	}
	key := clientRateKey(r)
	if !a.allowPoisonImage(key) {
		return []byte("Enough femboys for today\n"), "text/plain; charset=utf-8", true
	}
	images := a.poisonImages()
	if len(images) == 0 {
		return []byte("Idk why but femboy image is not loading\n"), "text/plain; charset=utf-8", false
	}
	path := images[rand.Intn(len(images))]
	body, err := os.ReadFile(path)
	if err != nil || len(body) == 0 {
		return []byte("Idk why but femboy image is not loading\n"), "text/plain; charset=utf-8", false
	}
	contentType := mediaContentType(path, body)
	return buildMediaPage(path, body, contentType), "text/html; charset=utf-8", false
}

func isBrowserLike(r *http.Request) bool {
	ua := strings.ToLower(r.Header.Get("User-Agent"))
	accept := strings.ToLower(r.Header.Get("Accept"))
	return strings.Contains(ua, "mozilla") || strings.Contains(accept, "text/html") || strings.Contains(accept, "image/")
}

func (a *App) currentFlagLine() string {
	minute := time.Now().Unix() / 60
	a.poisonMu.Lock()
	defer a.poisonMu.Unlock()
	if a.poisonFlagMinute == minute && a.poisonFlag != "" {
		return a.poisonFlag
	}
	a.poisonFlagMinute = minute
	a.poisonFlag = randomFlagLine()
	return a.poisonFlag
}

func randomFlagLine() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 32)
	for i := range b {
		b[i] = alphabet[rand.Intn(len(alphabet))]
	}
	return string(b)
}

func (a *App) allowPoisonImage(key string) bool {
	now := time.Now()
	cutoff := now.Add(-1 * time.Minute)
	a.poisonMu.Lock()
	defer a.poisonMu.Unlock()
	recent := a.poisonHits[key][:0]
	for _, t := range a.poisonHits[key] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) >= 10 {
		a.poisonHits[key] = recent
		return false
	}
	recent = append(recent, now)
	a.poisonHits[key] = recent
	return true
}

func clientRateKey(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		return strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (a *App) poisonImages() []string {
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".mp4": true, ".mov": true, ".mp3": true}
	out := []string{}
	_ = filepath.WalkDir(a.cfg.PoisonImageDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if allowed[strings.ToLower(filepath.Ext(path))] {
			out = append(out, path)
		}
		return nil
	})
	return out
}

func mediaContentType(path string, body []byte) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".mp3":
		return "audio/mpeg"
	default:
		return http.DetectContentType(body)
	}
}

func buildMediaPage(path string, body []byte, contentType string) []byte {
	dataURI := "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(body)
	name := html.EscapeString(filepath.Base(path))
	var media string
	if strings.HasPrefix(contentType, "video/") {
		media = `<video class="fit" src="` + dataURI + `" autoplay loop muted playsinline controls></video>`
	} else if strings.HasPrefix(contentType, "audio/") {
		media = `<div class="audio-wrap"><div class="title">` + name + `</div><audio src="` + dataURI + `" autoplay loop controls></audio></div>`
	} else {
		media = `<img class="fit" src="` + dataURI + `" alt="` + name + `" />`
	}
	page := `<!doctype html><html><head><meta charset="utf-8"><title>Flagmate</title><style>
html,body{margin:0;width:100%;height:100%;overflow:hidden;background:#000;}
.fit{position:fixed;inset:0;width:100vw;height:100vh;object-fit:contain;background:#000;}
.audio-wrap{width:100vw;height:100vh;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:24px;background:radial-gradient(circle,#1f2937,#000);color:#fff;font-family:monospace;}
.title{font-size:22px;opacity:.8;}
audio{width:min(720px,80vw);}
</style></head><body>` + media + `</body></html>`
	return []byte(page)
}

func fakeFlagToken(seed string, i int) string {
	base := sha256.Sum256([]byte(fmt.Sprintf("%s|flag|%d", seed, i)))
	raw := strings.ToUpper(hex.EncodeToString(base[:]))
	return fmt.Sprintf("FLAG{%s-%s-%s}", raw[:10], raw[10:20], raw[20:30])
}

func fakeNoise(seed string, i int) string {
	base := sha256.Sum256([]byte(fmt.Sprintf("%s|noise|%d", seed, i)))
	hexv := hex.EncodeToString(base[:])
	fragments := []string{
		"token=", "value=", "key=", "secret=", "proof=", "nonce=", "blob=",
	}
	parts := make([]string, 0, 6)
	for j := 0; j < 6; j++ {
		p := fragments[(i+j)%len(fragments)] + hexv[j*8:(j+1)*8]
		parts = append(parts, p)
	}
	return strings.Join(parts, "|")
}

func copyHeaders(dst, src http.Header) {
	for k, vals := range src {
		for _, v := range vals {
			dst.Add(k, v)
		}
	}
}

func listenPortFromAddr(addr string) int {
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		if strings.HasPrefix(addr, ":") {
			i, _ := strconv.Atoi(strings.TrimPrefix(addr, ":"))
			return i
		}
		return 0
	}
	i, _ := strconv.Atoi(p)
	return i
}

func parseHostPortDefault(hostport string) (string, int) {
	h, p, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport, 0
	}
	pi, _ := strconv.Atoi(p)
	return h, pi
}

func (a *App) readConn(conn net.Conn) {
	defer conn.Close()
	s := bufio.NewScanner(conn)
	buf := make([]byte, 0, 128*1024)
	s.Buffer(buf, 4*1024*1024)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" {
			continue
		}
		if err := a.handleEVE(line); err != nil {
			log.Printf("eve parse warning: %v", err)
		}
	}
}

func (a *App) handleEVE(raw string) error {
	var ev map[string]any
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		return err
	}
	if asString(ev["event_type"]) != "http" {
		a.forwardMirror(raw)
		return nil
	}

	srcIP := asString(ev["src_ip"])
	dstIP := asString(ev["dest_ip"])
	srcPort := asInt(ev["src_port"])
	dstPort := asInt(ev["dest_port"])
	proto := strings.ToLower(asString(ev["proto"]))
	if proto == "" {
		proto = "tcp"
	}

	svc, svcID := a.lookupService(srcPort, dstPort)
	if svc == nil {
		return nil
	}

	httpObj, _ := ev["http"].(map[string]any)
	rawReq := map[string]any{
		"method":     asString(httpObj["http_method"]),
		"uri":        asString(httpObj["url"]),
		"hostname":   asString(httpObj["hostname"]),
		"protocol":   asString(httpObj["protocol"]),
		"user_agent": asString(httpObj["http_user_agent"]),
	}
	if body := asString(httpObj["request_body_printable"]); body != "" {
		rawReq["body"] = body
	}

	status := asInt(httpObj["status"])
	rawResp := map[string]any{
		"status": status,
	}
	if length := asInt(httpObj["length"]); length > 0 {
		rawResp["length"] = length
	}
	if body := asString(httpObj["response_body_printable"]); body != "" {
		rawResp["body"] = body
	}

	banned := a.isBanned(rawReq, rawResp, status, svcID)
	flowHash := flowHash(rawReq, rawResp, svcID)
	stable := a.isStable(flowHash)

	flow := Flow{
		ID:           newFlowID(),
		ServiceID:    intPtr(svcID),
		Direction:    fmt.Sprintf("%s:%d -> %s:%d", srcIP, srcPort, dstIP, dstPort),
		RawRequest:   rawReq,
		RawResponse:  rawResp,
		Hash:         flowHash,
		Stable:       stable,
		Checker:      false,
		Banned:       banned,
		ResponseCode: status,
		FlowID:       asInt64(ev["flow_id"]),
		SrcIP:        srcIP,
		DstIP:        dstIP,
		SrcPort:      srcPort,
		DstPort:      dstPort,
		Proto:        proto,
		PktCount:     asInt(ev["pcap_cnt"]),
		BytesIn:      asInt(ev["bytes_toserver"]),
		BytesOut:     asInt(ev["bytes_toclient"]),
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	if err := a.insertFlow(flow); err != nil {
		return err
	}
	a.enrichFlow(&flow)
	a.broadcastFlow(flow)
	a.forwardMirror(raw)
	return nil
}

func (a *App) lookupService(srcPort, dstPort int) (*Service, int) {
	row := a.db.QueryRow(`SELECT id,name,port,protocol,created_at FROM services WHERE port = ? LIMIT 1`, dstPort)
	var s Service
	if err := row.Scan(&s.ID, &s.Name, &s.Port, &s.Protocol, &s.CreatedAt); err == nil {
		return &s, s.ID
	}
	row = a.db.QueryRow(`SELECT id,name,port,protocol,created_at FROM services WHERE port = ? LIMIT 1`, srcPort)
	if err := row.Scan(&s.ID, &s.Name, &s.Port, &s.Protocol, &s.CreatedAt); err == nil {
		return &s, s.ID
	}
	return nil, 0
}

func (a *App) isStable(hash string) bool {
	var count int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM flows WHERE hash = ?`, hash).Scan(&count)
	return count+1 >= a.cfg.StableN
}

func (a *App) isBanned(req, resp map[string]any, status int, serviceID int) bool {
	rows, err := a.db.Query(`SELECT id,pattern,mode FROM patterns WHERE active=1 AND service_id = ?`, serviceID)
	if err != nil {
		return false
	}
	defer rows.Close()

	reqText := strings.ToLower(jsonString(req))
	respText := strings.ToLower(jsonString(resp) + " " + strconv.Itoa(status))
	matchedIDs := []int{}

	for rows.Next() {
		var id int
		var p, mode string
		if err := rows.Scan(&id, &p, &mode); err != nil {
			continue
		}
		target := reqText + " " + respText
		switch strings.ToUpper(mode) {
		case "C":
			target = reqText
		case "S":
			target = respText
		case "B":
		}
		if patternMatch(strings.ToLower(p), target) {
			matchedIDs = append(matchedIDs, id)
		}
	}
	if len(matchedIDs) == 0 {
		return false
	}
	for _, id := range matchedIDs {
		_, _ = a.db.Exec(`UPDATE patterns SET match_count = match_count + 1 WHERE id = ?`, id)
	}
	return true
}

func patternMatch(pattern, target string) bool {
	re, err := regexp.Compile(pattern)
	if err == nil {
		return re.MatchString(target)
	}
	return strings.Contains(target, pattern)
}

func (a *App) insertFlow(f Flow) error {
	reqRaw, _ := json.Marshal(f.RawRequest)
	respRaw, _ := json.Marshal(f.RawResponse)
	reqHash, reqStore := a.payloadRef(reqRaw)
	respHash, respStore := a.payloadRef(respRaw)
	_, err := a.db.Exec(`INSERT INTO flows (id,service_id,direction,start_ts,end_ts,raw_request,raw_response,hash,stable,checker,banned,response_code,flow_id,src_ip,dst_ip,src_port,dst_port,proto,pkt_count,bytes_in,bytes_out,created_at,req_hash,resp_hash) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		f.ID, intPtrToAny(f.ServiceID), f.Direction, f.StartTS, f.EndTS, reqStore, respStore, f.Hash, boolInt(f.Stable), boolInt(f.Checker), boolInt(f.Banned), f.ResponseCode, f.FlowID, f.SrcIP, f.DstIP, f.SrcPort, f.DstPort, f.Proto, f.PktCount, f.BytesIn, f.BytesOut, f.CreatedAt, reqHash, respHash)
	return err
}

func (a *App) payloadRef(raw []byte) (string, string) {
	h := sha256.Sum256(raw)
	key := hex.EncodeToString(h[:])
	_, _ = a.db.Exec(`INSERT OR IGNORE INTO flow_payloads(hash, payload, bytes, created_at) VALUES (?, ?, ?, ?)`, key, string(raw), len(raw), time.Now().UTC().Format(time.RFC3339))
	return key, `{"_payload_ref":"` + key + `"}`
}

func (a *App) hydrateFlowPayloads(f *Flow) {
	f.RawRequest = a.hydratePayloadMap(f.RawRequest)
	f.RawResponse = a.hydratePayloadMap(f.RawResponse)
}

func (a *App) hydratePayloadMap(src map[string]any) map[string]any {
	ref := asString(src["_payload_ref"])
	if ref == "" {
		return src
	}
	var raw string
	if err := a.db.QueryRow(`SELECT payload FROM flow_payloads WHERE hash = ?`, ref).Scan(&raw); err != nil {
		return src
	}
	return parseJSONMap(raw)
}

func flowHash(req, resp map[string]any, serviceID int) string {
	base := fmt.Sprintf("%d|%s|%s|%v", serviceID, asString(req["method"]), asString(req["uri"]), resp["status"])
	h := sha256.Sum256([]byte(base))
	return hex.EncodeToString(h[:])
}

func (a *App) enrichFlow(f *Flow) {
	path := asString(f.RawRequest["uri"])
	if path == "" {
		path = asString(f.RawRequest["url"])
	}
	if path != "" {
		f.Destination = fmt.Sprintf("%s:%d%s", f.DstIP, f.DstPort, path)
	} else {
		f.Destination = fmt.Sprintf("%s:%d", f.DstIP, f.DstPort)
	}
	pct, avg := a.stabilityMetrics(f.Hash)
	f.StabilityPct = pct
	f.AvgInterval = avg
	f.Stable = pct >= 70
	f.Mirrored = a.isMirroredGroup(f.Hash)
	f.GroupName = a.groupName(f.Hash)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM flows WHERE hash = ?`, f.Hash).Scan(&f.GroupCount)
}

func (a *App) groupName(hash string) string {
	var name string
	if err := a.db.QueryRow(`SELECT name FROM flow_group_meta WHERE hash = ?`, hash).Scan(&name); err == nil && strings.TrimSpace(name) != "" {
		return name
	}
	if err := a.db.QueryRow(`SELECT name FROM mirror_groups WHERE hash = ?`, hash).Scan(&name); err == nil && strings.TrimSpace(name) != "" {
		return name
	}
	return ""
}

func (a *App) isMirroredGroup(hash string) bool {
	var enabled int
	if err := a.db.QueryRow(`SELECT enabled FROM mirror_groups WHERE hash = ?`, hash).Scan(&enabled); err != nil {
		return false
	}
	return enabled == 1
}

func (a *App) stabilityMetrics(hash string) (int, float64) {
	rows, err := a.db.Query(`SELECT created_at FROM flows WHERE hash = ? ORDER BY created_at ASC LIMIT 250`, hash)
	if err != nil {
		return 0, 0
	}
	defer rows.Close()
	times := []time.Time{}
	for rows.Next() {
		var raw string
		if rows.Scan(&raw) != nil {
			continue
		}
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			times = append(times, t)
		} else if t, err := time.Parse("2006-01-02 15:04:05", raw); err == nil {
			times = append(times, t)
		}
	}
	if len(times) < 2 {
		return 0, 0
	}
	intervals := make([]float64, 0, len(times)-1)
	for i := 1; i < len(times); i++ {
		d := times[i].Sub(times[i-1]).Seconds()
		if d > 0 {
			intervals = append(intervals, d)
		}
	}
	if len(intervals) == 0 {
		return 0, 0
	}
	avg := 0.0
	for _, v := range intervals {
		avg += v
	}
	avg /= float64(len(intervals))
	if avg <= 0 {
		return 0, 0
	}
	variance := 0.0
	for _, v := range intervals {
		delta := v - avg
		variance += delta * delta
	}
	std := math.Sqrt(variance / float64(len(intervals)))
	variation := std / avg
	regularity := 1 - math.Min(1, variation)
	volumeBoost := math.Min(1, float64(len(intervals))/10)
	pct := int(math.Round(100 * regularity * volumeBoost))
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return pct, avg
}

func newFlowID() string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid())))
	s := hex.EncodeToString(h[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", s[:8], s[8:12], s[12:16], s[16:20], s[20:32])
}

func (a *App) listServices(w http.ResponseWriter, _ *http.Request) {
	rows, err := a.db.Query(`SELECT id,name,port,protocol,created_at FROM services ORDER BY id DESC`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()
	out := []Service{}
	for rows.Next() {
		var s Service
		if err := rows.Scan(&s.ID, &s.Name, &s.Port, &s.Protocol, &s.CreatedAt); err == nil {
			out = append(out, s)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) createService(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name     string `json:"name"`
		Port     int    `json:"port"`
		Protocol string `json:"protocol"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}
	if in.Name == "" || in.Port < 1 || in.Port > 65535 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid service"})
		return
	}
	if in.Protocol == "" {
		in.Protocol = "tcp"
	}
	_, err := a.db.Exec(`INSERT INTO services(name,port,protocol,created_at) VALUES (?,?,?,?)`, in.Name, in.Port, strings.ToLower(in.Protocol), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (a *App) deleteService(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_, err := a.db.Exec(`DELETE FROM services WHERE id = ?`, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) listPatterns(w http.ResponseWriter, r *http.Request) {
	serviceID := parseInt(r.URL.Query().Get("service_id"), -1)
	query := `SELECT id,service_id,pattern,description,mode,active,match_count,created_at FROM patterns`
	args := []any{}
	if serviceID >= 0 {
		query += ` WHERE service_id = ?`
		args = append(args, serviceID)
	}
	query += ` ORDER BY id DESC`
	rows, err := a.db.Query(query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()
	out := []Pattern{}
	for rows.Next() {
		var p Pattern
		var active int
		var sid sql.NullInt64
		if err := rows.Scan(&p.ID, &sid, &p.Pattern, &p.Description, &p.Mode, &active, &p.MatchCount, &p.CreatedAt); err == nil {
			if sid.Valid {
				p.ServiceID = intPtr(int(sid.Int64))
			}
			p.Active = active == 1
			out = append(out, p)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) createPattern(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ServiceID   *int   `json:"service_id"`
		Pattern     string `json:"pattern"`
		Description string `json:"description"`
		Mode        string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}
	in.Pattern = strings.TrimSpace(in.Pattern)
	if in.Pattern == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "pattern required"})
		return
	}
	mode := strings.ToUpper(strings.TrimSpace(in.Mode))
	if mode == "" {
		mode = "B"
	}
	if mode != "B" && mode != "C" && mode != "S" {
		mode = "B"
	}
	_, err := a.db.Exec(`INSERT INTO patterns(service_id,pattern,description,mode,active,created_at) VALUES (?,?,?,?,?,?)`, intPtrToAny(in.ServiceID), in.Pattern, in.Description, mode, 1, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		res, updateErr := a.db.Exec(`UPDATE patterns SET service_id = ?, description = ?, mode = ?, active = 1 WHERE pattern = ?`, intPtrToAny(in.ServiceID), in.Description, mode, in.Pattern)
		if updateErr != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		if changed, _ := res.RowsAffected(); changed == 0 {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
	}
	a.recalculateAllFlowBans()
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (a *App) deletePattern(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_, err := a.db.Exec(`DELETE FROM patterns WHERE id = ?`, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	a.recalculateAllFlowBans()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) togglePattern(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var in struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}
	_, err := a.db.Exec(`UPDATE patterns SET active = ? WHERE id = ?`, boolInt(in.Active), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	a.recalculateAllFlowBans()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) listFlows(w http.ResponseWriter, r *http.Request) {
	page := max(1, parseInt(r.URL.Query().Get("page"), 1))
	size := max(1, min(500, parseInt(r.URL.Query().Get("size"), 50)))
	offset := (page - 1) * size
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	serviceID := parseInt(r.URL.Query().Get("service_id"), -1)
	bannedFilter := strings.TrimSpace(r.URL.Query().Get("banned"))
	checkerFilter := strings.TrimSpace(r.URL.Query().Get("checker"))
	collapse := boolQueryInt(r.URL.Query().Get("collapse")) == 1

	query := `SELECT id,service_id,direction,start_ts,end_ts,raw_request,raw_response,hash,stable,checker,banned,response_code,flow_id,src_ip,dst_ip,src_port,dst_port,proto,pkt_count,bytes_in,bytes_out,created_at FROM flows`
	args := []any{}
	where := []string{}
	if serviceID >= 0 {
		where = append(where, `service_id = ?`)
		args = append(args, serviceID)
	}
	if bannedFilter != "" {
		where = append(where, `banned = ?`)
		args = append(args, boolQueryInt(bannedFilter))
	}
	if checkerFilter != "" {
		where = append(where, `checker = ?`)
		args = append(args, boolQueryInt(checkerFilter))
	}
	if search != "" {
		where = append(where, `(direction LIKE ? OR created_at LIKE ? OR proto LIKE ? OR src_ip LIKE ? OR dst_ip LIKE ? OR CAST(src_port AS TEXT) LIKE ? OR CAST(dst_port AS TEXT) LIKE ? OR CAST(response_code AS TEXT) LIKE ? OR raw_request LIKE ? OR raw_response LIKE ? OR req_hash IN (SELECT hash FROM flow_payloads WHERE payload LIKE ?) OR resp_hash IN (SELECT hash FROM flow_payloads WHERE payload LIKE ?))`)
		needle := "%" + search + "%"
		args = append(args, needle, needle, needle, needle, needle, needle, needle, needle, needle, needle, needle, needle)
	}
	if len(where) > 0 {
		query += ` WHERE ` + strings.Join(where, ` AND `)
	}
	if collapse && search == "" {
		query += ` AND created_at = (SELECT MAX(f2.created_at) FROM flows f2 WHERE f2.hash = flows.hash)`
		if len(where) == 0 {
			query = strings.Replace(query, ` FROM flows AND `, ` FROM flows WHERE `, 1)
		}
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, size, offset)

	rows, err := a.db.Query(query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	flows := []Flow{}
	for rows.Next() {
		f, err := scanFlow(rows)
		if err == nil {
			a.hydrateFlowPayloads(&f)
			a.enrichFlow(&f)
			flows = append(flows, f)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"flows": flows})
}

func (a *App) getFlow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	row := a.db.QueryRow(`SELECT id,service_id,direction,start_ts,end_ts,raw_request,raw_response,hash,stable,checker,banned,response_code,flow_id,src_ip,dst_ip,src_port,dst_port,proto,pkt_count,bytes_in,bytes_out,created_at FROM flows WHERE id = ?`, id)
	f, err := scanFlowRow(row)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	a.hydrateFlowPayloads(&f)
	a.enrichFlow(&f)
	writeJSON(w, http.StatusOK, f)
}

func (a *App) flowHistory(w http.ResponseWriter, r *http.Request) {
	hash := strings.TrimSpace(r.URL.Query().Get("hash"))
	if hash == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "hash required"})
		return
	}
	limit := min(100, max(1, parseInt(r.URL.Query().Get("limit"), 100)))
	offset := max(0, parseInt(r.URL.Query().Get("offset"), 0))
	rows, err := a.db.Query(`SELECT id,service_id,direction,start_ts,end_ts,raw_request,raw_response,hash,stable,checker,banned,response_code,flow_id,src_ip,dst_ip,src_port,dst_port,proto,pkt_count,bytes_in,bytes_out,created_at FROM flows WHERE hash = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`, hash, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()
	out := []Flow{}
	for rows.Next() {
		f, err := scanFlow(rows)
		if err == nil {
			a.hydrateFlowPayloads(&f)
			a.enrichFlow(&f)
			out = append(out, f)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) labelFlow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var in struct {
		Checker bool `json:"checker"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}
	var err error
	if in.Checker {
		_, err = a.db.Exec(`UPDATE flows SET checker = 1, banned = 0 WHERE id = ?`, id)
	} else {
		_, err = a.db.Exec(`UPDATE flows SET checker = 0 WHERE id = ?`, id)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if in.Checker {
		a.recalculateAllFlowBans()
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) mirrorFlowGroup(w http.ResponseWriter, r *http.Request) {
	flow, err := a.flowByID(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "flow not found"})
		return
	}
	var in struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}
	if flow.ServiceID == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "flow has no service"})
		return
	}
	if in.Enabled {
		_, err = a.db.Exec(`INSERT INTO mirror_groups(hash, service_id, enabled, created_at) VALUES (?, ?, 1, ?) ON CONFLICT(hash) DO UPDATE SET service_id=excluded.service_id, enabled=1`, flow.Hash, *flow.ServiceID, time.Now().UTC().Format(time.RFC3339))
	} else {
		_, err = a.db.Exec(`UPDATE mirror_groups SET enabled = 0 WHERE hash = ?`, flow.Hash)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "hash": flow.Hash, "enabled": in.Enabled})
}

func (a *App) unbanFlow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_, err := a.db.Exec(`UPDATE flows SET banned = 0 WHERE id = ?`, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	a.recalculateAllFlowBans()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) matchingPatternsForFlow(w http.ResponseWriter, r *http.Request) {
	flow, err := a.flowByID(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "flow not found"})
		return
	}
	out := a.matchingPatterns(flow)
	writeJSON(w, http.StatusOK, out)
}

func (a *App) removeMatchingPatternsForFlow(w http.ResponseWriter, r *http.Request) {
	flow, err := a.flowByID(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "flow not found"})
		return
	}
	patterns := a.matchingPatterns(flow)
	for _, p := range patterns {
		_, _ = a.db.Exec(`DELETE FROM patterns WHERE id = ?`, p.ID)
	}
	_, _ = a.db.Exec(`UPDATE flows SET banned = 0 WHERE id = ?`, flow.ID)
	a.recalculateAllFlowBans()
	writeJSON(w, http.StatusOK, map[string]any{"removed": patterns})
}

func (a *App) flowByID(id string) (Flow, error) {
	row := a.db.QueryRow(`SELECT id,service_id,direction,start_ts,end_ts,raw_request,raw_response,hash,stable,checker,banned,response_code,flow_id,src_ip,dst_ip,src_port,dst_port,proto,pkt_count,bytes_in,bytes_out,created_at FROM flows WHERE id = ?`, id)
	f, err := scanFlowRow(row)
	if err != nil {
		return f, err
	}
	a.hydrateFlowPayloads(&f)
	a.enrichFlow(&f)
	return f, nil
}

func (a *App) matchingPatterns(flow Flow) []Pattern {
	if flow.ServiceID == nil {
		return []Pattern{}
	}
	rows, err := a.db.Query(`SELECT id,service_id,pattern,description,mode,active,match_count,created_at FROM patterns WHERE service_id = ?`, *flow.ServiceID)
	if err != nil {
		return []Pattern{}
	}
	defer rows.Close()
	reqText := strings.ToLower(jsonString(flow.RawRequest))
	respText := strings.ToLower(jsonString(flow.RawResponse) + " " + strconv.Itoa(flow.ResponseCode))
	out := []Pattern{}
	for rows.Next() {
		var p Pattern
		var active int
		var sid sql.NullInt64
		if rows.Scan(&p.ID, &sid, &p.Pattern, &p.Description, &p.Mode, &active, &p.MatchCount, &p.CreatedAt) != nil {
			continue
		}
		if sid.Valid {
			p.ServiceID = intPtr(int(sid.Int64))
		}
		p.Active = active == 1
		target := reqText + " " + respText
		switch strings.ToUpper(p.Mode) {
		case "C":
			target = reqText
		case "S":
			target = respText
		}
		if patternMatch(strings.ToLower(p.Pattern), target) {
			out = append(out, p)
		}
	}
	return out
}

func (a *App) recalculateAllFlowBans() {
	rows, err := a.db.Query(`SELECT id,service_id,direction,start_ts,end_ts,raw_request,raw_response,hash,stable,checker,banned,response_code,flow_id,src_ip,dst_ip,src_port,dst_port,proto,pkt_count,bytes_in,bytes_out,created_at FROM flows`)
	if err != nil {
		log.Printf("recalculate banned flows query error: %v", err)
		return
	}
	updates := map[string]bool{}
	for rows.Next() {
		flow, err := scanFlow(rows)
		if err != nil {
			continue
		}
		a.hydrateFlowPayloads(&flow)
		shouldBan := !flow.Checker && len(a.matchingPatterns(flow)) > 0
		if shouldBan != flow.Banned {
			updates[flow.ID] = shouldBan
		}
	}
	_ = rows.Close()
	for id, shouldBan := range updates {
		if _, err := a.db.Exec(`UPDATE flows SET banned = ? WHERE id = ?`, boolInt(shouldBan), id); err != nil {
			log.Printf("recalculate banned flow update error: %v", err)
		}
	}
}

func (a *App) uniqueWords(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	row := a.db.QueryRow(`SELECT raw_request, raw_response FROM flows WHERE id = ?`, id)
	var reqRaw, respRaw string
	if err := row.Scan(&reqRaw, &respRaw); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "flow not found"})
		return
	}
	base := extractWords(jsonString(a.hydratePayloadMap(parseJSONMap(reqRaw))) + " " + jsonString(a.hydratePayloadMap(parseJSONMap(respRaw))))
	checkerRows, _ := a.db.Query(`SELECT raw_request, raw_response FROM flows WHERE checker = 1 ORDER BY created_at DESC LIMIT 1000`)
	checkerSet := map[string]struct{}{}
	if checkerRows != nil {
		defer checkerRows.Close()
		for checkerRows.Next() {
			var cr, cs string
			if checkerRows.Scan(&cr, &cs) == nil {
				for _, w := range extractWords(jsonString(a.hydratePayloadMap(parseJSONMap(cr))) + " " + jsonString(a.hydratePayloadMap(parseJSONMap(cs)))) {
					checkerSet[w] = struct{}{}
				}
			}
		}
	}
	out := []string{}
	for _, wv := range base {
		if _, ok := checkerSet[wv]; !ok {
			out = append(out, wv)
		}
	}
	sort.Strings(out)
	if len(out) > 500 {
		out = out[:500]
	}
	writeJSON(w, http.StatusOK, map[string]any{"words": out})
}

func extractWords(src string) []string {
	re := regexp.MustCompile(`[A-Za-z0-9_\-/\.]{4,64}`)
	all := re.FindAllString(src, -1)
	set := map[string]struct{}{}
	for _, s := range all {
		s = strings.TrimSpace(strings.ToLower(s))
		if len(s) < 4 {
			continue
		}
		set[s] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	return out
}

func (a *App) flowGroups(w http.ResponseWriter, r *http.Request) {
	top := min(200, max(1, parseInt(r.URL.Query().Get("top"), 20)))
	rows, err := a.db.Query(`SELECT f.hash, COUNT(*) as cnt, MIN(f.created_at) as first_seen, MAX(f.created_at) as last_seen FROM flows f GROUP BY f.hash ORDER BY cnt DESC LIMIT ?`, top)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()
	out := []FlowGroupMeta{}
	for rows.Next() {
		var hash, first, last string
		var cnt int
		if rows.Scan(&hash, &cnt, &first, &last) == nil {
			if meta, ok := a.flowGroupMeta(hash, cnt, first, last, true); ok {
				out = append(out, meta)
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) flowGroupMeta(hash string, cnt int, first, last string, includeFlow bool) (FlowGroupMeta, bool) {
	row := a.db.QueryRow(`SELECT id,service_id,direction,start_ts,end_ts,raw_request,raw_response,hash,stable,checker,banned,response_code,flow_id,src_ip,dst_ip,src_port,dst_port,proto,pkt_count,bytes_in,bytes_out,created_at FROM flows WHERE hash = ? ORDER BY created_at DESC LIMIT 1`, hash)
	flow, err := scanFlowRow(row)
	if err != nil {
		return FlowGroupMeta{}, false
	}
	a.hydrateFlowPayloads(&flow)
	a.enrichFlow(&flow)
	var name string
	var checker int
	_ = a.db.QueryRow(`SELECT name, checker FROM flow_group_meta WHERE hash = ?`, hash).Scan(&name, &checker)
	if name == "" {
		name = flow.GroupName
	}
	meta := FlowGroupMeta{
		Hash:          hash,
		Name:          name,
		Checker:       checker == 1,
		Count:         cnt,
		ExampleFlowID: flow.ID,
		FirstSeen:     first,
		LastSeen:      last,
		Destination:   flow.Destination,
		Method:        asString(flow.RawRequest["method"]),
		URI:           asString(flow.RawRequest["uri"]),
		ResponseCode:  flow.ResponseCode,
		ServiceID:     flow.ServiceID,
		Mirrored:      flow.Mirrored,
	}
	if meta.URI == "" {
		meta.URI = asString(flow.RawRequest["url"])
	}
	if includeFlow {
		meta.LatestFlow = &flow
	}
	return meta, true
}

func (a *App) renameFlowGroup(w http.ResponseWriter, r *http.Request) {
	hash := chi.URLParam(r, "hash")
	var in struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}
	name := strings.TrimSpace(in.Name)
	_, err := a.db.Exec(`INSERT INTO flow_group_meta(hash, name, updated_at) VALUES (?, ?, ?) ON CONFLICT(hash) DO UPDATE SET name=excluded.name, updated_at=excluded.updated_at`, hash, name, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	_, _ = a.db.Exec(`UPDATE mirror_groups SET name = ? WHERE hash = ?`, name, hash)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) markFlowGroupChecker(w http.ResponseWriter, r *http.Request) {
	hash := chi.URLParam(r, "hash")
	var in struct {
		Checker bool `json:"checker"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}
	_, err := a.db.Exec(`INSERT INTO flow_group_meta(hash, checker, updated_at) VALUES (?, ?, ?) ON CONFLICT(hash) DO UPDATE SET checker=excluded.checker, updated_at=excluded.updated_at`, hash, boolInt(in.Checker), time.Now().UTC().Format(time.RFC3339))
	if err == nil {
		_, err = a.db.Exec(`UPDATE flows SET checker = ?, banned = CASE WHEN ? = 1 THEN 0 ELSE banned END WHERE hash = ?`, boolInt(in.Checker), boolInt(in.Checker), hash)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if in.Checker {
		a.recalculateAllFlowBans()
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) loadMirroring() {
	row := a.db.QueryRow(`SELECT enabled, targets, services FROM mirroring WHERE id = 1`)
	var enabled int
	var targets string
	var services string
	if err := row.Scan(&enabled, &targets, &services); err != nil {
		return
	}
	cfg := MirroringConfig{Enabled: enabled == 1, Targets: []MirrorTarget{}, Services: []ServiceMirrorConfig{}}
	_ = json.Unmarshal([]byte(targets), &cfg.Targets)
	_ = json.Unmarshal([]byte(services), &cfg.Services)
	a.mirrorMu.Lock()
	a.mirroring = cfg
	a.mirrorMu.Unlock()
}

func (a *App) getMirroring(w http.ResponseWriter, _ *http.Request) {
	a.mirrorMu.RLock()
	cfg := a.mirroring
	a.mirrorMu.RUnlock()
	writeJSON(w, http.StatusOK, cfg)
}

func (a *App) mirroredGroups(w http.ResponseWriter, _ *http.Request) {
	rows, err := a.db.Query(`SELECT mg.hash, mg.service_id, COUNT(f.id), MIN(f.created_at), MAX(f.created_at) FROM mirror_groups mg LEFT JOIN flows f ON f.hash = mg.hash WHERE mg.enabled = 1 GROUP BY mg.hash, mg.service_id ORDER BY mg.service_id ASC, MAX(f.created_at) DESC`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()
	out := []FlowGroupMeta{}
	for rows.Next() {
		var hash string
		var serviceID int
		var cnt int
		var first, last sql.NullString
		if rows.Scan(&hash, &serviceID, &cnt, &first, &last) != nil {
			continue
		}
		meta, ok := a.flowGroupMeta(hash, cnt, first.String, last.String, true)
		if !ok {
			continue
		}
		if meta.ServiceID == nil {
			meta.ServiceID = intPtr(serviceID)
		}
		out = append(out, meta)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) mirroringAttempts(w http.ResponseWriter, r *http.Request) {
	hash := strings.TrimSpace(r.URL.Query().Get("hash"))
	targetIP := strings.TrimSpace(r.URL.Query().Get("target_ip"))
	if hash == "" || targetIP == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "hash and target_ip required"})
		return
	}
	limit := min(200, max(1, parseInt(r.URL.Query().Get("limit"), 50)))
	rows, err := a.db.Query(`SELECT id,service_id,hash,flow_id,target_ip,target_port,success,flag,response,created_at FROM mirror_attempts WHERE hash = ? AND target_ip = ? ORDER BY created_at DESC LIMIT ?`, hash, targetIP, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, serviceID, port, success int
		var hash, flowID, ip, flag, response, createdAt string
		if rows.Scan(&id, &serviceID, &hash, &flowID, &ip, &port, &success, &flag, &response, &createdAt) == nil {
			out = append(out, map[string]any{"id": id, "service_id": serviceID, "hash": hash, "flow_id": flowID, "target_ip": ip, "target_port": port, "success": success == 1, "flag": flag, "response": response, "created_at": createdAt})
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) mirroringStats(w http.ResponseWriter, _ *http.Request) {
	rows, err := a.db.Query(`SELECT service_id, hash, target_ip, success, flag, created_at FROM mirror_attempts ORDER BY created_at ASC`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()
	attempts := []MirrorAttemptStat{}
	for rows.Next() {
		var serviceID, success int
		var hash, ip, flag, created string
		if rows.Scan(&serviceID, &hash, &ip, &success, &flag, &created) != nil {
			continue
		}
		t, _ := time.Parse(time.RFC3339, created)
		attempts = append(attempts, MirrorAttemptStat{ServiceID: serviceID, Hash: hash, TargetIP: ip, Success: success == 1, Flag: flag, CreatedAt: t})
	}
	teamStats := map[string]map[string]any{}
	groupStats := map[string]map[string]any{}
	uniqueFlags := map[string]struct{}{}
	total := len(attempts)
	successes := 0
	for _, aitem := range attempts {
		if _, ok := teamStats[aitem.TargetIP]; !ok {
			teamStats[aitem.TargetIP] = map[string]any{"target_ip": aitem.TargetIP, "requests": 0, "successes": 0, "flags": 0}
		}
		if _, ok := groupStats[aitem.Hash]; !ok {
			groupStats[aitem.Hash] = map[string]any{"hash": aitem.Hash, "requests": 0, "successes": 0, "flags": 0, "name": a.groupName(aitem.Hash)}
		}
		incStat(teamStats[aitem.TargetIP], "requests")
		incStat(groupStats[aitem.Hash], "requests")
		if aitem.Success {
			successes++
			incStat(teamStats[aitem.TargetIP], "successes")
			incStat(groupStats[aitem.Hash], "successes")
		}
		if aitem.Flag != "" {
			key := aitem.TargetIP + "|" + aitem.Flag
			if _, ok := uniqueFlags[key]; !ok {
				uniqueFlags[key] = struct{}{}
				incStat(teamStats[aitem.TargetIP], "flags")
				incStat(groupStats[aitem.Hash], "flags")
			}
		}
	}
	teams := mapValuesWithRate(teamStats)
	groups := mapValuesWithRate(groupStats)
	writeJSON(w, http.StatusOK, map[string]any{
		"total_requests": total,
		"successes":      successes,
		"success_rate":   percent(successes, total),
		"flags":          len(uniqueFlags),
		"teams":          teams,
		"groups":         groups,
		"series": map[string]any{
			"minute": bucketAttempts(attempts, time.Minute),
			"10m":    bucketAttempts(attempts, 10*time.Minute),
			"30m":    bucketAttempts(attempts, 30*time.Minute),
			"hour":   bucketAttempts(attempts, time.Hour),
		},
	})
}

func incStat(m map[string]any, key string) { m[key] = asInt(m[key]) + 1 }

func mapValuesWithRate(src map[string]map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(src))
	for _, item := range src {
		item["success_rate"] = percent(asInt(item["successes"]), asInt(item["requests"]))
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return asInt(out[i]["flags"]) > asInt(out[j]["flags"]) })
	return out
}

func bucketAttempts(attempts []MirrorAttemptStat, step time.Duration) []map[string]any {
	if len(attempts) == 0 {
		return []map[string]any{}
	}
	type bucket struct{ Requests, Successes, Flags int }
	buckets := map[int64]*bucket{}
	seenFlags := map[string]struct{}{}
	for _, item := range attempts {
		if item.CreatedAt.IsZero() {
			continue
		}
		key := item.CreatedAt.Truncate(step).Unix()
		b := buckets[key]
		if b == nil {
			b = &bucket{}
			buckets[key] = b
		}
		b.Requests++
		if item.Success {
			b.Successes++
		}
		if item.Flag != "" {
			flagKey := strconv.FormatInt(key, 10) + "|" + item.TargetIP + "|" + item.Flag
			if _, ok := seenFlags[flagKey]; !ok {
				seenFlags[flagKey] = struct{}{}
				b.Flags++
			}
		}
	}
	keys := make([]int64, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	out := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		b := buckets[key]
		out = append(out, map[string]any{"ts": time.Unix(key, 0).UTC().Format(time.RFC3339), "requests": b.Requests, "successes": b.Successes, "flags": b.Flags, "success_rate": percent(b.Successes, b.Requests)})
	}
	if len(out) > 60 {
		out = out[len(out)-60:]
	}
	return out
}

func percent(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return math.Round((float64(part)/float64(total))*1000) / 10
}

func (a *App) setMirroring(w http.ResponseWriter, r *http.Request) {
	var cfg MirroringConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}
	b, _ := json.Marshal(cfg.Targets)
	sb, _ := json.Marshal(cfg.Services)
	_, err := a.db.Exec(`UPDATE mirroring SET enabled = ?, targets = ?, services = ? WHERE id = 1`, boolInt(cfg.Enabled), string(b), string(sb))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	a.mirrorMu.Lock()
	a.mirroring = cfg
	a.mirrorMu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) getSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"poison_mode": a.poisonMode()})
}

func (a *App) setSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PoisonMode string `json:"poison_mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}
	mode := strings.ToLower(strings.TrimSpace(in.PoisonMode))
	if mode != "media" && mode != "flag" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid poison_mode"})
		return
	}
	_, err := a.db.Exec(`INSERT INTO settings(key, value) VALUES ('poison_mode', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, mode)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "poison_mode": mode})
}

func (a *App) poisonMode() string {
	var mode string
	if err := a.db.QueryRow(`SELECT value FROM settings WHERE key = 'poison_mode'`).Scan(&mode); err != nil {
		return "media"
	}
	if mode != "flag" && mode != "media" {
		return "media"
	}
	return mode
}

func (a *App) forwardMirror(raw string) {
	a.mirrorMu.RLock()
	cfg := a.mirroring
	a.mirrorMu.RUnlock()
	if !cfg.Enabled || len(cfg.Targets) == 0 {
		return
	}
	for _, t := range cfg.Targets {
		go func(target MirrorTarget) {
			addr := net.JoinHostPort(target.IP, strconv.Itoa(target.Port))
			conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
			if err != nil {
				return
			}
			defer conn.Close()
			_ = conn.SetWriteDeadline(time.Now().Add(300 * time.Millisecond))
			_, _ = conn.Write([]byte(raw + "\n"))
		}(t)
	}
}

func (a *App) startMirrorScheduler(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.runMirrorTick()
		}
	}
}

func (a *App) runMirrorTick() {
	a.mirrorMu.RLock()
	cfg := a.mirroring
	a.mirrorMu.RUnlock()
	if !cfg.Enabled {
		return
	}
	now := time.Now()
	for _, svc := range cfg.Services {
		if svc.ServiceID == 0 || len(cfg.Targets) == 0 {
			continue
		}
		interval := svc.IntervalSeconds
		if interval < 1 {
			interval = 60
		}
		if due, ok := a.mirrorDue[svc.ServiceID]; ok && now.Before(due) {
			continue
		}
		a.mirrorDue[svc.ServiceID] = now.Add(time.Duration(interval) * time.Second)
		go a.mirrorMarkedServiceGroups(svc, cfg.Targets)
	}
}

func (a *App) mirrorMarkedServiceGroups(cfg ServiceMirrorConfig, targets []MirrorTarget) {
	rows, err := a.db.Query(`SELECT hash FROM mirror_groups WHERE service_id = ? AND enabled = 1`, cfg.ServiceID)
	if err != nil {
		log.Printf("mirror groups query error: %v", err)
		return
	}
	hashes := []string{}
	for rows.Next() {
		var hash string
		if rows.Scan(&hash) == nil {
			hashes = append(hashes, hash)
		}
	}
	_ = rows.Close()
	servicePort := a.servicePort(cfg.ServiceID)
	if servicePort < 1 {
		return
	}
	for _, hash := range hashes {
		row := a.db.QueryRow(`SELECT id,service_id,direction,start_ts,end_ts,raw_request,raw_response,hash,stable,checker,banned,response_code,flow_id,src_ip,dst_ip,src_port,dst_port,proto,pkt_count,bytes_in,bytes_out,created_at FROM flows WHERE hash = ? ORDER BY created_at DESC LIMIT 1`, hash)
		flow, err := scanFlowRow(row)
		if err != nil {
			continue
		}
		a.hydrateFlowPayloads(&flow)
		a.enrichFlow(&flow)
		payload, _ := json.Marshal(map[string]any{"type": "flagmate_mirror", "service_id": cfg.ServiceID, "hash": hash, "flow": flow})
		for _, target := range targets {
			target.Port = servicePort
			success, flag, response := a.sendMirrorPayload(target, string(payload)+"\n")
			a.recordMirrorAttempt(cfg.ServiceID, hash, flow.ID, target, success, flag, response)
		}
	}
}

func (a *App) servicePort(serviceID int) int {
	var port int
	if err := a.db.QueryRow(`SELECT port FROM services WHERE id = ?`, serviceID).Scan(&port); err != nil {
		return 0
	}
	return port
}

func (a *App) sendMirrorPayload(target MirrorTarget, payload string) (bool, string, string) {
	addr := net.JoinHostPort(target.IP, strconv.Itoa(target.Port))
	conn, err := net.DialTimeout("tcp", addr, 700*time.Millisecond)
	if err != nil {
		return false, "", err.Error()
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(700 * time.Millisecond))
	if _, err := conn.Write([]byte(payload)); err != nil {
		return false, "", err.Error()
	}
	_ = conn.SetReadDeadline(time.Now().Add(1200 * time.Millisecond))
	buf := make([]byte, 8192)
	n, _ := conn.Read(buf)
	response := ""
	if n > 0 {
		response = string(buf[:n])
	}
	flag := extractFlag(response)
	return flag != "", flag, response
}

func (a *App) recordMirrorAttempt(serviceID int, hash, flowID string, target MirrorTarget, success bool, flag, response string) {
	if len(response) > 4096 {
		response = response[:4096]
	}
	_, _ = a.db.Exec(`INSERT INTO mirror_attempts(service_id,hash,flow_id,target_ip,target_port,success,flag,response,created_at) VALUES (?,?,?,?,?,?,?,?,?)`, serviceID, hash, flowID, target.IP, target.Port, boolInt(success), flag, response, time.Now().UTC().Format(time.RFC3339))
}

func extractFlag(src string) string {
	re := regexp.MustCompile(`(?i)([a-z0-9_]+\{[^\s{}]{4,128}\}|flag\{[^\s{}]{4,128}\}|test\{[^\s{}]{4,128}\})`)
	match := re.FindString(src)
	return strings.TrimSpace(match)
}

func scanFlow(rows *sql.Rows) (Flow, error) {
	var f Flow
	var reqRaw, respRaw string
	var stable, checker, banned int
	var sid sql.NullInt64
	if err := rows.Scan(&f.ID, &sid, &f.Direction, &f.StartTS, &f.EndTS, &reqRaw, &respRaw, &f.Hash, &stable, &checker, &banned, &f.ResponseCode, &f.FlowID, &f.SrcIP, &f.DstIP, &f.SrcPort, &f.DstPort, &f.Proto, &f.PktCount, &f.BytesIn, &f.BytesOut, &f.CreatedAt); err != nil {
		return f, err
	}
	if sid.Valid {
		f.ServiceID = intPtr(int(sid.Int64))
	}
	f.Stable = stable == 1
	f.Checker = checker == 1
	f.Banned = banned == 1
	f.RawRequest = parseJSONMap(reqRaw)
	f.RawResponse = parseJSONMap(respRaw)
	return f, nil
}

func scanFlowRow(row *sql.Row) (Flow, error) {
	var f Flow
	var reqRaw, respRaw string
	var stable, checker, banned int
	var sid sql.NullInt64
	if err := row.Scan(&f.ID, &sid, &f.Direction, &f.StartTS, &f.EndTS, &reqRaw, &respRaw, &f.Hash, &stable, &checker, &banned, &f.ResponseCode, &f.FlowID, &f.SrcIP, &f.DstIP, &f.SrcPort, &f.DstPort, &f.Proto, &f.PktCount, &f.BytesIn, &f.BytesOut, &f.CreatedAt); err != nil {
		return f, err
	}
	if sid.Valid {
		f.ServiceID = intPtr(int(sid.Int64))
	}
	f.Stable = stable == 1
	f.Checker = checker == 1
	f.Banned = banned == 1
	f.RawRequest = parseJSONMap(reqRaw)
	f.RawResponse = parseJSONMap(respRaw)
	return f, nil
}

func parseJSONMap(src string) map[string]any {
	out := map[string]any{}
	_ = json.Unmarshal([]byte(src), &out)
	return out
}

func ensureSuricataUnixHook(path, socket string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var root map[string]any
	if err := yaml.Unmarshal(b, &root); err != nil {
		return err
	}
	outs, _ := root["outputs"].([]any)
	for _, item := range outs {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		eve, ok := m["eve-log"].(map[string]any)
		if !ok {
			continue
		}
		if asString(eve["filetype"]) == "unix_stream" && asString(eve["filename"]) == socket {
			return nil
		}
	}
	newOut := map[string]any{
		"eve-log": map[string]any{
			"enabled":  "yes",
			"filetype": "unix_stream",
			"filename": socket,
			"types":    []any{"http"},
		},
	}
	outs = append(outs, newOut)
	root["outputs"] = outs
	enc, err := yaml.Marshal(root)
	if err != nil {
		return err
	}
	return os.WriteFile(path, enc, 0o644)
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.Itoa(int(t))
	case int:
		return strconv.Itoa(t)
	default:
		return ""
	}
}

func asInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case string:
		i, _ := strconv.Atoi(t)
		return i
	default:
		return 0
	}
}

func asInt64(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int:
		return int64(t)
	case int64:
		return t
	case string:
		i, _ := strconv.ParseInt(t, 10, 64)
		return i
	default:
		return 0
	}
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func boolQueryInt(v string) int {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return 1
	default:
		return 0
	}
}

func intPtr(v int) *int { return &v }

func intPtrToAny(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func jsonString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func parseInt(v string, def int) int {
	i, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return i
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/flagmate/suricata-ctf/backend/internal/api"
	"github.com/flagmate/suricata-ctf/backend/internal/ban"
	"github.com/flagmate/suricata-ctf/backend/internal/eve"
	"github.com/flagmate/suricata-ctf/backend/internal/flow"
	"github.com/flagmate/suricata-ctf/backend/internal/metrics"
	"github.com/flagmate/suricata-ctf/backend/internal/mirror"
	"github.com/flagmate/suricata-ctf/backend/internal/models"
	"github.com/flagmate/suricata-ctf/backend/internal/natsbus"
	"github.com/flagmate/suricata-ctf/backend/internal/normaliser"
	"github.com/flagmate/suricata-ctf/backend/internal/packetmirror"
	"github.com/flagmate/suricata-ctf/backend/internal/stable"
	"github.com/flagmate/suricata-ctf/backend/internal/store"
	"github.com/flagmate/suricata-ctf/backend/internal/suricata"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	eveSocket := os.Getenv("EVE_SOCKET")
	if eveSocket == "" {
		eveSocket = "/tmp/eve.sock"
	}

	postgresDSN := os.Getenv("POSTGRES_DSN")
	if postgresDSN == "" {
		log.Fatal("POSTGRES_DSN environment variable is required")
	}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://redis:6379/0"
	}

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats:4222"
	}

	rulesPath := os.Getenv("SURICATA_RULES_PATH")
	if rulesPath == "" {
		rulesPath = "/etc/suricata/rules/ctf.rules"
	}

	suricataPID, _ := strconv.Atoi(os.Getenv("SURICATA_PID"))

	s, err := store.New(postgresDSN, redisURL)
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}

	n := normaliser.New(nil)
	assembler := flow.NewAssembler(n)
	detector := stable.New(s)
	evaluator := ban.New(s)
	jsonMirror := mirror.New()
	metricsCollector = metrics.NewMetricsCollector()

	var natsBus *natsbus.NATSBus
	if natsURL != "" {
		natsBus, err = natsbus.New(natsURL, "flows:new")
		if err != nil {
			log.Printf("Warning: NATS not available, using in-memory broadcast: %v", err)
		}
	}

	ruleManager := suricata.NewRuleManager(rulesPath, suricataPID, 1000001)
	packetMirror := packetmirror.New(1)

	eveReader := eve.NewReader(eveSocket)
	eventCh := eveReader.Subscribe()

	go func() {
		for event := range eventCh {
			start := time.Now()
			ctx := assembler.ProcessEvent(event)
			if ctx == nil {
				continue
			}

			flowModel := assembler.ToModel(ctx)

			serviceID, err := findServiceByPort(s, event.DstPort)
			if err == nil {
				flowModel.ServiceID = &serviceID
			}

			flowModel.Stable = detector.IsStable(flowModel.Hash)
			s.IncrementHashCount(flowModel.Hash)

			// Don't auto-mark as checker - user decides manually
			// Stable flows are just highlighted green

			if evaluator.Evaluate(flowModel) {
				flowModel.Banned = true
			}

			if err := s.SaveFlow(flowModel); err != nil {
				log.Printf("Failed to save flow: %v", err)
			}

			metricsCollector.RecordFlow(flowModel.Stable, flowModel.Checker, flowModel.Banned)
			metricsCollector.RecordEVEEvent()
			metricsCollector.RecordFlowProcess(time.Since(start))

			if natsBus != nil {
				natsBus.Publish(flowModel)
			} else {
				broadcastFlow(flowModel)
			}

			if jsonMirror.IsEnabled() {
				go jsonMirror.Broadcast(fmt.Sprintf("%+v", event))
			}
		}
	}()

	go eveReader.Start()

	auth := api.NewAuthMiddleware()
	handler := api.NewHandler(s)

	mux := http.NewServeMux()

	mux.HandleFunc("POST /login", auth.Login)
	mux.HandleFunc("GET /services", auth.Validate(handler.ListServices))
	mux.HandleFunc("POST /services", auth.Validate(func(w http.ResponseWriter, r *http.Request) {
		handler.CreateService(w, r)
		ruleManager.AddService("service", 0, "tcp")
	}))
	mux.HandleFunc("DELETE /services/{id}", auth.Validate(handler.DeleteService))
	mux.HandleFunc("GET /patterns", auth.Validate(handler.ListPatterns))
	mux.HandleFunc("POST /patterns", auth.Validate(func(w http.ResponseWriter, r *http.Request) {
		handler.CreatePattern(w, r)
		evaluator.ReloadPatterns()
	}))
	mux.HandleFunc("DELETE /patterns/{id}", auth.Validate(func(w http.ResponseWriter, r *http.Request) {
		handler.DeletePattern(w, r)
		evaluator.ReloadPatterns()
	}))
	mux.HandleFunc("POST /patterns/{id}/toggle", auth.Validate(func(w http.ResponseWriter, r *http.Request) {
		handler.TogglePattern(w, r)
		evaluator.ReloadPatterns()
	}))
	mux.HandleFunc("GET /flows", auth.Validate(handler.ListFlows))
	mux.HandleFunc("GET /flows/{id}", auth.Validate(handler.GetFlow))
	mux.HandleFunc("GET /flows/{id}/history", auth.Validate(handler.GetFlowHistory))
	mux.HandleFunc("POST /flows/{id}/label", auth.Validate(handler.UpdateFlowLabel))
	mux.HandleFunc("POST /flows/{id}/ban", auth.Validate(handler.FlagFlowAsBanned))
	mux.HandleFunc("POST /flows/{id}/unban", auth.Validate(handler.UnbanFlow))
	mux.HandleFunc("GET /flows/{id}/unique-words", auth.Validate(handler.GetUniqueWords))
	mux.HandleFunc("GET /flow-groups", auth.Validate(handler.GetFlowGroups))
	mux.HandleFunc("POST /mirroring", auth.Validate(handler.UpdateMirroring))
	mux.HandleFunc("GET /mirroring", auth.Validate(handler.GetMirroring))
	mux.HandleFunc("/ws", wsHandler)
	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		fmt.Println("Server starting on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	eveReader.Stop()
	if natsBus != nil {
		natsBus.Close()
	}
	packetMirror.Stop()
}

func findServiceByPort(s *store.Store, port int) (int, error) {
	services, err := s.ListServices()
	if err != nil {
		return 0, err
	}
	for _, svc := range services {
		if svc.Port == port {
			return svc.ID, nil
		}
	}
	return 0, fmt.Errorf("service not found for port %d", port)
}

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan models.Flow, 1000)
var metricsCollector *metrics.MetricsCollector

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	clients[conn] = true
	if metricsCollector != nil {
		metricsCollector.SetWSClients(len(clients))
	}

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}

	delete(clients, conn)
	if metricsCollector != nil {
		metricsCollector.SetWSClients(len(clients))
	}
}

func broadcastFlow(flow models.Flow) {
	select {
	case broadcast <- flow:
	default:
	}
}

func init() {
	go func() {
		for flow := range broadcast {
			for client := range clients {
				err := client.WriteJSON(flow)
				if err != nil {
					client.Close()
					delete(clients, client)
				}
			}
		}
	}()
}

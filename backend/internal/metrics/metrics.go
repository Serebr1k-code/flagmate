package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	flowsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "flagmate_flows_total",
		Help: "Total number of flows processed",
	})

	flowsStable = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "flagmate_flows_stable",
		Help: "Number of currently stable flows",
	})

	flowsBanned = promauto.NewCounter(prometheus.CounterOpts{
		Name: "flagmate_flows_banned_total",
		Help: "Total number of flows flagged as banned",
	})

	flowsChecker = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "flagmate_flows_checker",
		Help: "Number of flows marked as checker",
	})

	mirrorTargetsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "flagmate_mirror_targets_active",
		Help: "Number of active mirroring targets",
	})

	eveEventsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "flagmate_eve_events_processed_total",
		Help: "Total number of EVE JSON events processed",
	})

	flowProcessDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "flagmate_flow_process_duration_seconds",
		Help:    "Time spent processing a single flow",
		Buckets: prometheus.DefBuckets,
	})

	patternMatches = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "flagmate_pattern_matches_total",
		Help: "Number of times each pattern matched",
	}, []string{"pattern_id"})

	serviceRules = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "flagmate_suricata_rules",
		Help: "Number of active Suricata rules",
	})

	wsClientsConnected = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "flagmate_ws_clients_connected",
		Help: "Number of currently connected WebSocket clients",
	})
)

type MetricsCollector struct {
	mu sync.RWMutex
	startTime time.Time
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		startTime: time.Now(),
	}
}

func (mc *MetricsCollector) RecordFlow(stable bool, checker bool, banned bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	flowsTotal.Inc()

	if stable {
		flowsStable.Inc()
	}
	if checker {
		flowsChecker.Inc()
	}
	if banned {
		flowsBanned.Inc()
	}
}

func (mc *MetricsCollector) RecordEVEEvent() {
	eveEventsProcessed.Inc()
}

func (mc *MetricsCollector) RecordFlowProcess(duration time.Duration) {
	flowProcessDuration.Observe(duration.Seconds())
}

func (mc *MetricsCollector) RecordPatternMatch(patternID int) {
	patternMatches.WithLabelValues(string(rune(patternID))).Inc()
}

func (mc *MetricsCollector) SetMirrorTargets(count int) {
	mirrorTargetsActive.Set(float64(count))
}

func (mc *MetricsCollector) SetServiceRules(count int) {
	serviceRules.Set(float64(count))
}

func (mc *MetricsCollector) SetWSClients(count int) {
	wsClientsConnected.Set(float64(count))
}

func (mc *MetricsCollector) GetUptime() time.Duration {
	return time.Since(mc.startTime)
}

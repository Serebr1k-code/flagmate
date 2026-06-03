package stable

import (
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/flagmate/suricata-ctf/backend/internal/store"
)

type Detector struct {
	store     *store.Store
	threshold int
	mu        sync.RWMutex
	stableHashes map[string]bool
}

func New(s *store.Store) *Detector {
	threshold := 5
	if env := os.Getenv("STABLE_THRESHOLD"); env != "" {
		if t, err := strconv.Atoi(env); err == nil {
			threshold = t
		}
	}

	d := &Detector{
		store:        s,
		threshold:    threshold,
		stableHashes: make(map[string]bool),
	}
	go d.monitorLoop()
	return d
}

func (d *Detector) IsStable(hash string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.stableHashes[hash]
}

func (d *Detector) monitorLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		groups, err := d.store.GetTopFlowGroups(100)
		if err != nil {
			continue
		}

		d.mu.Lock()
		for _, g := range groups {
			if g.Count >= d.threshold {
				d.stableHashes[g.Hash] = true
			} else {
				delete(d.stableHashes, g.Hash)
			}
		}
		d.mu.Unlock()
	}
}

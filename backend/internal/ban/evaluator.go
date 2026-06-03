package ban

import (
	"regexp"
	"sync"

	"github.com/flagmate/suricata-ctf/backend/internal/models"
	"github.com/flagmate/suricata-ctf/backend/internal/store"
)

type Evaluator struct {
	store  *store.Store
	mu     sync.RWMutex
	regexes []*regexp.Regexp
	patterns []models.Pattern
}

func New(s *store.Store) *Evaluator {
	e := &Evaluator{store: s}
	e.loadPatterns()
	return e
}

func (e *Evaluator) loadPatterns() {
	patterns, err := e.store.ListPatterns()
	if err != nil {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.patterns = patterns
	e.regexes = make([]*regexp.Regexp, len(patterns))
	for i, p := range patterns {
		if !p.Active {
			continue
		}
		re, err := regexp.Compile(p.Pattern)
		if err == nil {
			e.regexes[i] = re
		}
	}
}

func (e *Evaluator) Evaluate(flow models.Flow) bool {
	// Never ban checker flows
	if flow.Checker {
		return false
	}

	if flow.ResponseCode != 200 {
		return false
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, re := range e.regexes {
		if re != nil && re.MatchString(flow.NormPayload) {
			return true
		}
	}
	return false
}

func (e *Evaluator) ReloadPatterns() {
	e.loadPatterns()
}

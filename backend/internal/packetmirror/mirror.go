package packetmirror

import (
	"fmt"
	"log"
	"os/exec"
	"sync"
)

type PacketMirror struct {
	mu       sync.RWMutex
	rules    []MirrorRule
	active   bool
	queueNum int
}

type MirrorRule struct {
	ID          int
	Protocol    string
	DstPort     int
	TargetIP    string
	TargetPort  int
	Description string
}

func New(queueNum int) *PacketMirror {
	return &PacketMirror{
		queueNum: queueNum,
		rules:    make([]MirrorRule, 0),
	}
}

func (pm *PacketMirror) AddRule(rule MirrorRule) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if err := pm.applyIptablesRule(rule, true); err != nil {
		return fmt.Errorf("failed to add iptables rule: %w", err)
	}

	pm.rules = append(pm.rules, rule)
	log.Printf("Packet mirror rule added: %s -> %s:%d", formatRule(rule), rule.TargetIP, rule.TargetPort)
	return nil
}

func (pm *PacketMirror) RemoveRule(id int) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	var ruleToRemove *MirrorRule
	var remaining []MirrorRule

	for _, r := range pm.rules {
		if r.ID == id {
			ruleToRemove = &r
		} else {
			remaining = append(remaining, r)
		}
	}

	if ruleToRemove == nil {
		return fmt.Errorf("rule %d not found", id)
	}

	if err := pm.applyIptablesRule(*ruleToRemove, false); err != nil {
		log.Printf("Warning: failed to remove iptables rule: %v", err)
	}

	pm.rules = remaining
	log.Printf("Packet mirror rule removed: %s", formatRule(*ruleToRemove))
	return nil
}

func (pm *PacketMirror) Enable() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.active {
		return nil
	}

	for _, rule := range pm.rules {
		if err := pm.applyIptablesRule(rule, true); err != nil {
			return fmt.Errorf("failed to enable rule %d: %w", rule.ID, err)
		}
	}

	pm.active = true
	log.Println("Packet mirroring enabled")
	return nil
}

func (pm *PacketMirror) Disable() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if !pm.active {
		return nil
	}

	for _, rule := range pm.rules {
		if err := pm.applyIptablesRule(rule, false); err != nil {
			log.Printf("Warning: failed to disable rule %d: %v", rule.ID, err)
		}
	}

	pm.active = false
	log.Println("Packet mirroring disabled")
	return nil
}

func (pm *PacketMirror) ListRules() []MirrorRule {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]MirrorRule, len(pm.rules))
	copy(result, pm.rules)
	return result
}

func (pm *PacketMirror) IsActive() bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.active
}

func (pm *PacketMirror) applyIptablesRule(rule MirrorRule, add bool) error {
	action := "-A"
	if !add {
		action = "-D"
	}

	proto := rule.Protocol
	if proto == "" {
		proto = "tcp"
	}

	cmd := exec.Command("iptables",
		"-t", "mangle",
		action, "PREROUTING",
		"-p", proto,
		"--dport", fmt.Sprintf("%d", rule.DstPort),
		"-j", "NFQUEUE",
		"--queue-num", fmt.Sprintf("%d", pm.queueNum),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("iptables %s %s: %v: %s", action, formatRule(rule), err, string(output))
	}

	return nil
}

func formatRule(rule MirrorRule) string {
	return fmt.Sprintf("%s:%d -> %s:%d", rule.Protocol, rule.DstPort, rule.TargetIP, rule.TargetPort)
}

func (pm *PacketMirror) Stop() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, rule := range pm.rules {
		if err := pm.applyIptablesRule(rule, false); err != nil {
			log.Printf("Warning: failed to remove iptables rule on stop: %v", err)
		}
	}

	pm.active = false
	pm.rules = pm.rules[:0]
	log.Println("Packet mirror stopped")
	return nil
}

package suricata

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type RuleManager struct {
	mu           sync.RWMutex
	rulesPath    string
	suricataPID  int
	nextSID      int
	loadedRules  map[int]*Rule
}

type Rule struct {
	SID      int
	Name     string
	Port     int
	Protocol string
	Action   string
	Raw      string
}

func NewRuleManager(rulesPath string, suricataPID int, startSID int) *RuleManager {
	rm := &RuleManager{
		rulesPath:   rulesPath,
		suricataPID: suricataPID,
		nextSID:     startSID,
		loadedRules: make(map[int]*Rule),
	}

	if err := rm.ensureRulesFile(); err != nil {
		log.Printf("Warning: could not ensure rules file: %v", err)
	}

	if err := rm.loadExistingRules(); err != nil {
		log.Printf("Warning: could not load existing rules: %v", err)
	}

	return rm
}

func (rm *RuleManager) AddService(name string, port int, protocol string) (*Rule, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	sid := rm.nextSID
	rm.nextSID++

	rule := &Rule{
		SID:      sid,
		Name:     name,
		Port:     port,
		Protocol: strings.ToUpper(protocol),
		Action:   "alert",
		Raw:      fmt.Sprintf(`alert %s any any -> any %d (msg:"CTF Service %s"; sid:%d; rev:1;)`,
			strings.ToLower(protocol), port, name, sid),
	}

	rm.loadedRules[sid] = rule

	if err := rm.writeRulesFile(); err != nil {
		delete(rm.loadedRules, sid)
		return nil, fmt.Errorf("failed to write rules file: %w", err)
	}

	if err := rm.reloadSuricata(); err != nil {
		log.Printf("Warning: Suricata reload failed: %v", err)
	}

	log.Printf("Rule added: SID=%d, Service=%s, Port=%d", sid, name, port)
	return rule, nil
}

func (rm *RuleManager) RemoveService(sid int) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.loadedRules[sid]; !exists {
		return fmt.Errorf("rule SID %d not found", sid)
	}

	delete(rm.loadedRules, sid)

	if err := rm.writeRulesFile(); err != nil {
		return fmt.Errorf("failed to write rules file: %w", err)
	}

	if err := rm.reloadSuricata(); err != nil {
		log.Printf("Warning: Suricata reload failed: %v", err)
	}

	log.Printf("Rule removed: SID=%d", sid)
	return nil
}

func (rm *RuleManager) ListRules() []*Rule {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	rules := make([]*Rule, 0, len(rm.loadedRules))
	for _, r := range rm.loadedRules {
		rules = append(rules, r)
	}
	return rules
}

func (rm *RuleManager) GetNextSID() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.nextSID
}

func (rm *RuleManager) ensureRulesFile() error {
	dir := filepath.Dir(rm.rulesPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	if _, err := os.Stat(rm.rulesPath); os.IsNotExist(err) {
		header := "# CTF Suricata Rules - Managed by FlagMate\n# Do not edit manually\n\n"
		return os.WriteFile(rm.rulesPath, []byte(header), 0644)
	}

	return nil
}

func (rm *RuleManager) loadExistingRules() error {
	content, err := os.ReadFile(rm.rulesPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	maxSID := rm.nextSID

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		sid := extractSID(line)
		if sid > 0 {
			if sid >= maxSID {
				maxSID = sid + 1
			}

			parts := strings.SplitN(line, `msg:"`, 2)
			name := "unknown"
			if len(parts) > 1 {
				nameParts := strings.SplitN(parts[1], `";`, 2)
				if len(nameParts) > 0 {
					name = strings.TrimPrefix(nameParts[0], "CTF Service ")
				}
			}

			port := extractPort(line)
			proto := extractProtocol(line)

			rm.loadedRules[sid] = &Rule{
				SID:      sid,
				Name:     name,
				Port:     port,
				Protocol: proto,
				Action:   "alert",
				Raw:      line,
			}
		}
	}

	rm.nextSID = maxSID
	return nil
}

func (rm *RuleManager) writeRulesFile() error {
	var builder strings.Builder
	builder.WriteString("# CTF Suricata Rules - Managed by FlagMate\n")
	builder.WriteString("# Auto-generated. Do not edit manually.\n\n")

	for _, rule := range rm.loadedRules {
		builder.WriteString(rule.Raw + "\n")
	}

	return os.WriteFile(rm.rulesPath, []byte(builder.String()), 0644)
}

func (rm *RuleManager) reloadSuricata() error {
	if rm.suricataPID <= 0 {
		log.Println("Suricata PID not set, skipping reload")
		return nil
	}

	cmd := exec.Command("kill", "-USR2", strconv.Itoa(rm.suricataPID))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to send SIGUSR2 to Suricata (PID %d): %v: %s",
			rm.suricataPID, err, string(output))
	}

	log.Printf("Sent SIGUSR2 to Suricata (PID %d) for rule reload", rm.suricataPID)
	return nil
}

func extractSID(rule string) int {
	idx := strings.Index(rule, "sid:")
	if idx == -1 {
		return 0
	}

	rest := rule[idx+4:]
	endIdx := strings.IndexAny(rest, ";)")
	if endIdx == -1 {
		return 0
	}

	sidStr := strings.TrimSpace(rest[:endIdx])
	sid, err := strconv.Atoi(sidStr)
	if err != nil {
		return 0
	}

	return sid
}

func extractPort(rule string) int {
	parts := strings.Split(rule, "->")
	if len(parts) < 2 {
		return 0
	}

	destParts := strings.TrimSpace(parts[1])
	fields := strings.Fields(destParts)
	if len(fields) < 2 {
		return 0
	}

	portStr := strings.Trim(fields[1], "()")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0
	}

	return port
}

func extractProtocol(rule string) string {
	fields := strings.Fields(rule)
	if len(fields) < 2 {
		return "tcp"
	}

	return strings.ToUpper(fields[1])
}

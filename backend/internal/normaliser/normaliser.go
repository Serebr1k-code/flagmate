package normaliser

import (
	"encoding/json"
	"regexp"
	"sync"
)

type Rule struct {
	Pattern     string `json:"pattern"`
	Replacement string `json:"replacement"`
}

type Normaliser struct {
	rules []Rule
	regexCache []*regexp.Regexp
	mu         sync.RWMutex
}

var defaultRules = []Rule{
	{`Authorization:\s*Bearer\s+\S+`, `Authorization: Bearer <TOKEN>`},
	{`Cookie:\s*.*?=(\S+);?`, `Cookie: <COOKIE>=<VALUE>`},
	{`"session_id"\s*:\s*".*?"`, `"session_id":"<ID>"`},
	{`"token"\s*:\s*".*?"`, `"token":"<TOKEN>"`},
	{`"jwt"\s*:\s*".*?"`, `"jwt":"<TOKEN>"`},
	{`csrf_token=\w+`, `csrf_token=<TOKEN>`},
	{`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}.\d+Z`, `<TIME>`},
	{`\b\d{10}\b`, `<EPOCH>`},
	{`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`, `<UUID>`},
	{`([?&])\w+=\w+`, `$1<PARAM>=<VAL>`},
	{`boundary=----WebKitFormBoundary\w+`, `boundary=----WebKitFormBoundary<BOUNDARY>`},
	{`"request_id"\s*:\s*".*?"`, `"request_id":"<ID>"`},
	{`"nonce"\s*:\s*".*?"`, `"nonce":"<NONCE>"`},
	{`"timestamp"\s*:\s*\d+`, `"timestamp":<TIME>`},
}

func New(rules []Rule) *Normaliser {
	n := &Normaliser{}
	if len(rules) == 0 {
		rules = defaultRules
	}
	n.SetRules(rules)
	return n
}

func (n *Normaliser) SetRules(rules []Rule) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.rules = rules
	n.regexCache = make([]*regexp.Regexp, len(rules))
	for i, r := range rules {
		n.regexCache[i] = regexp.MustCompile(r.Pattern)
	}
}

func (n *Normaliser) Normalise(input string) string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	result := input
	for i, re := range n.regexCache {
		result = re.ReplaceAllString(result, n.rules[i].Replacement)
	}
	return result
}

func (n *Normaliser) NormaliseJSON(data interface{}) string {
	raw, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return n.Normalise(string(raw))
}

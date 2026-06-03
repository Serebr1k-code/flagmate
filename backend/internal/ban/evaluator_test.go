package ban

import (
	"regexp"
	"testing"

	"github.com/flagmate/suricata-ctf/backend/internal/models"
)

type mockStore struct {
	patterns []models.Pattern
}

func (m *mockStore) ListPatterns() ([]models.Pattern, error) {
	return m.patterns, nil
}

func TestEvaluator_ResponseCodeGuard(t *testing.T) {
	e := &Evaluator{
		patterns: []models.Pattern{
			{ID: 1, Pattern: "FLAG\\{.*\\}", Description: "flag pattern"},
		},
	}
	e.regexes = []*regexp.Regexp{regexp.MustCompile("FLAG\\{.*\\}")}

	flow := models.Flow{
		ResponseCode: 403,
		NormPayload:  `{"status": 403, "body": "FLAG{test}"}`,
	}

	if e.Evaluate(flow) {
		t.Error("Evaluator should not flag flow with non-200 response code")
	}
}

func TestEvaluator_PatternMatch(t *testing.T) {
	e := &Evaluator{
		patterns: []models.Pattern{
			{ID: 1, Pattern: "FLAG\\{.*\\}", Description: "flag pattern"},
		},
	}
	e.regexes = []*regexp.Regexp{regexp.MustCompile("FLAG\\{.*\\}")}

	flow := models.Flow{
		ResponseCode: 200,
		NormPayload:  `{"status": 200, "body": "FLAG{abc123}"}`,
	}

	if !e.Evaluate(flow) {
		t.Error("Evaluator should flag flow matching pattern with 200 response")
	}
}

func TestEvaluator_NoMatch(t *testing.T) {
	e := &Evaluator{
		patterns: []models.Pattern{
			{ID: 1, Pattern: "FLAG\\{.*\\}", Description: "flag pattern"},
		},
	}
	e.regexes = []*regexp.Regexp{regexp.MustCompile("FLAG\\{.*\\}")}

	flow := models.Flow{
		ResponseCode: 200,
		NormPayload:  `{"status": 200, "body": "Hello World"}`,
	}

	if e.Evaluate(flow) {
		t.Error("Evaluator should not flag flow without pattern match")
	}
}

package normaliser

import (
	"testing"
)

func TestNormaliser_DefaultRules(t *testing.T) {
	n := New(nil)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Bearer token masking",
			input:    "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.abc123",
			expected: "Authorization: Bearer <TOKEN>",
		},
		{
			name:     "UUID masking",
			input:    "session_id=550e8400-e29b-41d4-a716-446655440000",
			expected: "session_id=<UUID>",
		},
		{
			name:     "Timestamp masking",
			input:    "2024-01-15T10:30:00.000Z",
			expected: "<TIME>",
		},
		{
			name:     "Unix epoch masking",
			input:    "timestamp=1705312200",
			expected: "timestamp=<EPOCH>",
		},
		{
			name:     "CSRF token masking",
			input:    "csrf_token=abc123def456",
			expected: "csrf_token=<TOKEN>",
		},
		{
			name:     "No masking needed",
			input:    "GET /api/health HTTP/1.1",
			expected: "GET /api/health HTTP/1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := n.Normalise(tt.input)
			if result != tt.expected {
				t.Errorf("Normalise(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormaliser_CustomRules(t *testing.T) {
	rules := []Rule{
		{Pattern: `secret=\w+`, Replacement: `secret=<HIDDEN>`},
		{Pattern: `\d{4}-\d{2}-\d{2}`, Replacement: `<DATE>`},
	}

	n := New(rules)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Custom secret masking",
			input:    "secret=abc123",
			expected: "secret=<HIDDEN>",
		},
		{
			name:     "Custom date masking",
			input:    "date=2024-01-15",
			expected: "date=<DATE>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := n.Normalise(tt.input)
			if result != tt.expected {
				t.Errorf("Normalise(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormaliser_JSON(t *testing.T) {
	n := New(nil)

	data := map[string]interface{}{
		"Authorization": "Bearer eyJhbGciOiJIUzI1NiJ9.test",
		"user_id":       "550e8400-e29b-41d4-a716-446655440000",
	}

	result := n.NormaliseJSON(data)
	if result == "" {
		t.Error("NormaliseJSON returned empty string")
	}
}

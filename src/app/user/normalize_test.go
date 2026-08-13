package user

import "testing"

// normalize helpers implement the write-path normalization that backs the
// case-insensitive uniqueness (M8): emails are trimmed + lowercased,
// usernames are trimmed (display case preserved).
func TestNormalizeEmail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"already canonical", "alice@example.com", "alice@example.com"},
		{"mixed case", "Alice.Smith@Example.COM", "alice.smith@example.com"},
		{"leading and trailing spaces", "  bob@example.com  ", "bob@example.com"},
		{"empty", "", ""},
		{"whitespace only", "   ", ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeEmail(tt.input); got != tt.want {
				t.Errorf("normalizeEmail(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeUsername(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"keeps display case", "Alice", "Alice"},
		{"trims surrounding spaces", "  alice  ", "alice"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeUsername(tt.input); got != tt.want {
				t.Errorf("normalizeUsername(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

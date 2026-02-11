package user

import (
	"testing"
)

func TestValidateEmail(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name        string
		email       string
		shouldError bool
		errorMsg    string
	}

	tests := []testCase{
		// Valid emails
		{
			name:        "Valid simple email",
			email:       "user@example.com",
			shouldError: false,
		},
		{
			name:        "Valid email with subdomain",
			email:       "user@mail.example.com",
			shouldError: false,
		},
		{
			name:        "Valid email with plus sign",
			email:       "user+tag@example.com",
			shouldError: false,
		},
		{
			name:        "Valid email with dots",
			email:       "first.last@example.com",
			shouldError: false,
		},
		{
			name:        "Valid email with numbers",
			email:       "user123@example123.com",
			shouldError: false,
		},
		{
			name:        "Valid email with hyphen",
			email:       "user-name@example.com",
			shouldError: false,
		},
		{
			name:        "Valid email with underscore",
			email:       "user_name@example.com",
			shouldError: false,
		},
		{
			name:        "Valid email with multiple subdomains",
			email:       "user@mail.subdomain.example.com",
			shouldError: false,
		},
		{
			name:        "Valid email with long TLD",
			email:       "user@example.museum",
			shouldError: false,
		},
		{
			name:        "Valid email minimal",
			email:       "a@b.co",
			shouldError: false,
		},

		// Invalid emails - Empty or length issues
		{
			name:        "Empty email",
			email:       "",
			shouldError: true,
			errorMsg:    "email cannot be empty",
		},
		{
			name:        "Email too short",
			email:       "a@b",
			shouldError: true,
			errorMsg:    "email is too short",
		},

		// Invalid emails - Missing @ symbol
		{
			name:        "Missing @ symbol",
			email:       "userexample.com",
			shouldError: true,
			errorMsg:    "email must contain @ symbol",
		},

		// Invalid emails - Multiple @ symbols
		{
			name:        "Multiple @ symbols",
			email:       "user@@example.com",
			shouldError: true,
			errorMsg:    "email must contain exactly one @ symbol",
		},
		{
			name:        "Multiple @ symbols different positions",
			email:       "user@name@example.com",
			shouldError: true,
			errorMsg:    "email must contain exactly one @ symbol",
		},

		// Invalid emails - Local part issues
		{
			name:        "Empty local part",
			email:       "@example.com",
			shouldError: true,
			errorMsg:    "email local part cannot be empty",
		},
		{
			name:        "Local part too long (over 64 chars)",
			email:       "verylonglocalpartverylonglocalpartverylonglocalpartverylonglocalpart@example.com",
			shouldError: true,
			errorMsg:    "email local part is too long (max 64 characters)",
		},
		{
			name:        "Local part starts with dot",
			email:       ".user@example.com",
			shouldError: true,
			errorMsg:    "email local part cannot start or end with a dot",
		},
		{
			name:        "Local part ends with dot",
			email:       "user.@example.com",
			shouldError: true,
			errorMsg:    "email local part cannot start or end with a dot",
		},
		{
			name:        "Local part with consecutive dots",
			email:       "user..name@example.com",
			shouldError: true,
			errorMsg:    "email local part cannot contain consecutive dots",
		},

		// Invalid emails - Domain issues
		{
			name:        "Empty domain",
			email:       "user@",
			shouldError: true,
			errorMsg:    "email domain cannot be empty",
		},
		{
			name:        "Domain without dot",
			email:       "user@example",
			shouldError: true,
			errorMsg:    "email domain must contain at least one dot",
		},
		{
			name:        "Domain starts with dot",
			email:       "user@.example.com",
			shouldError: true,
			errorMsg:    "email domain cannot start or end with a dot",
		},
		{
			name:        "Domain ends with dot",
			email:       "user@example.com.",
			shouldError: true,
			errorMsg:    "email domain cannot start or end with a dot",
		},
		{
			name:        "Domain starts with hyphen",
			email:       "user@-example.com",
			shouldError: true,
			errorMsg:    "email domain cannot start or end with a hyphen",
		},
		{
			name:        "Domain ends with hyphen",
			email:       "user@example.com-",
			shouldError: true,
			errorMsg:    "email domain cannot start or end with a hyphen",
		},
		{
			name:        "Domain with consecutive dots",
			email:       "user@example..com",
			shouldError: true,
			errorMsg:    "email domain cannot contain consecutive dots",
		},
		{
			name:        "Domain label too long (over 63 chars)",
			email:       "user@verylongdomainlabelverylongdomainlabelverylongdomainlabelverylongdomainlabel.com",
			shouldError: true,
			errorMsg:    "email domain label is too long (max 63 characters)",
		},
		{
			name:        "Domain label starts with hyphen",
			email:       "user@mail.-example.com",
			shouldError: true,
			errorMsg:    "email domain label cannot start or end with a hyphen",
		},
		{
			name:        "Domain label ends with hyphen",
			email:       "user@mail-.example.com",
			shouldError: true,
			errorMsg:    "email domain label cannot start or end with a hyphen",
		},

		// Invalid emails - TLD issues
		{
			name:        "TLD too short (1 char)",
			email:       "user@example.c",
			shouldError: true,
			errorMsg:    "email top-level domain must be at least 2 characters",
		},

		// Invalid emails - Special characters
		{
			name:        "Email with spaces",
			email:       "user name@example.com",
			shouldError: true,
			errorMsg:    "email format is invalid",
		},
		{
			name:        "Email with special characters",
			email:       "user@#$%@example.com",
			shouldError: true,
			errorMsg:    "email must contain exactly one @ symbol",
		},
		{
			name:        "Domain with special characters",
			email:       "user@exam ple.com",
			shouldError: true,
			errorMsg:    "email format is invalid",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateEmail(tc.email)

			if tc.shouldError {
				if err == nil {
					t.Errorf("Expected error for email '%s', but got nil", tc.email)
				} else if tc.errorMsg != "" && err.Error() != tc.errorMsg {
					t.Errorf("Expected error message '%s', but got '%s'", tc.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for email '%s', but got: %v", tc.email, err)
				}
			}
		})
	}
}

func TestValidateEmailEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("Email with whitespace should be trimmed and validated", func(t *testing.T) {
		email := "  user@example.com  "
		err := validateEmail(email)
		if err != nil {
			t.Errorf("Expected email with whitespace to be valid after trimming, got error: %v", err)
		}
	})

	t.Run("Case sensitivity should be allowed", func(t *testing.T) {
		emails := []string{
			"User@Example.COM",
			"USER@EXAMPLE.COM",
			"user@EXAMPLE.com",
		}
		for _, email := range emails {
			err := validateEmail(email)
			if err != nil {
				t.Errorf("Expected email '%s' to be valid, got error: %v", email, err)
			}
		}
	})

	t.Run("Maximum valid local part length (64 chars)", func(t *testing.T) {
		// Exactly 64 characters in local part
		email := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa@example.com"
		err := validateEmail(email)
		if err != nil {
			t.Errorf("Expected 64-char local part to be valid, got error: %v", err)
		}
	})

	t.Run("Maximum valid email length (254 chars)", func(t *testing.T) {
		// Total length exactly 254 characters
		// 64 (local) + 1 (@) + 189 (domain including dots and TLD) = 254
		localPart := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 64 chars
		domainPart := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.com" // 189 chars
		email := localPart + "@" + domainPart // 64 + 1 + 189 = 254
		err := validateEmail(email)
		if err != nil {
			t.Errorf("Expected 254-char email to be valid, got error: %v (length: %d)", err, len(email))
		}
	})
}

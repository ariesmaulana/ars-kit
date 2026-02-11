package user

import (
	"errors"
	"net"
	"regexp"
	"strings"
)

// validateEmail performs comprehensive email validation
// Returns error if email is invalid, nil if valid
func validateEmail(email string) error {
	// Check if email is empty
	if email == "" {
		return errors.New("email cannot be empty")
	}

	// Trim whitespace
	email = strings.TrimSpace(email)

	// Check minimum length (must be at least a@b.c = 5 chars)
	if len(email) < 5 {
		return errors.New("email is too short")
	}

	// Check maximum length (RFC 5321 specifies max 254 chars)
	if len(email) > 254 {
		return errors.New("email is too long")
	}

	// Check for @ symbol
	if !strings.Contains(email, "@") {
		return errors.New("email must contain @ symbol")
	}

	// Split email into local and domain parts
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return errors.New("email must contain exactly one @ symbol")
	}

	localPart := parts[0]
	domainPart := parts[1]

	// Check for invalid characters (spaces, tabs, etc.) early to return generic error
	if strings.ContainsAny(email, " \t\n\r") {
		return errors.New("email format is invalid")
	}

	// Validate local part (before @)
	if err := validateLocalPart(localPart); err != nil {
		return err
	}

	// Validate domain part (after @)
	if err := validateDomainPart(domainPart); err != nil {
		return err
	}

	// Final regex pattern for email validation (RFC 5322 simplified)
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return errors.New("email format is invalid")
	}

	return nil
}

// validateLocalPart validates the local part of an email address (before @)
func validateLocalPart(localPart string) error {
	if localPart == "" {
		return errors.New("email local part cannot be empty")
	}

	if len(localPart) > 64 {
		return errors.New("email local part is too long (max 64 characters)")
	}

	// Check local part doesn't start or end with dot
	if strings.HasPrefix(localPart, ".") || strings.HasSuffix(localPart, ".") {
		return errors.New("email local part cannot start or end with a dot")
	}

	// Check for consecutive dots in local part
	if strings.Contains(localPart, "..") {
		return errors.New("email local part cannot contain consecutive dots")
	}

	return nil
}

// validateDomainPart validates the domain part of an email address (after @)
func validateDomainPart(domainPart string) error {
	if domainPart == "" {
		return errors.New("email domain cannot be empty")
	}

	if len(domainPart) > 255 {
		return errors.New("email domain is too long (max 255 characters)")
	}

	// Check domain contains at least one dot
	if !strings.Contains(domainPart, ".") {
		return errors.New("email domain must contain at least one dot")
	}

	// Check domain doesn't start or end with dot or hyphen
	if strings.HasPrefix(domainPart, ".") || strings.HasSuffix(domainPart, ".") {
		return errors.New("email domain cannot start or end with a dot")
	}

	if strings.HasPrefix(domainPart, "-") || strings.HasSuffix(domainPart, "-") {
		return errors.New("email domain cannot start or end with a hyphen")
	}

	// Check for consecutive dots in domain
	if strings.Contains(domainPart, "..") {
		return errors.New("email domain cannot contain consecutive dots")
	}

	// Validate domain labels (parts separated by dots)
	domainLabels := strings.Split(domainPart, ".")
	if len(domainLabels) < 2 {
		return errors.New("email domain must have at least two labels (e.g., example.com)")
	}

	for i, label := range domainLabels {
		if err := validateDomainLabel(label, i == len(domainLabels)-1); err != nil {
			return err
		}
	}

	// Additional domain validation: check if domain looks like a valid hostname
	if err := validateHostname(domainPart); err != nil {
		return err
	}

	return nil
}

// validateDomainLabel validates a single label in the domain
func validateDomainLabel(label string, isTLD bool) error {
	if label == "" {
		return errors.New("email domain cannot have empty labels")
	}

	if len(label) > 63 {
		return errors.New("email domain label is too long (max 63 characters)")
	}

	if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
		return errors.New("email domain label cannot start or end with a hyphen")
	}

	// Check TLD (top-level domain) is at least 2 characters
	if isTLD && len(label) < 2 {
		return errors.New("email top-level domain must be at least 2 characters")
	}

	return nil
}

// validateHostname performs additional hostname validation
func validateHostname(hostname string) error {
	// Convert to lowercase for validation
	hostname = strings.ToLower(hostname)

	// Check for common typos and invalid patterns
	invalidPatterns := []string{
		"..",   // consecutive dots (already checked but double-check)
		"@",    // should not have @ in domain
		".@",   // dot before at
		"@.",   // at before dot
	}

	for _, pattern := range invalidPatterns {
		if strings.Contains(hostname, pattern) {
			return errors.New("email domain contains invalid pattern: " + pattern)
		}
	}

	// Optional: Validate that domain looks like it could be resolved
	// Note: This doesn't actually do DNS lookup, just checks format
	if err := validateDomainFormat(hostname); err != nil {
		return err
	}

	return nil
}

// validateDomainFormat checks if domain has a valid format for DNS
func validateDomainFormat(domain string) error {
	// Check for IP addresses in domain (generally not recommended for email)
	if net.ParseIP(domain) != nil {
		return errors.New("email domain should not be an IP address")
	}

	// Check if domain is enclosed in brackets (IPv6 or literal)
	if strings.HasPrefix(domain, "[") && strings.HasSuffix(domain, "]") {
		return errors.New("email domain should not be enclosed in brackets")
	}

	// Ensure domain doesn't look like a URL
	urlPrefixes := []string{"http://", "https://", "ftp://", "www."}
	for _, prefix := range urlPrefixes {
		if strings.HasPrefix(strings.ToLower(domain), prefix) {
			return errors.New("email domain should not contain URL prefix")
		}
	}

	// Check for suspicious patterns that might indicate a malformed email
	if strings.Count(domain, ".") > 10 {
		return errors.New("email domain has too many subdomain levels")
	}

	return nil
}


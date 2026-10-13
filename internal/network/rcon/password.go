package rcon

import "strings"

// ValidPassword reports whether a configured RCON secret meets the minimum policy.
func ValidPassword(password string) bool {
	if len(password) < 16 || strings.TrimSpace(password) != password {
		return false
	}
	lower := strings.ToLower(password)
	for _, common := range []string{"password", "changeme", "qwerty", "123456", "letmein"} {
		if strings.Contains(lower, common) {
			return false
		}
	}
	var categories [4]bool
	unique := make(map[byte]struct{}, len(password))
	for i := 0; i < len(password); i++ {
		c := password[i]
		unique[c] = struct{}{}
		switch {
		case c >= 'a' && c <= 'z':
			categories[0] = true
		case c >= 'A' && c <= 'Z':
			categories[1] = true
		case c >= '0' && c <= '9':
			categories[2] = true
		default:
			categories[3] = true
		}
	}
	if len(unique) < 8 {
		return false
	}
	categoryCount := 0
	for _, present := range categories {
		if present {
			categoryCount++
		}
	}
	return categoryCount >= 2
}

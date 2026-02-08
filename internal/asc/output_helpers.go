package asc

import "strings"

func compactWhitespace(input string) string {
	clean := sanitizeTerminal(input)
	return strings.Join(strings.Fields(clean), " ")
}

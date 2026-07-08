package process

import "regexp"

// redactPatterns match obvious secret values in command lines. We redact the
// value while preserving the key so audits stay useful.
var redactPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(token=)[^\s]+`),
	regexp.MustCompile(`(?i)(api[_-]?key=)[^\s]+`),
	regexp.MustCompile(`(?i)(password=)[^\s]+`),
	regexp.MustCompile(`(?i)(secret=)[^\s]+`),
	regexp.MustCompile(`(?i)(AWS_SECRET_ACCESS_KEY=)[^\s]+`),
	regexp.MustCompile(`(?i)(authorization:\s*bearer\s+)[^\s]+`),
}

// Redact replaces obvious secret values in s with a placeholder.
func Redact(s string) string {
	for _, re := range redactPatterns {
		s = re.ReplaceAllString(s, "${1}[REDACTED]")
	}
	return s
}

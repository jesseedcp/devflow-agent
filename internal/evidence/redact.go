package evidence

import (
	"strings"
	"unicode/utf8"

	"regexp"
)

type redactionRule struct {
	pattern     *regexp.Regexp
	replacement string
}

var redactionRules = []redactionRule{
	{regexp.MustCompile(`(?i)(authorization:\s*bearer\s+)[^\s]+`), `${1}***`},
	{regexp.MustCompile(`(?i)(authorization:\s*basic\s+)[^\s]+`), `${1}***`},
	{regexp.MustCompile(`(?i)(cookie:\s*).+`), `${1}***`},
	{regexp.MustCompile(`(?i)(set-cookie:\s*).+`), `${1}***`},
	{regexp.MustCompile(`(?i)((?:[?&]|\b)(?:token|access_token|refresh_token|api_key|apikey|secret|password|passwd|session|signature|client_secret|tenant_access_token)=)[^&\s]+`), `${1}***`},
	{regexp.MustCompile(`(?i)("(?:token|access_token|refresh_token|api_key|apikey|secret|password|passwd|session|signature|client_secret|tenant_access_token)"\s*:\s*")[^"]*(")`), `${1}***${2}`},
}

func Redact(value string) string {
	out := value
	for _, rule := range redactionRules {
		out = rule.pattern.ReplaceAllString(out, rule.replacement)
	}
	return out
}

func Excerpt(value string, maxBytes int) string {
	if maxBytes <= 0 {
		maxBytes = 4096
	}
	redacted := Redact(value)
	if len(redacted) <= maxBytes {
		return redacted
	}
	cut := redacted[:maxBytes]
	for !utf8.ValidString(cut) && len(cut) > 0 {
		cut = cut[:len(cut)-1]
	}
	return strings.TrimRight(cut, "\r\n ") + "... (truncated)"
}

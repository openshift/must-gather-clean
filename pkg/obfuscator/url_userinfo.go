package obfuscator

import "regexp"

const urlUserinfoPasswordReplacement = "****"

// urlUserinfoPasswordPattern matches user:password@ in URL schemes (http, https, socks5, etc.).
var urlUserinfoPasswordPattern = regexp.MustCompile(`(://[^:/@]+:)([^@/]+)(@)`)

type urlUserinfoObfuscator struct{}

func (u *urlUserinfoObfuscator) Path(s string) string {
	return u.replace(s)
}

func (u *urlUserinfoObfuscator) Contents(s string) string {
	return u.replace(s)
}

func (u *urlUserinfoObfuscator) Report() ReplacementReport {
	return ReplacementReport{}
}

func (u *urlUserinfoObfuscator) replace(input string) string {
	return urlUserinfoPasswordPattern.ReplaceAllString(input, "${1}"+urlUserinfoPasswordReplacement+"${3}")
}

func NewURLUserinfoObfuscator() ReportingObfuscator {
	return &urlUserinfoObfuscator{}
}

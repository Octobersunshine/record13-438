package slowlog

import (
	"regexp"
	"strings"
)

var (
	multiLineCommentRE = regexp.MustCompile(`/\*[^*]*\*+(?:[^/*][^*]*\*+)*/`)
	singleLineCommentRE = regexp.MustCompile(`(?:--|\#)[^\n]*`)
	hexLiteralRE  = regexp.MustCompile(`\b0x[0-9a-fA-F]+\b`)
	boolLiteralRE = regexp.MustCompile(`\b(?:true|false|null)\b`)
	floatLiteralRE = regexp.MustCompile(`\b\d+\.\d+\b`)
	intLiteralRE  = regexp.MustCompile(`\b\d+\b`)
	stringLiteralRE = regexp.MustCompile(`'[^']*'`)
	doubleStringRE  = regexp.MustCompile(`"[^"]*"`)
	inListRE = regexp.MustCompile(`\bIN\s*\((?:\?\s*,\s*)*\?\)`)
	wsRE     = regexp.MustCompile(`\s+`)
)

func Fingerprint(sql string) string {
	if sql == "" {
		return ""
	}

	s := strings.ToLower(sql)

	s = multiLineCommentRE.ReplaceAllString(s, " ")
	s = singleLineCommentRE.ReplaceAllString(s, " ")

	s = hexLiteralRE.ReplaceAllString(s, "?")

	s = stringLiteralRE.ReplaceAllString(s, "?")
	s = doubleStringRE.ReplaceAllString(s, "?")

	s = floatLiteralRE.ReplaceAllString(s, "?")
	s = intLiteralRE.ReplaceAllString(s, "?")

	s = boolLiteralRE.ReplaceAllString(s, "?")

	s = inListRE.ReplaceAllString(s, "in (?)")

	s = wsRE.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)

	s = strings.TrimSuffix(s, ";")
	s = strings.TrimSpace(s)

	return s
}

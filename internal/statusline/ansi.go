package statusline

import (
	"strings"
	"unicode/utf8"
)

const (
	ansiReset         = "\033[0m"
	ansiBold          = "\033[1m"
	ansiDim           = "\033[2m"
	ansiFgRed         = "\033[31m"
	ansiFgGreen       = "\033[32m"
	ansiFgYellow      = "\033[33m"
	ansiFgBlue        = "\033[34m"
	ansiFgMagenta     = "\033[35m"
	ansiFgCyan        = "\033[36m"
	ansiFgWhite       = "\033[37m"
	ansiFgGray        = "\033[90m"
	ansiFgBrightRed   = "\033[91m"
	ansiFgBrightGreen = "\033[92m"
	ansiFgBrightYel   = "\033[93m"
	ansiFgBrightBlue  = "\033[94m"
	ansiFgBrightMag   = "\033[95m"
	ansiFgBrightCyan  = "\033[96m"
	ansiFgBrightWhite = "\033[97m"
)

type styler struct {
	noColor bool
}

func (s styler) paint(text string, codes ...string) string {
	if text == "" || s.noColor || len(codes) == 0 {
		return text
	}
	var b strings.Builder
	for _, c := range codes {
		b.WriteString(c)
	}
	b.WriteString(text)
	b.WriteString(ansiReset)
	return b.String()
}

func StripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inEscape := false
	for i := 0; i < len(s); i++ {
		if inEscape {
			if (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') {
				inEscape = false
			}
			continue
		}
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			inEscape = true
			i++
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func VisibleWidth(s string) int {
	return utf8.RuneCountInString(StripANSI(s))
}

package templates

import (
	"strings"

	"github.com/knq/chromedp/cmd/chromedp-gen/internal"
)

const (
	commentWidth  = 80
	commentPrefix = `// `
)

// formatComment formats a comment.
func formatComment(s, chop, newstr string) string {
	s = strings.TrimPrefix(s, chop)
	s = internal.CodeRE.ReplaceAllString(s, "")

	l := len(s)
	if newstr != "" && l > 0 {
		s = strings.ToLower(s[:1]) + s[1:]
	}
	s = newstr + strings.TrimSuffix(s, ".")
	if l < 1 {
		s += "[no description]"
	}
	s += "."

	return wrap(s, commentWidth-len(commentPrefix), commentPrefix)
}

// wrap wraps a line of text to the specified width, and adding the prefix to
// each wrapped line.
func wrap(s string, width int, prefix string) string {
	words := strings.Fields(strings.TrimSpace(s))
	if len(words) == 0 {
		return s
	}

	wrapped := prefix + words[0]
	spaceLeft := width - len(wrapped)
	for _, word := range words[1:] {
		if len(word)+1 > spaceLeft {
			wrapped += "\n" + prefix + word
			spaceLeft = width - len(word)
		} else {
			wrapped += " " + word
			spaceLeft -= 1 + len(word)
		}
	}

	return wrapped
}

// Package templates contains the valyala/quicktemplate based code generation
// templates used by chromedp-gen.
package templates

import (
	"strings"
	"unicode"

	"github.com/knq/chromedp/cmd/chromedp-gen/internal"
	"github.com/knq/snaker"
)

const (
	commentWidth  = 80
	commentPrefix = `// `
)

var toUpper = map[string]bool{
	"DOM": true,
	"X":   true,
	"Y":   true,
}

var keep = map[string]bool{
	"JavaScript": true,
}

var badHTMLReplacer = strings.NewReplacer(
	"&lt;", "<",
	"&gt;", ">",
	"&gt", ">",
)

// formatComment formats a comment.
func formatComment(s, chop, newstr string) string {
	s = strings.TrimPrefix(s, chop)
	s = strings.TrimSpace(internal.CodeRE.ReplaceAllString(s, ""))
	s = badHTMLReplacer.Replace(s)

	l := len(s)
	if newstr != "" && l > 0 {
		if i := strings.IndexFunc(s, unicode.IsSpace); i != -1 {
			firstWord, remaining := s[:i], s[i:]
			if snaker.IsInitialism(firstWord) || toUpper[firstWord] {
				s = strings.ToUpper(firstWord)
			} else if keep[firstWord] {
				s = firstWord
			} else {
				s = strings.ToLower(firstWord[:1]) + firstWord[1:]
			}
			s += remaining
		}
	}
	s = newstr + strings.TrimSuffix(s, ".")
	if l < 1 {
		s += "[no description]"
	}
	s += "."

	s, _ = internal.MisspellReplacer.Replace(s)

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

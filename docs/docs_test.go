package docs_test

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// These tests read the documents and need no browser. The repository root is
// the parent of this directory.
const root = ".."

var (
	// markdownLink matches a relative link to a file and leaves a web address
	// alone.
	markdownLink = regexp.MustCompile(`\]\((?:\./)?([^)#:\s]+)(?:#[^)]*)?\)`)
	// decisionHead matches the first lines of a decision file.
	decisionHead = regexp.MustCompile(`\A# (.+)\n\nStatus: (.+)\.\n`)
	// decisionName matches the name of a decision file, a date and a slug.
	decisionName = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})-[a-z0-9-]+\.md$`)
)

func TestMarkdownLinksResolve(t *testing.T) {
	for _, path := range markdown(t) {
		for _, m := range markdownLink.FindAllStringSubmatch(read(t, path), -1) {
			if _, err := os.Stat(filepath.Join(filepath.Dir(path), m[1])); err != nil {
				t.Errorf("%s: the link to %s does not resolve", path, m[1])
			}
		}
	}
}

func TestDecisionIndexIsComplete(t *testing.T) {
	index := read(t, filepath.Join(root, "docs", "decisions", "README.md"))
	entries, err := os.ReadDir(filepath.Join(root, "docs", "decisions"))
	if err != nil {
		t.Fatal(err)
	}
	var count int
	for _, e := range entries {
		if e.Name() == "README.md" {
			continue
		}
		count++
		name := decisionName.FindStringSubmatch(e.Name())
		if name == nil {
			t.Errorf("%s: a decision file is named YYYY-MM-DD-short-slug.md", e.Name())
			continue
		}
		m := decisionHead.FindStringSubmatch(read(t, filepath.Join(root, "docs", "decisions", e.Name())))
		if m == nil {
			t.Errorf("%s: a decision opens with a title line, a blank line and \"Status: <status>.\"", e.Name())
			continue
		}
		row := fmt.Sprintf("| %s | [%s](%s) | %s |", name[1], m[1], e.Name(), m[2])
		if !strings.Contains(index, row+"\n") {
			t.Errorf("the index has no exact row for %s. Add:\n%s", e.Name(), row)
		}
	}
	if rows := strings.Count(index, "\n| 20"); rows != count {
		t.Errorf("the index has %d rows and there are %d decision files", rows, count)
	}
}

func TestRootHoldsFiveDocuments(t *testing.T) {
	want := map[string]bool{"README.md": true, "AGENTS.md": true, "CLAUDE.md": true, "CONTRIBUTING.md": true, "LICENSE": true}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !(strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".txt") || strings.HasPrefix(name, "LICENSE")) {
			continue
		}
		if !want[name] {
			t.Errorf("%s is in the repository root. Move it to docs/", name)
		}
		delete(want, name)
	}
	for name := range want {
		t.Errorf("the repository root has no %s", name)
	}
}

func TestClaudeImportsAgents(t *testing.T) {
	if got := strings.TrimSpace(read(t, filepath.Join(root, "CLAUDE.md"))); got != "@AGENTS.md" {
		t.Errorf("CLAUDE.md holds %q, and it must hold only @AGENTS.md", got)
	}
}

func TestSkillsAreCopies(t *testing.T) {
	agents := files(t, filepath.Join(root, ".agents", "skills"))
	claude := files(t, filepath.Join(root, ".claude", "skills"))
	for _, name := range []string{"go-pedantry", "simple-english"} {
		if _, ok := agents[filepath.Join(name, "SKILL.md")]; !ok {
			t.Errorf(".agents/skills/%s has no SKILL.md", name)
		}
	}
	for name, a := range agents {
		c, ok := claude[name]
		switch {
		case !ok:
			t.Errorf(".claude/skills/%s is missing", name)
		case !bytes.Equal(a, c):
			t.Errorf(".claude/skills/%s differs from .agents/skills/%s", name, name)
		}
	}
	for name := range claude {
		if _, ok := agents[name]; !ok {
			t.Errorf(".agents/skills/%s is missing", name)
		}
	}
}

// proseRules are the simple-english rules that a regular expression can check.
var proseRules = map[string]*regexp.Regexp{
	"modal":       regexp.MustCompile(`(?i)\b(?:should|would|may|might|could)\b`),
	"semicolon":   regexp.MustCompile(`;`),
	"dash":        regexp.MustCompile(`\x{2014}|\x{2013}|\s--\s`),
	"contraction": regexp.MustCompile(`(?i)\b[a-z]+n't\b|\b(?:it|that|there|here|what|let|he|she|who)'s\b|\b(?:i|you|we|they|it|that)'(?:re|ve|ll|d|m)\b`),
	"bold":        regexp.MustCompile(`\*\*`),
	"perfect":     regexp.MustCompile(`(?i)\b(?:has|have) been\b`),
	"spelling":    regexp.MustCompile(`(?i)\b(?:licence[sd]?|behaviours?|colours?|favour\w*|centres?|catalogue[ds]?|cancell(?:ed|ing)|grey|whilst|amongst|towards|(?:organi|recogni|normali|initiali|optimi|summari|standardi)s(?:e|es|ed|ing|ation))\b`),
}

// notProse matches what the rules do not apply to: fenced code, code spans,
// quotations, link targets and web addresses.
var notProse = regexp.MustCompile("(?s)```.*?```|`[^`\n]*`|\"[^\"\n]*\"|\\]\\([^)]*\\)|https?://\\S+")

func TestProseIsSimpleEnglish(t *testing.T) {
	for _, path := range markdown(t) {
		if filepath.Base(path) == "README.md" && filepath.Dir(path) == root {
			continue
		}
		text := notProse.ReplaceAllStringFunc(read(t, path), func(s string) string {
			return strings.Map(func(r rune) rune {
				if r == '\n' {
					return r
				}
				return ' '
			}, s)
		})
		for n, line := range strings.Split(text, "\n") {
			for name, re := range proseRules {
				for _, m := range re.FindAllString(line, -1) {
					t.Errorf("%s:%d: %s %q", path, n+1, name, m)
				}
			}
		}
	}
}

// markdown returns every document that this project writes. The skills come
// from elsewhere and are skipped.
func markdown(t *testing.T) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && (d.Name() == ".git" || d.Name() == ".agents" || d.Name() == ".claude" || d.Name() == "testdata"):
			return filepath.SkipDir
		case strings.HasSuffix(path, ".md"):
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// files returns the contents of every file under dir by relative path, and
// fails on a symbolic link.
func files(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := make(map[string][]byte)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			t.Errorf("%s is a symbolic link. Copy the file instead", path)
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		out[rel] = []byte(read(t, path))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

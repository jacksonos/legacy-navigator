package cobol

import (
	"path"
	"strings"
)

type copier struct {
	files  map[string]string
	byKey  map[string]string // upper basename without extension -> file name
	copied map[string]bool   // files that were expanded somewhere
	cache  map[string][]Token
}

func baseKey(name string) string {
	b := path.Base(strings.ReplaceAll(name, "\\", "/"))
	return strings.ToUpper(strings.TrimSuffix(b, path.Ext(b)))
}

func (c *copier) tokens(file string) []Token {
	t, ok := c.cache[file]
	if !ok {
		t = tokenize(file, c.files[file])
		c.cache[file] = t
	}
	return t
}

// expand replaces every COPY statement by the tokens of its copybook, which
// keep their own Loc. stack holds the copybooks being expanded (recursion guard).
func (c *copier) expand(toks []Token, stack []string, warns *[]Warning) []Token {
	var out []Token
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if t.Text != "COPY" || i+1 >= len(toks) {
			out = append(out, t)
			continue
		}
		j := i + 1
		replacing := false
		for j < len(toks) && toks[j].Text != "." {
			replacing = replacing || toks[j].Text == "REPLACING"
			j++
		}
		name := strings.Trim(toks[i+1].Text, `"'`)
		key := strings.ToUpper(name)
		i = j // the statement's period is dropped
		file, ok := c.byKey[key]
		switch {
		case !ok:
			*warns = append(*warns, Warning{WMissingCopy, name, t.Loc})
			continue
		case contains(stack, key):
			*warns = append(*warns, Warning{WUnsupported, "recursive COPY " + name + " not expanded", t.Loc})
			continue
		}
		if replacing {
			*warns = append(*warns, Warning{WUnsupported, "COPY " + name + " REPLACING not supported; expanded without replacement", t.Loc})
		}
		c.copied[file] = true
		out = append(out, c.expand(c.tokens(file), append(stack[:len(stack):len(stack)], key), warns)...)
	}
	return out
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

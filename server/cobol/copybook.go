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

// replRule is one REPLACING clause: from -> to over whole tokens, or over a
// word prefix/suffix when mode is LEADING/TRAILING.
type replRule struct {
	mode     string // "", "LEADING" or "TRAILING"
	from, to []string
}

// operand reads a ==pseudo text== (words between the == delimiters, possibly
// spanning tokens and lines) or a single word at toks[k].
func operand(toks []Token, k int) (words []string, next int, ok bool) {
	if k >= len(toks) || toks[k].Text == "." {
		return nil, k, false
	}
	t := toks[k].Text
	if !strings.HasPrefix(t, "==") {
		return []string{t}, k + 1, true
	}
	t = t[2:]
	for {
		if strings.HasSuffix(t, "==") {
			if t = strings.TrimSuffix(t, "=="); t != "" {
				words = append(words, t)
			}
			return words, k + 1, true
		}
		if t != "" {
			words = append(words, t)
		}
		if k++; k >= len(toks) {
			return nil, k, false
		}
		t = toks[k].Text
	}
}

// parseReplacing reads the clauses after REPLACING and returns the index of
// the statement's period (or the first token it could not understand).
func parseReplacing(toks []Token, k int) ([]replRule, int) {
	var rules []replRule
	for k < len(toks) && toks[k].Text != "." {
		r := replRule{}
		if m := toks[k].Text; m == "LEADING" || m == "TRAILING" {
			r.mode = m
			k++
		}
		var ok bool
		if r.from, k, ok = operand(toks, k); !ok || k >= len(toks) || toks[k].Text != "BY" {
			break
		}
		if r.to, k, ok = operand(toks, k+1); !ok {
			break
		}
		rules = append(rules, r)
	}
	return rules, k
}

// replace applies rules to toks; every replacement keeps the Loc of the
// token it replaces (the first one, for a multi-token match).
func replace(toks []Token, rules []replRule) []Token {
	out := make([]Token, 0, len(toks))
	sub := func(at Token, words []string) {
		for _, w := range words {
			out = append(out, Token{w, at.Loc, at.Col})
		}
	}
next:
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		for _, r := range rules {
			switch n := len(r.from); {
			case r.mode == "":
				if n == 0 || i+n > len(toks) {
					continue
				}
				ok := true
				for k, w := range r.from {
					ok = ok && toks[i+k].Text == w
				}
				if ok {
					sub(t, r.to)
					i += n - 1
					continue next
				}
				// NAME(1) is NAME followed by a subscript
				if b, _, cut := strings.Cut(t.Text, "("); cut && n == 1 && len(r.to) == 1 && b == r.from[0] {
					out = append(out, Token{r.to[0] + t.Text[len(b):], t.Loc, t.Col})
					continue next
				}
			case n == 1 && len(r.to) <= 1 && t.Text != "." && !isLiteral(t.Text):
				p, q := r.from[0], strings.Join(r.to, "")
				switch {
				case r.mode == "LEADING" && strings.HasPrefix(t.Text, p):
					t.Text = q + t.Text[len(p):]
				case r.mode == "TRAILING" && strings.HasSuffix(t.Text, p):
					t.Text = t.Text[:len(t.Text)-len(p)] + q
				default:
					continue
				}
				if t.Text != "" {
					out = append(out, t)
				}
				continue next
			}
		}
		out = append(out, t)
	}
	return out
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
		j := i + 2
		if j < len(toks) && (toks[j].Text == "OF" || toks[j].Text == "IN") {
			j += 2
		}
		var rules []replRule
		if j < len(toks) && toks[j].Text == "REPLACING" {
			rules, j = parseReplacing(toks, j+1)
		}
		for j < len(toks) && toks[j].Text != "." {
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
		c.copied[file] = true
		body := c.expand(c.tokens(file), append(stack[:len(stack):len(stack)], key), warns)
		if len(rules) > 0 {
			body = replace(body, rules)
		}
		out = append(out, body...)
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

package cobol

import "strings"

// Token is a word, a quoted literal or "." with its original position.
// Words are upper-cased; literals keep their quotes and case.
type Token struct {
	Text string
	Loc  Loc
	Col  int // 1-based original column
}

// formatDirective recognizes >>SOURCE [FORMAT [IS]] FREE|FIXED and $SET SOURCEFORMAT"FREE|FIXED".
func formatDirective(line string) (free, ok bool) {
	u := strings.ToUpper(strings.TrimSpace(line))
	if !strings.HasPrefix(u, ">>SOURCE") && !(strings.HasPrefix(u, "$SET") && strings.Contains(u, "SOURCEFORMAT")) {
		return false, false
	}
	switch {
	case strings.Contains(u, "FREE"):
		return true, true
	case strings.Contains(u, "FIXED"):
		return false, true
	}
	return false, false
}

// stripComment cuts a floating "*>" comment (outside literals) to the end of the line.
func stripComment(s string) string {
	var q byte
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case q != 0:
			if c == q {
				q = 0
			}
		case c == '\'' || c == '"':
			q = c
		case c == '*' && i+1 < len(s) && s[i+1] == '>':
			return s[:i]
		}
	}
	return s
}

// lit scans a quoted literal body from p; a doubled quote is an escape.
func lit(s string, p int, q byte) (end int, closed bool) {
	for p < len(s) {
		if s[p] == q {
			if p+1 < len(s) && s[p+1] == q {
				p += 2
				continue
			}
			return p + 1, true
		}
		p++
	}
	return p, false
}

// tokenize applies the fixed-format rules (cols 1-6 and 73+ ignored, col 7
// '*' '/' 'D' comment/debug, '-' continuation) until a free-format directive
// switches to free format (no columns); "*>" comments work in both.
func tokenize(file, src string) []Token {
	var toks []Token
	var open byte // quote of a literal left unterminated on the previous line
	free := false
	for n, raw := range strings.Split(src, "\n") {
		line := strings.ReplaceAll(strings.TrimRight(raw, "\r"), "\t", "    ")
		if f, ok := formatDirective(line); ok {
			free = f
			continue
		}
		ind, text := byte(' '), line
		if !free {
			if len(line) < 7 {
				continue
			}
			ind = line[6]
			if ind == '*' || ind == '/' || ind == 'D' || ind == 'd' {
				continue
			}
			text = line[7:]
			if len(text) > 65 {
				text = text[:65]
			}
		}
		text = stripComment(text)
		j := 0
		if ind == '-' {
			for j < len(text) && text[j] == ' ' {
				j++
			}
			if open != 0 && j < len(text) && text[j] == open && len(toks) > 0 {
				end, closed := lit(text, j+1, open)
				toks[len(toks)-1].Text += text[j+1 : end]
				if closed {
					open = 0
				}
				j = end
			}
		} else {
			open = 0
		}
		for j < len(text) {
			c := text[j]
			if c == ' ' || ((c == ',' || c == ';') && (j+1 == len(text) || text[j+1] == ' ')) {
				j++
				continue
			}
			loc := Loc{file, n + 1}
			if c == '\'' || c == '"' {
				end, closed := lit(text, j+1, c)
				toks = append(toks, Token{text[j:end], loc, 8 + j})
				if !closed {
					open = c
				}
				j = end
				continue
			}
			k := j
			for k < len(text) && text[k] != ' ' {
				k++
			}
			w := strings.ToUpper(text[j:k])
			dot := len(w) > 1 && strings.HasSuffix(w, ".")
			if dot {
				w = w[:len(w)-1]
			}
			toks = append(toks, Token{w, loc, 8 + j})
			if dot {
				toks = append(toks, Token{".", loc, 8 + k - 1})
			}
			j = k
		}
	}
	return toks
}

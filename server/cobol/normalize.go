package cobol

import "strings"

// Token is a word, a quoted literal or "." with its original position.
// Words are upper-cased; literals keep their quotes and case.
type Token struct {
	Text string
	Loc  Loc
	Col  int // 1-based original column
}

func isFreeFormat(src string) bool {
	for _, l := range strings.Split(src, "\n") {
		u := strings.ToUpper(strings.TrimSpace(l))
		if strings.HasPrefix(u, ">>SOURCE") && strings.Contains(u, "FREE") {
			return true
		}
	}
	return false
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

// tokenize applies the fixed-format rules: cols 1-6 and 73+ ignored, col 7
// '*' '/' 'D' comment/debug, '-' continuation.
func tokenize(file, src string) []Token {
	var toks []Token
	var open byte // quote of a literal left unterminated on the previous line
	for n, raw := range strings.Split(src, "\n") {
		line := strings.ReplaceAll(strings.TrimRight(raw, "\r"), "\t", "    ")
		if len(line) < 7 {
			continue
		}
		ind := line[6]
		if ind == '*' || ind == '/' || ind == 'D' || ind == 'd' {
			continue
		}
		text := line[7:]
		if len(text) > 65 {
			text = text[:65]
		}
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

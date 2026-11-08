package main

import (
	"regexp"
	"strconv"
	"strings"
)

type Citation struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	EndLine int    `json:"endLine"`
}

var (
	bracketRe = regexp.MustCompile(`\[([^\[\]]+)\]`)
	citeRe    = regexp.MustCompile(`^(\S+?):(\d+)(?:-(\d+))?$`)
	spacesRe  = regexp.MustCompile(`[ \t]{2,}`)
	prepunct  = regexp.MustCompile(`[ \t]+([.,;:])`)
)

// validateCitations keeps the [file:line] / [file:a-b] citations that fall inside one evidence range.
// Invalid ones are removed from the text and returned raw in unverified. Brackets that are not
// citations are left alone; in a bracket listing several, only the valid ones stay.
func validateCitations(answer string, ev []Evidence) (clean string, valid []Citation, unverified []string) {
	valid, unverified = []Citation{}, []string{}
	seen := map[Citation]bool{}
	removed := false
	clean = bracketRe.ReplaceAllStringFunc(answer, func(m string) string {
		items := strings.Split(m[1:len(m)-1], ",")
		var keep []string
		var found []Citation
		for _, it := range items {
			g := citeRe.FindStringSubmatch(strings.TrimSpace(it))
			if g == nil {
				return m // not a citation list
			}
			a, _ := strconv.Atoi(g[2])
			b := a
			if g[3] != "" {
				b, _ = strconv.Atoi(g[3])
			}
			c := Citation{g[1], a, b}
			if inEvidence(c, ev) {
				keep = append(keep, g[0])
				found = append(found, c)
			} else {
				unverified = append(unverified, g[0])
			}
		}
		for _, c := range found {
			if !seen[c] {
				seen[c] = true
				valid = append(valid, c)
			}
		}
		if len(keep) == len(items) {
			return "[" + strings.Join(keep, ", ") + "]"
		}
		removed = true
		if len(keep) == 0 {
			return ""
		}
		return "[" + strings.Join(keep, ", ") + "]"
	})
	if removed {
		clean = strings.TrimSpace(prepunct.ReplaceAllString(spacesRe.ReplaceAllString(clean, " "), "$1"))
	}
	return
}

func inEvidence(c Citation, ev []Evidence) bool {
	if c.Line < 1 || c.EndLine < c.Line {
		return false
	}
	for _, e := range ev {
		if e.File == c.File && c.Line >= e.StartLine && c.EndLine <= e.EndLine {
			return true
		}
	}
	return false
}

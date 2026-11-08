package main

import (
	"reflect"
	"testing"
)

func TestValidateCitations(t *testing.T) {
	ev := []Evidence{{File: "A.cbl", StartLine: 10, EndLine: 20}, {File: "B.cpy", StartLine: 1, EndLine: 3}}
	cases := []struct {
		in, out string
		valid   []Citation
		unv     []string
	}{
		{"Calcula [A.cbl:12] y [A.cbl:12].", "Calcula [A.cbl:12] y [A.cbl:12].", []Citation{{"A.cbl", 12, 12}}, []string{}},
		{"Rango [A.cbl:11-15] ok", "Rango [A.cbl:11-15] ok", []Citation{{"A.cbl", 11, 15}}, []string{}},
		{"Fuera [A.cbl:5] fin", "Fuera fin", []Citation{}, []string{"A.cbl:5"}},
		{"Cruza [A.cbl:18-25].", "Cruza.", []Citation{}, []string{"A.cbl:18-25"}},
		{"Falso [NOPE.cbl:1] y [x] y [B.cpy:2]", "Falso y [x] y [B.cpy:2]", []Citation{{"B.cpy", 2, 2}}, []string{"NOPE.cbl:1"}},
		{"Lista [A.cbl:12, B.cpy:9] fin", "Lista [A.cbl:12] fin", []Citation{{"A.cbl", 12, 12}}, []string{"B.cpy:9"}},
		{"Invertido [A.cbl:15-12]", "Invertido", []Citation{}, []string{"A.cbl:15-12"}},
	}
	for _, c := range cases {
		out, valid, unv := validateCitations(c.in, ev)
		if out != c.out || !reflect.DeepEqual(valid, c.valid) || !reflect.DeepEqual(unv, c.unv) {
			t.Errorf("%q -> %q %v %v", c.in, out, valid, unv)
		}
	}
}

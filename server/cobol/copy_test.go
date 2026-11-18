package cobol

import (
	"reflect"
	"strconv"
	"testing"
)

func TestCopyReplacing(t *testing.T) {
	main := fx("IDENTIFICATION DIVISION.", "PROGRAM-ID. T.", "DATA DIVISION.", "WORKING-STORAGE SECTION.", // 1-4
		"COPY CB REPLACING ==FLD== BY ==FIELD==",            // 5
		"    ==PIC 9",                                       // 6
		"    VALUE 0== BY ==PIC X==.",                       // 7: pseudo-text spanning lines
		"COPY CB2 REPLACING LEADING ==TMP-== BY ==NEW-==.",  // 8
		"COPY CB3 REPLACING OLD-NAME BY NEW-NAME.",          // 9
		"COPY CB4 REPLACING TRAILING ==-OLD== BY ==-NEW==.", // 10
		"PROCEDURE DIVISION.",                               // 11
		"P1.",                                               // 12
		"    MOVE FIELD TO NEW-X.",                          // 13
		"    MOVE A-NEW TO NEW-NAME.",                       // 14
	)
	proj := Analyze(map[string]string{
		"T.cbl":   main,
		"CB.cpy":  fx("01  FLD PIC 9 VALUE 0.", "01  OTHER-FLD PIC X."),
		"CB2.cpy": fx("01  TMP-X PIC X.", "01  KEEP-TMP-Y PIC X."),
		"CB3.cpy": fx("01  OLD-NAME PIC X.", "01  OLD-NAME-2 PIC X."),
		"CB4.cpy": fx("01  A-OLD PIC X."),
	})
	if len(proj.Warnings) != 0 {
		t.Fatalf("warnings = %+v", proj.Warnings)
	}
	p := program(t, proj, "T")
	var got []string
	for _, d := range p.Data {
		got = append(got, d.Name+":"+d.Pic+"@"+d.Loc.File+":"+strconv.Itoa(d.Loc.Line))
	}
	want := []string{
		"FIELD:X@CB.cpy:1", "OTHER-FLD:X@CB.cpy:2",
		"NEW-X:X@CB2.cpy:1", "KEEP-TMP-Y:X@CB2.cpy:2",
		"NEW-NAME:X@CB3.cpy:1", "OLD-NAME-2:X@CB3.cpy:2",
		"A-NEW:X@CB4.cpy:1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("data = %v\nwant   %v", got, want)
	}
	for _, c := range []struct {
		name, kind string
		want       []int
	}{
		{"FIELD", "read", []int{13}}, {"NEW-X", "write", []int{13}},
		{"A-NEW", "read", []int{14}}, {"NEW-NAME", "write", []int{14}},
		{"FIELD", "def", []int{1}},
	} {
		file := "T.cbl"
		if c.kind == "def" {
			file = "CB.cpy"
		}
		if got := lines(p, c.name, c.kind, file); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s %s = %v, want %v", c.name, c.kind, got, c.want)
		}
	}
}

package cobol

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestFreeFormat(t *testing.T) {
	for _, dir := range []string{">>SOURCE FORMAT FREE", ">>source free", `$SET SOURCEFORMAT"FREE"`} {
		src := strings.Join([]string{
			dir,                                         // 1
			"identification division.",                  // 2
			"program-id. t. *> trailing comment",        // 3
			"data division.",                            // 4
			"working-storage section.",                  // 5
			"01 ws-a pic x. *> 01 ws-hidden pic x.",     // 6
			"*> 01 ws-c pic x.",                         // 7
			"01 ws-b pic x.",                            // 8
			"procedure division.",                       // 9
			"main.",                                     // 10
			"    move ws-a to ws-b *> to ws-c",          // 11
			"    perform sub.",                          // 12
			"    stop run.",                             // 13
			"sub.",                                      // 14
			`    display "*> not a comment" ws-a.`,      // 15
			">>SOURCE FORMAT FIXED",                     // 16
			"       SUB2.",                              // 17
			"           MOVE WS-B TO WS-A. *> floating", // 18
			"      * fixed comment MOVE WS-B TO WS-HIDDEN.", // 19
		}, "\n") + "\n"
		proj := Analyze(map[string]string{"t.cbl": src})
		if len(proj.Warnings) != 0 {
			t.Fatalf("%s: warnings %+v", dir, proj.Warnings)
		}
		p := program(t, proj, "T")
		var data, paras []string
		for _, d := range p.Data {
			data = append(data, d.Name+"@"+strconv.Itoa(d.Loc.Line))
		}
		for _, b := range p.Paragraphs {
			paras = append(paras, b.Name+"@"+strconv.Itoa(b.Loc.Line))
		}
		if want := []string{"WS-A@6", "WS-B@8"}; !reflect.DeepEqual(data, want) {
			t.Errorf("%s: data = %v", dir, data)
		}
		if want := []string{"MAIN@10", "SUB@14", "SUB2@17"}; !reflect.DeepEqual(paras, want) {
			t.Errorf("%s: paragraphs = %v", dir, paras)
		}
		for _, c := range []struct {
			name, kind string
			want       []int
		}{
			{"WS-A", "read", []int{11, 15}}, {"WS-A", "write", []int{18}},
			{"WS-B", "write", []int{11}}, {"WS-B", "read", []int{18}},
		} {
			if got := lines(p, c.name, c.kind, "t.cbl"); !reflect.DeepEqual(got, c.want) {
				t.Errorf("%s: %s %s = %v, want %v", dir, c.name, c.kind, got, c.want)
			}
		}
		if got := edgeLines(p, "MAIN", "SUB", "perform"); !reflect.DeepEqual(got, []int{12}) {
			t.Errorf("%s: perform edge = %v", dir, got)
		}
	}
}

func TestFixedFloatingComment(t *testing.T) {
	src := fx("IDENTIFICATION DIVISION.", "PROGRAM-ID. T.", "DATA DIVISION.", "WORKING-STORAGE SECTION.",
		"01  A PIC X. *> 01 HIDDEN PIC X.", "01  B PIC X VALUE '*> kept'.", "PROCEDURE DIVISION.", "P1.", "    MOVE A TO B. *> MOVE A TO HIDDEN.")
	p := program(t, Analyze(map[string]string{"T.cbl": src}), "T")
	var names []string
	for _, d := range p.Data {
		names = append(names, d.Name)
	}
	if !reflect.DeepEqual(names, []string{"A", "B"}) {
		t.Errorf("data = %v", names)
	}
	if got := lines(p, "B", "write", "T.cbl"); !reflect.DeepEqual(got, []int{9}) {
		t.Errorf("B write = %v", got)
	}
}

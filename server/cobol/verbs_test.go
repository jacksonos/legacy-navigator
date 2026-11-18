package cobol

import (
	"reflect"
	"strings"
	"testing"
)

// stmtRefs parses one statement inside a program that defines AA..HH and
// returns its non-def refs as "NAME:kind", in source order.
func stmtRefs(t *testing.T, stmt string) []string {
	t.Helper()
	var ls []string
	ls = append(ls, "IDENTIFICATION DIVISION.", "PROGRAM-ID. T.", "DATA DIVISION.", "WORKING-STORAGE SECTION.") // 1-4
	for _, n := range strings.Fields("AA BB CC DD EE FF GG HH") {
		ls = append(ls, "01  "+n+" PIC X.") // 5-12
	}
	ls = append(ls, "PROCEDURE DIVISION.", "P1.") // 13-14
	for _, l := range strings.Split(stmt+".", "\n") {
		ls = append(ls, "    "+l) // from line 15
	}
	p := program(t, Analyze(map[string]string{"T.cbl": fx(ls...)}), "T")
	got := []string{}
	for _, r := range p.Refs {
		if r.Kind != "def" && r.Loc.Line >= 15 {
			got = append(got, r.Name+":"+r.Kind)
		}
	}
	return got
}

func TestVerbOperandModes(t *testing.T) {
	for _, c := range []struct {
		stmt string
		want []string
	}{
		// INSPECT
		{"INSPECT AA REPLACING ALL 'X' BY 'Y'", []string{"AA:write"}},
		{"INSPECT AA CONVERTING BB TO CC", []string{"AA:write", "BB:read", "CC:read"}},
		{"INSPECT AA TALLYING BB FOR ALL CC", []string{"AA:read", "BB:write", "CC:read"}},
		{"INSPECT AA TALLYING BB FOR ALL 'A' REPLACING ALL 'A' BY CC", []string{"AA:write", "BB:write", "CC:read"}},
		// SET
		{"SET AA TO BB", []string{"AA:write", "BB:read"}},
		{"SET AA UP BY BB", []string{"AA:write", "BB:read"}},
		{"SET AA DOWN BY BB", []string{"AA:write", "BB:read"}},
		{"SET AA BB TO TRUE", []string{"AA:write", "BB:write"}},
		// SEARCH ... VARYING, with WHEN/AT END phrases resetting the mode
		{"SEARCH AA VARYING BB AT END MOVE CC TO DD\nWHEN EE = FF MOVE GG TO HH",
			[]string{"AA:read", "BB:write", "CC:read", "DD:write", "EE:read", "FF:read", "GG:read", "HH:write"}},
		// sort/merge record verbs
		{"RETURN AA INTO BB", []string{"AA:read", "BB:write"}},
		{"RELEASE AA FROM BB", []string{"AA:read", "BB:read"}},
		// DIVIDE ... REMAINDER
		{"DIVIDE AA INTO BB GIVING CC REMAINDER DD", []string{"AA:read", "BB:read", "CC:write", "DD:write"}},
		{"DIVIDE AA BY BB GIVING CC REMAINDER DD", []string{"AA:read", "BB:read", "CC:write", "DD:write"}},
		// STRING / UNSTRING
		{"STRING AA DELIMITED BY SIZE INTO BB WITH POINTER CC", []string{"AA:read", "BB:write", "CC:write"}},
		{"UNSTRING AA DELIMITED BY ',' INTO BB CC\nWITH POINTER DD TALLYING IN EE",
			[]string{"AA:read", "BB:write", "CC:write", "DD:write", "EE:write"}},
		// CALL / ACCEPT
		{"CALL 'SUB' USING AA BB RETURNING CC", []string{"AA:read", "BB:read", "CC:write"}},
		{"ACCEPT AA FROM DATE", []string{"AA:write"}},
		{"ACCEPT AA FROM ENVIRONMENT BB", []string{"AA:write", "BB:read"}},
		// subscripts and reference modification: base takes the mode, inner names are read
		{"MOVE AA(BB) TO CC(DD EE)", []string{"AA:read", "BB:read", "CC:write", "DD:read", "EE:read"}},
		{"MOVE AA (BB) TO CC (DD:EE)", []string{"AA:read", "BB:read", "CC:write", "DD:read", "EE:read"}},
		{"MOVE AA(BB:CC) TO DD(1:EE)", []string{"AA:read", "BB:read", "CC:read", "DD:write", "EE:read"}},
		{"ADD AA TO BB(CC)", []string{"AA:read", "BB:write", "CC:read"}},
		{"COMPUTE AA(BB) = CC(DD)", []string{"AA:write", "BB:read", "CC:read", "DD:read"}},
		// existing rules keep working
		{"IF AA = BB MOVE CC TO DD ELSE MOVE EE TO FF END-IF", []string{"AA:read", "BB:read", "CC:read", "DD:write", "EE:read", "FF:write"}},
		{"READ AA INTO BB", []string{"AA:io", "BB:write"}},
		// regression: a verb that was not in the old list used to leave write mode on for the next statement
		{"MOVE AA TO BB GENERATE CC", []string{"AA:read", "BB:write", "CC:read"}},
		{"MOVE AA TO BB INITIATE CC", []string{"AA:read", "BB:write", "CC:read"}},
		{"COMPUTE AA = BB TERMINATE CC", []string{"AA:write", "BB:read", "CC:read"}},
		{"ACCEPT AA SUPPRESS BB", []string{"AA:write", "BB:read"}},
		{"MOVE AA TO BB INVOKE CC", []string{"AA:read", "BB:write", "CC:read"}},
	} {
		if got := stmtRefs(t, c.stmt); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s\n got  %v\n want %v", c.stmt, got, c.want)
		}
	}
}

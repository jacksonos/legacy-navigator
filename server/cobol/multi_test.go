package cobol

import (
	"reflect"
	"strconv"
	"testing"
)

func twoPrograms() string {
	return fx("IDENTIFICATION DIVISION.", "PROGRAM-ID. A.", "DATA DIVISION.", "WORKING-STORAGE SECTION.", // 1-4
		"01  X PIC X.", "PROCEDURE DIVISION.", "MAIN.", // 5-7
		"    MOVE 'A' TO X.", "    CALL 'B'.", "    STOP RUN.", "END PROGRAM A.", // 8-11
		"IDENTIFICATION DIVISION.", "PROGRAM-ID. B.", "DATA DIVISION.", "WORKING-STORAGE SECTION.", // 12-15
		"01  Y PIC X.", "PROCEDURE DIVISION.", "MAIN.", // 16-18
		"    MOVE 'B' TO Y.", "    GOBACK.", "END PROGRAM B.") // 19-21
}

func nestedPrograms() string {
	return fx("IDENTIFICATION DIVISION.", "PROGRAM-ID. OUTER.", "DATA DIVISION.", "WORKING-STORAGE SECTION.", // 1-4
		"01  X PIC X.", "PROCEDURE DIVISION.", "MAIN.", // 5-7
		"    CALL 'INNER'.", "    MOVE 'A' TO X.", "    STOP RUN.", // 8-10
		"IDENTIFICATION DIVISION.", "PROGRAM-ID. INNER.", "DATA DIVISION.", "WORKING-STORAGE SECTION.", // 11-14
		"01  Y PIC X.", "PROCEDURE DIVISION.", "MAIN.", // 15-17
		"    MOVE 'B' TO Y.", "    EXIT PROGRAM.", "END PROGRAM INNER.", "END PROGRAM OUTER.") // 18-21
}

func TestSeveralPrograms(t *testing.T) {
	type prog struct {
		id                 string
		start, idLine, end int
		paras              []string
		data               []string
		xw, yw             []int // write refs to X and Y inside the program
	}
	for _, c := range []struct {
		name  string
		src   string
		progs []prog
		calls []string // call edges "program:from>to@line"
	}{
		{"consecutive", twoPrograms(), []prog{
			{"A", 1, 2, 11, []string{"MAIN@7"}, []string{"X"}, []int{8}, []int{}},
			{"B", 12, 13, 21, []string{"MAIN@18"}, []string{"Y"}, []int{}, []int{19}},
		}, []string{"A:MAIN>B@9"}},
		{"nested", nestedPrograms(), []prog{
			{"OUTER", 1, 2, 21, []string{"MAIN@7"}, []string{"X"}, []int{9}, []int{}},
			{"INNER", 11, 12, 20, []string{"MAIN@17"}, []string{"Y"}, []int{}, []int{18}},
		}, []string{"OUTER:MAIN>INNER@8"}},
	} {
		proj := Analyze(map[string]string{"T.cbl": c.src})
		if len(proj.Warnings) != 0 || len(proj.Programs) != len(c.progs) {
			t.Fatalf("%s: programs %d, warnings %+v", c.name, len(proj.Programs), proj.Warnings)
		}
		var calls []string
		for i, w := range c.progs {
			p := proj.Programs[i]
			if p.ID != w.id || p.File != "T.cbl" || p.StartLine != w.start || p.IDLine != w.idLine || p.EndLine != w.end {
				t.Errorf("%s: program %d = %s %s %d/%d/%d", c.name, i, p.ID, p.File, p.StartLine, p.IDLine, p.EndLine)
			}
			var paras, data []string
			for _, b := range p.Paragraphs {
				paras = append(paras, b.Name+"@"+strconv.Itoa(b.Loc.Line))
			}
			for _, d := range p.Data {
				data = append(data, d.Name)
			}
			if !reflect.DeepEqual(paras, w.paras) || !reflect.DeepEqual(data, w.data) {
				t.Errorf("%s: %s paragraphs %v data %v", c.name, w.id, paras, data)
			}
			if got := lines(&p, "X", "write", "T.cbl"); !reflect.DeepEqual(got, w.xw) {
				t.Errorf("%s: %s X writes %v, want %v", c.name, w.id, got, w.xw)
			}
			if got := lines(&p, "Y", "write", "T.cbl"); !reflect.DeepEqual(got, w.yw) {
				t.Errorf("%s: %s Y writes %v, want %v", c.name, w.id, got, w.yw)
			}
			for _, e := range p.Edges {
				calls = append(calls, p.ID+":"+e.From+">"+e.To+"@"+strconv.Itoa(e.Loc.Line))
			}
		}
		if !reflect.DeepEqual(calls, c.calls) {
			t.Errorf("%s: edges %v, want %v", c.name, calls, c.calls)
		}
	}
}

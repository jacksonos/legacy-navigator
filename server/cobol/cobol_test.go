package cobol

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func samples(t *testing.T, omit ...string) map[string]string {
	t.Helper()
	m := map[string]string{}
	ents, err := os.ReadDir("../samples")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		skip := false
		for _, o := range omit {
			skip = skip || o == e.Name()
		}
		if skip {
			continue
		}
		b, err := os.ReadFile(filepath.Join("../samples", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		m[e.Name()] = string(b)
	}
	return m
}

func program(t *testing.T, proj *Project, id string) *Program {
	t.Helper()
	for i := range proj.Programs {
		if proj.Programs[i].ID == id {
			return &proj.Programs[i]
		}
	}
	t.Fatalf("program %s not found in %+v", id, proj.Programs)
	return nil
}

// lines returns the sorted lines of the refs matching name, kind and file.
func lines(p *Program, name, kind, file string) []int {
	out := []int{}
	for _, r := range p.Refs {
		if r.Name == name && r.Kind == kind && r.Loc.File == file {
			out = append(out, r.Loc.Line)
		}
	}
	sort.Ints(out)
	return out
}

func edgeLines(p *Program, from, to, kind string) []int {
	out := []int{}
	for _, e := range p.Edges {
		if e.From == from && e.To == to && e.Kind == kind {
			out = append(out, e.Loc.Line)
		}
	}
	return out
}

// fx builds fixed-format source: every line starts in col 8.
func fx(lines ...string) string {
	return "       " + strings.Join(lines, "\n       ") + "\n"
}

const hdr = "IDENTIFICATION DIVISION.\nPROGRAM-ID. T.\nDATA DIVISION.\nWORKING-STORAGE SECTION.\n"

func TestCalcsaldStructure(t *testing.T) {
	p := program(t, Analyze(samples(t)), "CALCSALD")
	if p.File != "CALCSALD.cbl" {
		t.Errorf("file = %s", p.File)
	}
	// Line numbers taken from `grep -n` over samples/CALCSALD.cbl.
	paras := []struct {
		name       string
		start, end int
	}{
		{"0000-PRINCIPAL", 45, 50},
		{"1000-INICIO", 52, 55},
		{"2000-LEER-MOV", 57, 63},
		{"2000-EXIT", 64, 65},
		{"3000-PROCESAR", 67, 75},
		{"4000-CALCULAR-SALDO", 77, 96},
		{"5000-ESCRIBIR-REPORTE", 98, 101},
		{"9000-FIN", 103, 107},
	}
	if len(p.Paragraphs) != len(paras) {
		t.Fatalf("got %d paragraphs: %+v", len(p.Paragraphs), p.Paragraphs)
	}
	for i, w := range paras {
		g := p.Paragraphs[i]
		if g.Name != w.name || g.Loc != (Loc{"CALCSALD.cbl", w.start}) || g.EndLine != w.end {
			t.Errorf("paragraph %d = %+v, want %+v", i, g, w)
		}
	}
	files := map[string]FileDecl{}
	for _, f := range p.Files {
		files[f.Name] = f
	}
	want := map[string]FileDecl{
		"MOVIMIENTOS": {"MOVIMIENTOS", "MOVIMIENTOS.DAT", "MOV-REGISTRO", Loc{"CALCSALD.cbl", 7}},
		"CUENTAS":     {"CUENTAS", "CUENTAS.DAT", "CTA-REGISTRO", Loc{"CALCSALD.cbl", 10}},
		"REPORTE":     {"REPORTE", "REPORTE.TXT", "REP-LINEA", Loc{"CALCSALD.cbl", 15}},
	}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("files = %+v", files)
	}
	data := map[string]DataItem{}
	for _, d := range p.Data {
		data[d.Name] = d
	}
	for _, c := range []DataItem{
		{"CTA-SALDO", 5, "CTA-REGISTRO", "S9(11)V99", Loc{"CTAREG.cpy", 5}},
		{"CTA-ACTIVA", 88, "CTA-ESTADO", "", Loc{"CTAREG.cpy", 7}},
		{"REP-SALDO", 5, "WS-LINEA-REP", "-(11)9.99", Loc{"CALCSALD.cbl", 43}},
		{"FIN-MOVIMIENTOS", 88, "WS-FIN-MOV", "", Loc{"CALCSALD.cbl", 33}},
		{"WS-FS-MOV", 1, "", "XX", Loc{"CALCSALD.cbl", 28}},
	} {
		if g := data[c.Name]; g != c {
			t.Errorf("data %s = %+v, want %+v", c.Name, g, c)
		}
	}
	if _, ok := data["FILLER"]; ok {
		t.Error("FILLER must be skipped")
	}
	if len(p.Sections) != 0 {
		t.Errorf("unexpected sections %+v", p.Sections)
	}
}

func TestCalcsaldRefs(t *testing.T) {
	p := program(t, Analyze(samples(t)), "CALCSALD")
	const cbl = "CALCSALD.cbl"
	for _, c := range []struct {
		name, kind, file string
		want             []int
	}{
		{"CTA-SALDO", "def", "CTAREG.cpy", []int{5}},
		{"CTA-SALDO", "write", cbl, []int{86, 88}},
		{"CTA-SALDO", "read", cbl, []int{86, 100}},
		{"MOV-IMPORTE", "read", cbl, []int{68, 85, 86, 88, 89}},
		{"MOV-IMPORTE", "write", cbl, []int{}},
		{"WS-CLAVE-CTA", "write", cbl, []int{78}},
		{"WS-CONTADOR", "write", cbl, []int{63}},
		{"WS-TOTAL-DEPOSITOS", "write", cbl, []int{85}},
		{"FIN-MOVIMIENTOS", "write", cbl, []int{60}},
		{"FIN-MOVIMIENTOS", "read", cbl, []int{48}},
		{"REP-CUENTA", "write", cbl, []int{99}},
		{"REP-SALDO", "write", cbl, []int{100}},
		{"WS-LINEA-REP", "read", cbl, []int{101}},
		{"CUENTAS", "io", cbl, []int{54, 79, 93, 105}},
		{"CTA-REGISTRO", "io", cbl, []int{93}},
		{"REPORTE", "io", cbl, []int{55, 101, 106}},
		{"REP-LINEA", "io", cbl, []int{101}},
		{"MOVIMIENTOS", "io", cbl, []int{53, 58, 104}},
		{"CUENTAS", "def", cbl, []int{10}},
	} {
		if got := lines(p, c.name, c.kind, c.file); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s %s in %s: got %v, want %v", c.name, c.kind, c.file, got, c.want)
		}
	}
	for _, r := range p.Refs {
		if r.Name == "CTA-SALDO" && r.Loc.Line == 86 && r.Kind == "write" && r.Paragraph != "4000-CALCULAR-SALDO" {
			t.Errorf("paragraph of write = %q", r.Paragraph)
		}
	}
}

func TestCalcsaldEdges(t *testing.T) {
	p := program(t, Analyze(samples(t)), "CALCSALD")
	for _, c := range []struct {
		from, to, kind string
		want           []int
	}{
		{"0000-PRINCIPAL", "1000-INICIO", "perform", []int{46}},
		{"0000-PRINCIPAL", "2000-LEER-MOV", "perform", []int{47}},
		{"0000-PRINCIPAL", "2000-EXIT", "perform", []int{47}},
		{"0000-PRINCIPAL", "3000-PROCESAR", "perform", []int{48}},
		{"0000-PRINCIPAL", "9000-FIN", "perform", []int{49}},
		{"3000-PROCESAR", "4000-CALCULAR-SALDO", "perform", []int{70}},
		{"3000-PROCESAR", "5000-ESCRIBIR-REPORTE", "perform", []int{71}},
		{"3000-PROCESAR", "2000-LEER-MOV", "perform", []int{75}},
		{"3000-PROCESAR", "2000-EXIT", "perform", []int{75}},
		{"2000-LEER-MOV", "2000-EXIT", "goto", []int{61}},
		{"3000-PROCESAR", "VALIDAR", "call", []int{68}},
		{"3000-PROCESAR", "?", "call", []int{73}},
	} {
		if got := edgeLines(p, c.from, c.to, c.kind); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s -%s-> %s: got %v, want %v", c.from, c.kind, c.to, got, c.want)
		}
	}
	for _, e := range p.Edges {
		if e.To == "?" && (!e.Dynamic || e.Kind != "call") {
			t.Errorf("dynamic call edge = %+v", e)
		}
		if e.To != "?" && (e.Dynamic || e.Uncertain) {
			t.Errorf("edge should be certain: %+v", e)
		}
	}
	if len(p.Edges) != 12 {
		t.Errorf("edges = %d, want 12: %+v", len(p.Edges), p.Edges)
	}
}

func warnAt(ws []Warning, kind string) []Warning {
	var out []Warning
	for _, w := range ws {
		if w.Kind == kind {
			out = append(out, w)
		}
	}
	return out
}

func TestSampleWarnings(t *testing.T) {
	proj := Analyze(samples(t))
	p := program(t, proj, "CALCSALD")
	if len(proj.Programs) != 2 {
		t.Errorf("programs = %d", len(proj.Programs))
	}
	mc := warnAt(p.Warnings, WMissingCopy)
	if len(mc) != 1 || mc[0].Detail != "AUDITREG" || mc[0].Loc != (Loc{"CALCSALD.cbl", 27}) {
		t.Errorf("missing_copy = %+v", mc)
	}
	dc := warnAt(p.Warnings, WDynamicCall)
	if len(dc) != 1 || dc[0].Loc != (Loc{"CALCSALD.cbl", 73}) || dc[0].Detail != "WS-RUTINA" {
		t.Errorf("dynamic_call = %+v", dc)
	}
	if len(p.Warnings) != 2 {
		t.Errorf("warnings = %+v", p.Warnings)
	}
	if len(proj.Warnings) != 0 {
		t.Errorf("project warnings with VALIDAR loaded: %+v", proj.Warnings)
	}

	proj = Analyze(samples(t, "VALIDAR.cbl"))
	uc := warnAt(proj.Warnings, WUnresolvedCall)
	if len(uc) != 1 || uc[0].Detail != "VALIDAR" || uc[0].Loc != (Loc{"CALCSALD.cbl", 68}) {
		t.Errorf("unresolved_call = %+v", proj.Warnings)
	}
}

func TestValidar(t *testing.T) {
	p := program(t, Analyze(samples(t)), "VALIDAR")
	if len(p.Paragraphs) != 1 || p.Paragraphs[0].Name != "0000-INICIO" || p.Paragraphs[0].Loc.Line != 11 || p.Paragraphs[0].EndLine != 17 {
		t.Errorf("paragraphs = %+v", p.Paragraphs)
	}
	if got := lines(p, "LK-VALIDO", "write", "VALIDAR.cbl"); !reflect.DeepEqual(got, []int{13}) {
		t.Errorf("LK-VALIDO writes = %v", got)
	}
	if got := lines(p, "LK-IMPORTE", "read", "VALIDAR.cbl"); !reflect.DeepEqual(got, []int{10, 12}) {
		t.Errorf("LK-IMPORTE reads = %v", got)
	}
}

func TestNormalizer(t *testing.T) {
	long := func(s string) string { return s + strings.Repeat(" ", 72-len(s)) }
	src := "000100 IDENTIFICATION DIVISION.\n" +
		"000200 PROGRAM-ID. T.\n" +
		"000300 DATA DIVISION.\n" +
		"000400 WORKING-STORAGE SECTION.\n" +
		"000500 01  A PIC X.\n" +
		"000600 01  B PIC X(40) VALUE 'ABC\n" +
		"000700-    'DEF'.\n" +
		"000800 01  IGN PIC X.\n" +
		"000900 PROCEDURE DIVISION.\n" +
		"001000 P1.\n" +
		"001100*    MOVE B TO A.\n" +
		"001200/    MOVE B TO A.\n" +
		"001300D    MOVE B TO A.\n" +
		long("001400     MOVE A TO B") + "IGN\n" +
		"001500\tMOVE\n" +
		"x\n" +
		"\n" +
		"001600     STOP RUN.\n"
	toks := tokenize("T.cbl", src)
	found := false
	for _, tk := range toks {
		if tk.Text == "'ABCDEF'" {
			found = tk.Loc == (Loc{"T.cbl", 6})
		}
	}
	if !found {
		t.Errorf("continued literal not joined: %+v", toks)
	}
	p := program(t, Analyze(map[string]string{"T.cbl": src}), "T")
	for _, r := range p.Refs {
		if r.Kind != "def" && r.Loc.Line != 14 {
			t.Errorf("unexpected ref %+v (comments/debug/col 73+ must be ignored)", r)
		}
	}
	if got := lines(p, "A", "read", "T.cbl"); !reflect.DeepEqual(got, []int{14}) {
		t.Errorf("A reads = %v", got)
	}
	if got := lines(p, "B", "write", "T.cbl"); !reflect.DeepEqual(got, []int{14}) {
		t.Errorf("B writes = %v", got)
	}
	if got := lines(p, "IGN", "read", "T.cbl"); len(got) != 0 {
		t.Errorf("text after col 72 was parsed: %v", got)
	}
}

func TestGoToDepending(t *testing.T) {
	src := fx("IDENTIFICATION DIVISION.", "PROGRAM-ID. T.", "DATA DIVISION.", "WORKING-STORAGE SECTION.",
		"01  N PIC 9.", "PROCEDURE DIVISION.", "P1.", "    GO TO A B", "        DEPENDING ON N.",
		"A.", "    GO TO B.", "B.", "    STOP RUN.")
	p := program(t, Analyze(map[string]string{"T.cbl": src}), "T")
	var unc []string
	for _, e := range p.Edges {
		if e.Uncertain {
			unc = append(unc, e.From+">"+e.To)
		}
	}
	if !reflect.DeepEqual(unc, []string{"P1>A", "P1>B"}) {
		t.Errorf("uncertain edges = %v (%+v)", unc, p.Edges)
	}
	w := warnAt(p.Warnings, WGotoDepending)
	if len(w) != 1 || w[0].Loc.Line != 8 {
		t.Errorf("warnings = %+v", p.Warnings)
	}
	if got := edgeLines(p, "A", "B", "goto"); !reflect.DeepEqual(got, []int{11}) {
		t.Errorf("plain goto = %v", got)
	}
	if got := lines(p, "N", "read", "T.cbl"); !reflect.DeepEqual(got, []int{9}) {
		t.Errorf("N reads = %v", got)
	}
}

func TestAlter(t *testing.T) {
	src := fx("IDENTIFICATION DIVISION.", "PROGRAM-ID. T.", "PROCEDURE DIVISION.", "P1.",
		"    ALTER P2 TO PROCEED TO P3.", "P2.", "    GO TO P4.", "P3.", "    EXIT.", "P4.", "    EXIT.")
	p := program(t, Analyze(map[string]string{"T.cbl": src}), "T")
	w := warnAt(p.Warnings, WAlter)
	if len(w) != 1 || w[0].Loc.Line != 5 {
		t.Fatalf("warnings = %+v", p.Warnings)
	}
	var found bool
	for _, e := range p.Edges {
		if e.From == "P2" && e.To == "P3" && e.Uncertain {
			found = true
		}
	}
	if !found {
		t.Errorf("no uncertain edge P2->P3: %+v", p.Edges)
	}
}

func TestExecSQL(t *testing.T) {
	src := fx("IDENTIFICATION DIVISION.", "PROGRAM-ID. T.", "DATA DIVISION.", "WORKING-STORAGE SECTION.",
		"01  HV PIC X.", "01  K2 PIC X.", "01  COL PIC X.", "PROCEDURE DIVISION.", "P1.",
		"    EXEC SQL SELECT COL INTO :HV", "        FROM TAB WHERE K = :K2 END-EXEC", "    MOVE HV TO K2.")
	p := program(t, Analyze(map[string]string{"T.cbl": src}), "T")
	if got := lines(p, "HV", "read", "T.cbl"); !reflect.DeepEqual(got, []int{10, 12}) {
		t.Errorf("HV reads = %v", got)
	}
	if got := lines(p, "K2", "read", "T.cbl"); !reflect.DeepEqual(got, []int{11}) {
		t.Errorf("K2 reads = %v", got)
	}
	if got := lines(p, "K2", "write", "T.cbl"); !reflect.DeepEqual(got, []int{12}) {
		t.Errorf("K2 writes = %v", got)
	}
	if got := lines(p, "COL", "read", "T.cbl"); len(got) != 0 {
		t.Errorf("SQL keyword/column became a ref: %v", got)
	}
}

func TestNotCobolAndFreeFormat(t *testing.T) {
	good := fx("IDENTIFICATION DIVISION.", "PROGRAM-ID. T.", "DATA DIVISION.", "WORKING-STORAGE SECTION.",
		"    COPY PLAIN.", "PROCEDURE DIVISION.", "P1.", "    STOP RUN.")
	proj := Analyze(map[string]string{
		"T.cbl":      good,
		"PLAIN":      fx("01  X PIC X."), // no DIVISION but COPYed: fine
		"README.txt": "hello world\n",
		"free.cbl":   ">>SOURCE FORMAT FREE\nIDENTIFICATION DIVISION.\n",
	})
	if len(proj.Programs) != 1 {
		t.Fatalf("programs = %+v", proj.Programs)
	}
	var got []string
	for _, w := range proj.Warnings {
		got = append(got, w.Kind+"@"+w.Loc.File)
	}
	sort.Strings(got)
	if want := []string{"free_format@free.cbl", "not_cobol@README.txt"}; !reflect.DeepEqual(got, want) {
		t.Errorf("warnings = %v", got)
	}
	if len(proj.Programs[0].Data) != 1 || proj.Programs[0].Data[0].Loc.File != "PLAIN" {
		t.Errorf("data = %+v", proj.Programs[0].Data)
	}
}

func TestCopyEdgeCases(t *testing.T) {
	main := fx("IDENTIFICATION DIVISION.", "PROGRAM-ID. T.", "DATA DIVISION.", "WORKING-STORAGE SECTION.",
		"    COPY A.", "    COPY 'B' OF LIB.", "    COPY C REPLACING ==X== BY ==Y==.", "PROCEDURE DIVISION.", "P1.", "    STOP RUN.")
	proj := Analyze(map[string]string{
		"T.cbl": main,
		"A.cpy": fx("01  FA PIC X.", "COPY B."),
		"B.cpy": fx("01  FB PIC X.", "COPY A."), // A -> B -> A
		"C.cpy": fx("01  FC PIC X."),
	})
	p := program(t, proj, "T")
	names := []string{}
	for _, d := range p.Data {
		names = append(names, d.Name)
	}
	if !reflect.DeepEqual(names, []string{"FA", "FB", "FB", "FA", "FC"}) {
		t.Errorf("data = %v", names)
	}
	var recursive, replacing int
	for _, w := range warnAt(p.Warnings, WUnsupported) {
		if strings.Contains(w.Detail, "recursive") {
			recursive++
		}
		if strings.Contains(w.Detail, "REPLACING") {
			replacing++
		}
	}
	if recursive != 2 || replacing != 1 {
		t.Errorf("warnings = %+v", p.Warnings)
	}
	// A copybook that copies itself must not loop either.
	Analyze(map[string]string{"T.cbl": main, "A.cpy": fx("COPY A."), "B.cpy": "", "C.cpy": ""})
}

func TestNoCrashOnJunk(t *testing.T) {
	for _, s := range []string{"", "\t", "abc", "      ", "       COPY", "       PROCEDURE DIVISION", "       IDENTIFICATION DIVISION.\n       PROGRAM-ID.", fx("PERFORM", "GO TO", "CALL", "ALTER")} {
		Analyze(map[string]string{"X.cbl": s})
		Analyze(map[string]string{"X.cbl": hdr + s})
	}
}

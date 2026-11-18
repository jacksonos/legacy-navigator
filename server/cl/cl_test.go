package cl

import (
	"fmt"
	"strings"
	"testing"
)

func edges(src string) (string, string) {
	p := parseFile("T.clp", src)
	var e, w []string
	for _, x := range p.Edges {
		s := fmt.Sprintf("%s>%s:%s@%d", x.From, x.To, x.Kind, x.Loc.Line)
		if x.Async {
			s += ":async"
		}
		if x.Dynamic {
			s += ":dyn"
		}
		e = append(e, s)
	}
	for _, x := range p.Warnings {
		w = append(w, x.Kind+":"+x.Detail)
	}
	return strings.Join(e, " "), strings.Join(w, " ")
}

func refs(src string) string {
	var out []string
	for _, x := range parseFile("T.clp", src).Refs {
		s := fmt.Sprintf("%s:%s@%d", x.Name, x.Kind, x.Loc.Line)
		if x.Paragraph != "" {
			s += "/" + x.Paragraph
		}
		if x.Detail != "" {
			s += "[" + x.Detail + "]"
		}
		out = append(out, s)
	}
	return strings.Join(out, " ")
}

func TestEdges(t *testing.T) {
	cases := []struct{ name, src, edges, warns string }{
		{"call keyword and lib", "PGM\nCALL PGM(BANCOLIB/BATCHDIA) PARM(&A)\nENDPGM", "T>BATCHDIA:call@2", ""},
		{"call positional", "PGM\ncall lib/cargamov\nENDPGM", "T>CARGAMOV:call@2", ""},
		{"callprc and tfrctl", "PGM\nCALLPRC PRC(MIPROC)\nTFRCTL PGM(LIB/OTRO)\nENDPGM", "T>MIPROC:call@2 T>OTRO:call@3", ""},
		{"dynamic call", "PGM\nCALL PGM(&PROG)\nENDPGM", "T>?:call@2:dyn", "dynamic_call:&PROG"},
		{"sbmjob nested cmd", "PGM\nSBMJOB CMD(CALL PGM(LIB/EXTRACTO) PARM('A')) JOB(EXTRDIA)\nENDPGM", "T>EXTRACTO:call@2:async", ""},
		{"sbmjob dynamic", "PGM\nSBMJOB CMD(CALL PGM(&P)) JOB(J)\nENDPGM", "T>?:call@2:async:dyn", "dynamic_call:&P"},
		{"if then call", "PGM\nIF COND(&A *EQ 1) THEN(CALL PGM(UNO))\nENDPGM", "T>UNO:call@2", ""},
		{"if bare cond", "PGM\nIF (&A *EQ 1) CALL DOS\nENDPGM", "T>DOS:call@2", ""},
		{"if then do else", "PGM\nIF COND(&A) THEN(DO)\nCALL UNO\nENDDO\nELSE CMD(CALL DOS)\nELSE GOTO FIN\nFIN: ENDPGM", "T>UNO:call@3 T>DOS:call@5 T>FIN:goto@6", ""},
		{"goto forms and monmsg", "PGM\nCALL X\nMONMSG MSGID(CPF0000) EXEC(GOTO ERROR)\nGOTO CMDLBL(FIN)\nERROR: GOTO (FIN)\nFIN: ENDPGM", "T>X:call@2 T>ERROR:goto@3 T>FIN:goto@4 ERROR>FIN:goto@5", ""},
		{"edges leave from label", "PGM\nA: CALL X\nB:\nCALL Y", "A>X:call@2 B>Y:call@4", ""},
		{"select when", "PGM\nSELECT\nWHEN COND(&A *EQ 1) THEN(CALL UNO)\nOTHERWISE CMD(CALL DOS)\nENDSELECT", "T>UNO:call@3 T>DOS:call@4", ""},
		{"continuation plus", "PGM\nCALL PGM(LIB/ABC+\n   DEF)\nENDPGM", "T>ABCDEF:call@2", ""},
		{"continuation minus", "PGM\nCALL PGM(AB -\n   CD)\nENDPGM", "T>AB    CD:call@2", ""},
		{"comments", "/* header\n  CALL NOPE */\nPGM /* x */\nCALL OK /* CALL NO */\n/* a */ CALL OK2\nENDPGM", "T>OK:call@4 T>OK2:call@5", ""},
		{"string with comment mark", "PGM\nSNDPGMMSG MSG('a /* b')\nCALL OK\nENDPGM", "T>OK:call@3", ""},
		{"case insensitive", "pgm\nsbmjob cmd(call pgm(lib/abc)) job(x)\nendpgm", "T>ABC:call@2:async", ""},
	}
	for _, c := range cases {
		e, w := edges(c.src)
		if e != c.edges || w != c.warns {
			t.Errorf("%s:\n got edges %q warns %q\nwant edges %q warns %q", c.name, e, w, c.edges, c.warns)
		}
	}
}

func TestRefs(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"ovrdbf", "PGM\nOVRDBF FILE(MOVIMIENTOS) TOFILE(BANCOLIB/MOVDIA) MBR(M1)\nOVRDBF FILE(CUENTAS) TOFILE(BANCOLIB/CTAMAE)\nDLTOVR FILE(*ALL)\nDLTOVR FILE(CUENTAS)",
			"MOVIMIENTOS:io@2[BANCOLIB/MOVDIA MBR(M1)] CUENTAS:io@3[BANCOLIB/CTAMAE] CUENTAS:io@5[DLTOVR]"},
		{"ovrdbf positional", "PGM\nOVRDBF F1 LIB/F2", "F1:io@2[LIB/F2]"},
		{"chgvar", "PGM\nCHGVAR VAR(&A) VALUE(&B *CAT 'x &C')", "&A:write@2 &B:read@2"},
		{"chgvar positional", "PGM\nCHGVAR &A (&B + 1)", "&A:write@2 &B:read@2"},
		{"rtv writes", "PGM\nRTVSYSVAL SYSVAL(QDATE) RTNVAL(&F)\nRTVJOBA JOB(&J) USER(&U)\nRTVDTAARA DTAARA(&D) RTNVAR(&V)", "&F:write@2 &J:write@3 &U:write@3 &D:read@4 &V:write@4"},
		{"dclf rcvf", "PGM\nDCLF FILE(LIB/PANTALLA)\nLOOP: RCVF\nSNDRCVF", "PANTALLA:io@2[DCLF] PANTALLA:io@3/LOOP[RCVF] PANTALLA:io@4/LOOP[SNDRCVF]"},
		{"other reads", "PGM\nSNDPGMMSG MSG('hola &X' *CAT &Y) TOPGMQ(*SAME)\nIF COND(&Z *EQ 1) THEN(CHGVAR &A 1)", "&Y:read@2 &Z:read@3 &A:write@3"},
		{"call parm", "PGM\nCALL X PARM(&A &B)", "&A:read@2 &B:read@2"},
		{"label paragraph", "PGM\nCHGVAR &A 1\nL1: CHGVAR &B 2", "&A:write@2 &B:write@3/L1"},
	}
	for _, c := range cases {
		if got := refs(c.src); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

func TestStructure(t *testing.T) {
	src := "PGM PARM(&P)\n  DCL VAR(&FECHA) TYPE(*CHAR) LEN(8)\n  DCL &N *DEC 5\n  DCLF FILE(L/F)\nINICIO: CALL X\nERROR:\n  SNDPGMMSG MSG('x')\nENDPGM\n"
	p := parseFile("sub/cierredia.CLP", src)
	if p.ID != "CIERREDIA" || p.Language != "cl" || p.File != "sub/cierredia.CLP" || p.IDLine != 1 || p.StartLine != 1 || p.EndLine != 8 {
		t.Fatalf("program %+v", p)
	}
	if len(p.Data) != 2 || p.Data[0].Name != "&FECHA" || p.Data[0].Pic != "*CHAR(8)" || p.Data[1].Name != "&N" || p.Data[1].Pic != "*DEC(5)" {
		t.Fatalf("data %+v", p.Data)
	}
	if len(p.Files) != 1 || p.Files[0].Name != "F" || p.Files[0].Assign != "L/F" {
		t.Fatalf("files %+v", p.Files)
	}
	if len(p.Paragraphs) != 2 || p.Paragraphs[0].Name != "INICIO" || p.Paragraphs[0].Loc.Line != 5 || p.Paragraphs[0].EndLine != 5 ||
		p.Paragraphs[1].Name != "ERROR" || p.Paragraphs[1].EndLine != 8 {
		t.Fatalf("labels %+v", p.Paragraphs)
	}
}

func TestDetectAndAnalyze(t *testing.T) {
	for name, want := range map[string]bool{"A.clp": true, "A.CLLE": true, "a.cl": true, "A.cbl": false, "A.txt": false} {
		if Detect(name, "plain") != want {
			t.Errorf("Detect(%s)", name)
		}
	}
	if !Detect("A.txt", "/* x */\n  PGM\n  CALL X\n  ENDPGM\n") || Detect("A.txt", "PGM only\n") {
		t.Error("Detect .txt")
	}
	proj := Analyze(map[string]string{"B.clp": "PGM\nCALL A\nENDPGM", "A.clp": "PGM\nENDPGM"})
	if len(proj.Programs) != 2 || proj.Programs[0].ID != "A" || proj.Programs[1].ID != "B" {
		t.Fatalf("%+v", proj.Programs)
	}
}

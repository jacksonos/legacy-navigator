package main

import (
	"github.com/jacksonos/legacy-navigator/server/cl"
	"github.com/jacksonos/legacy-navigator/server/cobol"
)

// analyze parses COBOL and CL files together. CALLs are resolved by program name
// across both languages: CL -> COBOL (and CL -> CL), COBOL -> CL.
func analyze(files map[string]string) *cobol.Project {
	clFiles, other := map[string]string{}, map[string]string{}
	for n, c := range files {
		if cl.Detect(n, c) {
			clFiles[n] = c
		} else {
			other[n] = c
		}
	}
	proj := cobol.Analyze(other)
	if len(clFiles) == 0 {
		return proj
	}
	cp := cl.Analyze(clFiles)
	isCL := map[string]bool{}
	for _, p := range cp.Programs {
		isCL[p.ID] = true
	}
	// cobol.Analyze only knew COBOL programs: a CALL to a CL program is not unresolved.
	ws := proj.Warnings[:0:0]
	for _, w := range proj.Warnings {
		if w.Kind != cobol.WUnresolvedCall || !isCL[w.Detail] {
			ws = append(ws, w)
		}
	}
	proj.Warnings = ws
	proj.Programs = append(proj.Programs, cp.Programs...)
	ids := map[string]bool{}
	for _, p := range proj.Programs {
		ids[p.ID] = true
	}
	for _, p := range cp.Programs {
		for _, e := range p.Edges {
			if e.Kind == "call" && !e.Dynamic && !ids[e.To] {
				proj.Warnings = append(proj.Warnings, cobol.Warning{Kind: cobol.WUnresolvedCall, Detail: e.To, Loc: e.Loc})
			}
		}
	}
	return proj
}

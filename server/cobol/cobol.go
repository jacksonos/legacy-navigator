// Package cobol parses fixed-format COBOL sources held in memory: it
// normalizes lines, expands COPY and indexes data, paragraphs, references
// and the call graph. It is regex/line-based and never does I/O.
package cobol

// Loc is a position in an original input file (Line is 1-based).
type Loc struct {
	File string
	Line int
}

// Warning kinds.
const (
	WMissingCopy    = "missing_copy"
	WDynamicCall    = "dynamic_call"
	WUnresolvedCall = "unresolved_call"
	WAlter          = "alter"
	WGotoDepending  = "goto_depending"
	WNotCobol       = "not_cobol"
	WFreeFormat     = "free_format"
	WUnsupported    = "unsupported" // COPY REPLACING, recursive COPY
)

type Warning struct {
	Kind   string
	Detail string
	Loc    Loc
}

type Project struct {
	Programs []Program
	Warnings []Warning
}

type Block struct {
	Name    string
	Section string // paragraphs only
	Loc     Loc
	EndLine int
}

type DataItem struct {
	Name   string
	Level  int
	Parent string
	Pic    string
	Loc    Loc
}

type FileDecl struct {
	Name   string
	Assign string
	Record string
	Loc    Loc
}

// Ref kinds: def | read | write | io.
type Ref struct {
	Name      string
	Kind      string
	Loc       Loc
	Paragraph string
}

// Edge kinds: perform | goto | call.
type Edge struct {
	From      string
	To        string
	Kind      string
	Loc       Loc
	Uncertain bool
	Dynamic   bool
}

type Program struct {
	ID         string
	File       string
	Sections   []Block
	Paragraphs []Block
	Data       []DataItem
	Files      []FileDecl
	Refs       []Ref
	Edges      []Edge
	Warnings   []Warning
}

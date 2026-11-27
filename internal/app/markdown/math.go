package markdown

import (
	"bytes"
	"strings"

	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var (
	kindMathBlock     = ast.NewNodeKind("IncipitMathBlock")
	kindInlineFormula = ast.NewNodeKind("IncipitInlineFormula")
	mathBlockInfoKey  = parser.NewContextKey()
)

type mathBlockNode struct {
	ast.BaseBlock
	dialect profile.MathDialect
	raw     string
}

func (n *mathBlockNode) Kind() ast.NodeKind { return kindMathBlock }
func (n *mathBlockNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

type formulaNode struct {
	ast.BaseInline
	dialect profile.MathDialect
	text    string
}

func (n *formulaNode) Kind() ast.NodeKind { return kindInlineFormula }
func (n *formulaNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

type mathFenceState struct {
	node       *mathBlockNode
	fenceLen   int
	indent     int
	singleLine bool
}

type mathBlockParser struct{}

var defaultMathBlockParser = &mathBlockParser{}

func (b *mathBlockParser) Trigger() []byte { return []byte{profile.MathSpelling[0]} }

func (b *mathBlockParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, _ := reader.PeekLine()
	pos := pc.BlockOffset()
	dollar := profile.MathSpelling[0]
	if pos < 0 || pos >= len(line) || line[pos] != dollar {
		return nil, parser.NoChildren
	}
	fenceLen := 0
	for i := pos; i < len(line) && line[i] == dollar; i++ {
		fenceLen++
	}
	if fenceLen < len(profile.MathSpelling) {
		return nil, parser.NoChildren
	}
	rest := bytes.TrimSpace(line[pos+fenceLen:])
	closing := bytes.Repeat([]byte{dollar}, fenceLen)
	if len(rest) >= fenceLen && bytes.HasSuffix(rest, closing) && !bytes.Contains(rest[:len(rest)-fenceLen], closing) {
		body := strings.TrimSpace(string(rest[:len(rest)-fenceLen]))
		node := &mathBlockNode{dialect: profile.DialectTeX, raw: body}
		pc.Set(mathBlockInfoKey, &mathFenceState{node: node, singleLine: true})
		reader.AdvanceToEOL()
		return node, parser.NoChildren
	}
	dialect := profile.DialectTeX
	info := strings.ToLower(string(rest))
	switch info {
	case "", "tex", "latex":
	case "typst":
		dialect = profile.DialectTypst
	default:
		return nil, parser.NoChildren
	}
	node := &mathBlockNode{dialect: dialect}
	pc.Set(mathBlockInfoKey, &mathFenceState{node: node, fenceLen: fenceLen, indent: pos})
	reader.AdvanceToEOL()
	return node, parser.NoChildren
}

func (b *mathBlockParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	st, _ := pc.Get(mathBlockInfoKey).(*mathFenceState)
	if st == nil || st.singleLine {
		return parser.Close
	}
	line, segment := reader.PeekLine()
	dollar := profile.MathSpelling[0]
	w, pos := util.IndentWidth(line, reader.LineOffset())
	if w < 4 {
		i := pos
		for ; i < len(line) && line[i] == dollar; i++ {
		}
		if i-pos >= st.fenceLen && util.IsBlank(line[i:]) {
			reader.AdvanceToEOL()
			return parser.Close
		}
	}
	pos, padding := util.IndentPositionPadding(line, reader.LineOffset(), segment.Padding, st.indent)
	if pos < 0 {
		pos = max(0, util.FirstNonSpacePosition(line)) - segment.Padding
		padding = 0
	}
	seg := text.NewSegmentPadding(segment.Start+pos, segment.Stop, padding)
	seg.ForceNewline = true
	node.Lines().Append(seg)
	reader.AdvanceToEOL()
	return parser.Continue | parser.NoChildren
}

func (b *mathBlockParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {
	if st, _ := pc.Get(mathBlockInfoKey).(*mathFenceState); st != nil && st.node == node {
		pc.Set(mathBlockInfoKey, nil)
	}
}

func (b *mathBlockParser) CanInterruptParagraph() bool { return true }
func (b *mathBlockParser) CanAcceptIndentedLine() bool { return false }

type inlineFormulaParser struct{}

var defaultInlineFormulaParser = &inlineFormulaParser{}

func (s *inlineFormulaParser) Trigger() []byte { return []byte{profile.FormulaSpelling[0]} }

func (s *inlineFormulaParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, _ := block.PeekLine()
	dollar := profile.FormulaSpelling[0]
	if block.PrecendingCharacter() == '\\' || len(line) < 2 || line[0] != dollar || line[1] == dollar {
		return nil
	}
	if isFormulaSpace(line[1]) {
		return nil
	}
	savedLine, savedPos := block.Position()
	block.Advance(1)
	var buf strings.Builder
	for {
		cur, _ := block.PeekLine()
		if cur == nil {
			block.SetPosition(savedLine, savedPos)
			return nil
		}
		for i := 0; i < len(cur); i++ {
			if cur[i] == '\\' && i+1 < len(cur) && cur[i+1] == dollar {
				buf.WriteByte('\\')
				buf.WriteByte(dollar)
				i++
				continue
			}
			if cur[i] == dollar {
				prevByte := byte(0)
				if i > 0 {
					prevByte = cur[i-1]
				} else if buf.Len() > 0 {
					s := buf.String()
					prevByte = s[len(s)-1]
				}
				nextDollar := i+1 < len(cur) && cur[i+1] == dollar
				nextDigit := i+1 < len(cur) && cur[i+1] >= '0' && cur[i+1] <= '9'
				if buf.Len()+i > 0 && !isFormulaSpace(prevByte) && !nextDollar && !nextDigit {
					buf.Write(cur[:i])
					src := singleLineText(buf.String())
					if src == "" {
						block.SetPosition(savedLine, savedPos)
						return nil
					}
					block.Advance(i + 1)
					return &formulaNode{dialect: profile.DialectTeX, text: src}
				}
			}
		}
		buf.Write(cur)
		block.AdvanceLine()
	}
}

func isFormulaSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

type mathExtension struct{}

var incipitMath = &mathExtension{}

func (e *mathExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithBlockParsers(util.Prioritized(defaultMathBlockParser, 700)),
		parser.WithInlineParsers(util.Prioritized(defaultInlineFormulaParser, 150)),
	)
}

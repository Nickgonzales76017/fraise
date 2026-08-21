// MIT License
package parser

import (
	"fmt"
	"strings"

	"github.com/RonsenbergVI/fraise/internal/query/lexer"
)

// SourceFieldNode is provenance metadata on a remembered fact. It is data, not
// fact identity. The field owns its quoted representation so spaces and
// punctuation cannot be re-tokenized when an AST is rendered back to FQL.
type SourceFieldNode struct {
	key   lexer.Token
	value string
}

func (n SourceFieldNode) String() string {
	return fmt.Sprintf("%s:'%s'", n.key.Literal, strings.ReplaceAll(n.value, "'", "''"))
}
func (n SourceFieldNode) Key() string         { return n.key.Literal }
func (n SourceFieldNode) Value() string       { return n.value }
func (n SourceFieldNode) Pos() lexer.Position { return n.key.Pos }
func (n SourceFieldNode) End() lexer.Position { return n.key.Pos }

// Source returns the one provenance reference carried by the remember command.
func (r RememberCommandNode[P]) Source() string {
	for _, a := range r.anchors {
		if f, ok := a.Field().(SourceFieldNode); ok {
			return f.Value()
		}
	}
	return ""
}

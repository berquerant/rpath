package rpath

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/berquerant/ybase"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

var (
	_ Queryer  = &YamlQuery{}
	_ Node     = &YAMLNode{}
	_ ItemNode = &YAMLItemNode{}
)

// YamlQuery finds yaml path of the element present at specified position.
type YamlQuery struct{}

func (q *YamlQuery) Query(r io.Reader, p *Position) (*Result, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	yBytes := ybase.Bytes(content)
	if err := p.Fill(yBytes); err != nil {
		return nil, err
	}
	fileNode, err := parser.ParseBytes(content, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	for _, doc := range fileNode.Docs {
		nodeMap := NewPathNodeMap()
		for _, node := range newYAMLNodes(doc, yBytes) {
			nodeMap.Add(node)
		}
		for _, node := range NewPathNodeComplementor(nodeMap).Complement() {
			nodeMap.Add(node)
		}
		if x, ok := nodeMap.Find(p.Offset); ok {
			meta := x.GetMeta(content)
			meta["char"] = fmt.Sprintf("%q", content[p.Offset])
			return &Result{
				Path:     x.Left.Path(),
				Position: p,
				Left:     x.Left.Meta(),
				Right:    x.Right.Meta(),
				Meta:     meta,
			}, nil
		}
	}

	return nil, ErrNotFound
}

func newYAMLNodes(root ast.Node, bytes ybase.Bytes) []Node {
	nodeC := make(chan ast.Node, 100)
	collector := &yamlNodeCollector{
		nodeC: nodeC,
	}
	go func() {
		defer close(nodeC)
		ast.Walk(collector, root)
	}()

	result := []Node{}
	for node := range nodeC {
		if x, ok := NewYAMLNode(node, bytes); ok {
			if y, ok := x.AsItemNode(); ok {
				result = append(result, y)
				continue
			}
			result = append(result, x)
		}
	}
	return result
}

type yamlNodeCollector struct {
	nodeC chan<- ast.Node
}

func (v *yamlNodeCollector) Visit(node ast.Node) ast.Visitor {
	v.nodeC <- node
	return v
}

type YAMLNode struct {
	node ast.Node
	pos  *Position
	path string
	typ  string
}

func (n YAMLNode) Pos() *Position { return n.pos }
func (n YAMLNode) Path() string   { return n.path }
func (n YAMLNode) Describe() string {
	return fmt.Sprintf("node[%s] %d:%d(%d) %s",
		n.Path(), n.pos.Line, n.pos.Column, n.Pos().Offset, n.typ,
	)
}
func (n YAMLNode) Clone() Node {
	return &YAMLNode{
		node: n.node,
		pos:  n.pos.Clone(),
		path: n.path,
		typ:  n.typ,
	}
}
func (n YAMLNode) Meta() any {
	return map[string]any{
		"line":   n.pos.Line,
		"column": n.pos.Column,
		"offset": n.pos.Offset,
		"type":   n.typ,
	}
}

var (
	yamlItemPathRegexp = regexp.MustCompile(`\[[0-9]+\]$`)
)

func (n YAMLNode) AsItemNode() (*YAMLItemNode, bool) {
	if !yamlItemPathRegexp.MatchString(n.path) {
		return nil, false
	}
	x := yamlItemPathRegexp.FindString(n.path)
	itemPath, _ := strings.CutSuffix(n.path, x)
	itemIndex, _ := strconv.Atoi(strings.Trim(x, "[]"))
	return &YAMLItemNode{
		YAMLNode:  n,
		itemPath:  itemPath,
		itemIndex: itemIndex,
	}, true
}

type YAMLItemNode struct {
	YAMLNode
	itemPath  string
	itemIndex int
}

func (n YAMLItemNode) ItemPath() string { return n.itemPath }
func (n YAMLItemNode) ItemIndex() int   { return n.itemIndex }
func (n YAMLItemNode) Describe() string {
	return fmt.Sprintf("node[%s] %d:%d(%d) %s item",
		n.Path(), n.pos.Line, n.pos.Column, n.Pos().Offset, n.typ,
	)
}

func NewYAMLNode(node ast.Node, bytes ybase.Bytes) (*YAMLNode, bool) {
	if node.GetToken() == nil {
		return nil, false
	}
	p := node.GetToken().Position
	if p == nil {
		return nil, false
	}
	offset := p.Offset
	if x, ok := bytes.Offset(p.Line, p.Column); ok {
		offset = x
	}
	return &YAMLNode{
		node: node,
		path: node.GetPath(),
		typ:  node.Type().String(),
		pos: &Position{
			Line:   p.Line,
			Column: p.Column,
			Offset: offset,
		},
	}, true
}

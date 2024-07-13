package rpath

import (
	"bytes"
	"fmt"
	"io"

	"github.com/berquerant/jsonast"
	"github.com/berquerant/ybase"
)

var (
	_ Queryer  = &JSONQuery{}
	_ Node     = &JSONNode{}
	_ ItemNode = &JSONItemNode{}
)

type JSONQuery struct{}

func (q *JSONQuery) Query(r io.Reader, p *Position) (*Result, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	yBytes := ybase.Bytes(content)
	if err := p.Fill(yBytes); err != nil {
		return nil, err
	}
	root, err := jsonast.Parse(bytes.NewBuffer(content))
	if err != nil {
		return nil, err
	}
	var (
		marshaler jsonast.Marshaler
		nodeMap   = NewPathNodeMap()
	)
	for _, node := range newJSONNodes(
		marshaler.Build(root.Value, jsonast.NewPath())) {
		nodeMap.Add(node)
	}
	// add last position as root (.) node.
	// first position node is already added as root node.
	// includes the entire range in the root,
	// so if it doesn't match any other range it will match the root.
	nodeMap.Add(&JSONNode{
		typ:  "Value",
		path: jsonast.NewPath(),
		pos:  NewLastPosition(yBytes),
	})
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
	return nil, ErrNotFound
}

func newJSONNodes(root jsonast.JNode) []Node {
	nodeC := make(chan jsonast.JNode, 100)
	go func() {
		defer close(nodeC)
		walkJSONNode(root, func(n jsonast.JNode) {
			nodeC <- n
		})
	}()

	result := []Node{}
	for node := range nodeC {
		x := NewJSONNode(node)
		if y, ok := x.AsItemNode(); ok {
			result = append(result, y)
			continue
		}
		result = append(result, x)
	}
	return result
}

var (
	nilJValue  *jsonast.JValue
	nilJPair   *jsonast.JPair
	nilJObject *jsonast.JObject
	nilJArray  *jsonast.JArray
)

func isNilJNode(node jsonast.JNode) bool {
	return node == nil || node == nilJValue || node == nilJPair || node == nilJObject || node == nilJArray
}

func walkJSONNode(node jsonast.JNode, f func(jsonast.JNode)) {
	if isNilJNode(node) {
		return
	}

	switch node := node.(type) {
	case *jsonast.JValue:
		f(node)
		walkJSONNode(node.Object, f)
		walkJSONNode(node.Array, f)
	case *jsonast.JPair:
		f(node)
		walkJSONNode(node.Value, f)
	case *jsonast.JObject:
		f(node)
		for _, p := range node.Pairs {
			walkJSONNode(p, f)
		}
	case *jsonast.JArray:
		f(node)
		for _, p := range node.Items {
			walkJSONNode(p, f)
		}
	default:
		panic("unknown json node")
	}
}

type JSONNode struct {
	node jsonast.JNode
	typ  string
	path jsonast.Path
	pos  *Position
}

func (n JSONNode) Pos() *Position { return n.pos }
func (n JSONNode) Path() string   { return n.path.AsPath() }
func (n JSONNode) Describe() string {
	return fmt.Sprintf("node[%s] %d:%d(%d) %s", n.Path(), n.pos.Line, n.pos.Column, n.pos.Offset, n.typ)
}
func (n JSONNode) Clone() Node {
	p := make([]jsonast.PathElement, len(n.path))
	copy(p, n.path)
	return &JSONNode{
		node: n.node,
		typ:  n.typ,
		path: p,
		pos:  n.pos.Clone(),
	}
}
func (n JSONNode) Meta() any {
	return map[string]any{
		"line":   n.pos.Line,
		"column": n.pos.Column,
		"offset": n.pos.Offset,
		"type":   n.typ,
	}
}

func (n JSONNode) AsItemNode() (*JSONItemNode, bool) {
	if len(n.path) == 0 {
		return nil, false
	}
	last := n.path[len(n.path)-1]
	index, ok := last.(*jsonast.IndexPath)
	if !ok || index.IndexString != "" {
		return nil, false
	}
	return &JSONItemNode{
		JSONNode:  n,
		itemPath:  n.path.WithoutLast().AsPath(),
		itemIndex: index.IndexInt,
	}, true
}

type JSONItemNode struct {
	JSONNode
	itemPath  string
	itemIndex int
}

func (n JSONItemNode) ItemPath() string { return n.itemPath }
func (n JSONItemNode) ItemIndex() int   { return n.itemIndex }

func NewJSONNode(node jsonast.JNode) *JSONNode {
	n := &JSONNode{
		node: node,
	}
	newPos := func(p *jsonast.JPos) *Position {
		return &Position{
			Line:   p.Line,
			Column: p.Column,
			Offset: p.Offset,
		}
	}
	switch x := n.node.(type) {
	case *jsonast.JValue:
		n.typ = "Value"
		n.path = x.Path
		n.pos = newPos(x.Pos)
	case *jsonast.JObject:
		n.typ = "Object"
		n.path = x.Path
		n.pos = newPos(x.Pos)
	case *jsonast.JPair:
		n.typ = "Pair"
		n.path = x.Path
		n.pos = newPos(x.Pos)
	case *jsonast.JArray:
		n.typ = "Array"
		n.path = x.Path
		n.pos = newPos(x.Pos)
	default:
		panic("unknown json node")
	}
	return n
}

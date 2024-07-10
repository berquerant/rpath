package rpath

import (
	"fmt"
	"io"
	"slices"

	"github.com/berquerant/ybase"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

var (
	_ Queryer = &YamlQuery{}
)

// YamlQuery finds yaml path of the element present at specified position.
type YamlQuery struct{}

func (q *YamlQuery) Query(r io.Reader, p *Position) (*Result, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if err := p.Fill(ybase.Bytes(content)); err != nil {
		return nil, err
	}
	fileNode, err := parser.ParseBytes(content, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	getNodeInfo := func(n ast.Node) any {
		return map[string]any{
			"line":   n.GetToken().Position.Line,
			"column": n.GetToken().Position.Column,
			"offset": n.GetToken().Position.Offset,
			"indent": n.GetToken().Position.IndentLevel,
			"type":   n.Type().String(),
		}
	}
	getMeta := func(left, right ast.Node) any {
		leftOffset := left.GetToken().Position.Offset
		rightOffset := right.GetToken().Position.Offset
		return map[string]any{
			"content": string(content[leftOffset-1 : rightOffset]),
			"char":    fmt.Sprintf("%q", content[p.Offset]),
		}
	}

	for _, doc := range fileNode.Docs {
		m := newYamlPathMap(doc)
		if r, ok := m.find(p.Offset); ok {
			return &Result{
				Path:     r.left.GetPath(),
				Position: p,
				Left:     getNodeInfo(r.left),
				Right:    getNodeInfo(r.right),
				Meta:     getMeta(r.left, r.right),
			}, nil
		}
	}
	return nil, ErrNotFound
}

func newYamlPathMap(root ast.Node) yamlPathMap {
	r := yamlPathMap(map[string]*yamlPathEntry{})
	ast.Walk(&yamlPathCollector{
		pathMap: r,
	}, root)
	return r
}

type (
	yamlPathEntry struct {
		nodes []ast.Node
	}
	// yaml path to nodes map
	yamlPathMap map[string]*yamlPathEntry
)

func (e yamlPathEntry) valid() bool {
	return len(e.nodes) == 2
}

func (e yamlPathEntry) in(offset int) bool {
	return inRange(
		offset+1,
		e.nodes[0].GetToken().Position.Offset,
		e.nodes[1].GetToken().Position.Offset,
	)
}

func (e yamlPathEntry) size() int {
	return e.nodes[1].GetToken().Position.Offset - e.nodes[0].GetToken().Position.Offset
}

type yamlNodePair struct {
	left  ast.Node
	right ast.Node
}

func (m yamlPathMap) find(offset int) (*yamlNodePair, bool) {
	var result *yamlPathEntry
	for _, e := range m {
		if !(e.valid() && e.in(offset)) {
			continue
		}
		if result == nil {
			result = e
			continue
		}
		if e.size() < result.size() {
			// more specific
			result = e
		}
	}

	if result != nil {
		return &yamlNodePair{
			left:  result.nodes[0],
			right: result.nodes[1],
		}, true
	}
	return nil, false
}

func (m yamlPathMap) add(node ast.Node) {
	if node.GetToken() == nil || node.GetToken().Position == nil {
		return
	}

	path := node.GetPath()
	e, ok := m[path]
	if !ok {
		m[path] = &yamlPathEntry{
			nodes: []ast.Node{node},
		}
		return
	}
	e.nodes = append(e.nodes, node)

	// sort entries
	switch len(e.nodes) {
	case 0, 1:
		return
	case 2:
		if e.nodes[0].GetToken().Position.Offset > e.nodes[1].GetToken().Position.Offset {
			e.nodes[0], e.nodes[1] = e.nodes[1], e.nodes[0]
		}
	default:
		slices.SortStableFunc(e.nodes, func(a, b ast.Node) int {
			left := a.GetToken().Position.Offset
			right := b.GetToken().Position.Offset
			return left - right
		})
		// keep the widest pair
		e.nodes = []ast.Node{
			e.nodes[0],
			e.nodes[len(e.nodes)-1],
		}
	}
}

type yamlPathCollector struct {
	pathMap yamlPathMap
}

func (v *yamlPathCollector) Visit(node ast.Node) ast.Visitor {
	v.pathMap.add(node)
	return v
}

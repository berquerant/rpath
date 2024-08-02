package rpath

import (
	"fmt"
	"io"
	"log"
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
	nodeMaps, err := q.prepare(r, p)
	if err != nil {
		return nil, err
	}

	for i := range nodeMaps.maxIndex {
		if nodeMap, ok := nodeMaps.get(i + 1); ok {
			firstNode := nodeMap.SortedNodes()[0]
			if p.Offset >= firstNode.Pos().Offset {
				OnDebug(func() {
					log.Printf("Ignore: Docs[%d], because offset %d is greater than docs[%d]'s first offset %d",
						i, p.Offset, i+1, firstNode.Pos().Offset,
					)
				})
				continue
			}
		}

		nodeMap, _ := nodeMaps.get(i)
		if x, ok := nodeMap.Find(p.Offset); ok {
			meta := x.GetMeta(nodeMaps.content)
			meta["char"] = fmt.Sprintf("%q", nodeMaps.content[p.Offset])
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

type yamlNodeMaps struct {
	get      func(int) (*PathNodeMap, bool)
	maxIndex int
	content  []byte
}

func (*YamlQuery) prepare(r io.Reader, p *Position) (*yamlNodeMaps, error) {
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

	nodeMaps := make(map[int]*PathNodeMap, len(fileNode.Docs))
	get := func(index int) (*PathNodeMap, bool) {
		if r, ok := nodeMaps[index]; ok {
			return r, true
		}
		if index < 0 || index >= len(fileNode.Docs) {
			return nil, false
		}

		nodeMap := NewPathNodeMap()
		doc := fileNode.Docs[index]
		for _, node := range newYAMLNodes(doc, yBytes) {
			nodeMap.Add(node)
		}
		for _, node := range NewPathNodeComplementor(nodeMap).Complement() {
			nodeMap.Add(node)
		}
		// add last node pair to tail of the document
		// because the last node does not cover the range between the node and tail of the document
		sortedNodes := nodeMap.SortedNodes()
		lastNode := sortedNodes[len(sortedNodes)-1].Clone()
		lastPosition := NewLastPosition(yBytes)
		lastNode.Pos().Line = lastPosition.Line
		lastNode.Pos().Column = lastPosition.Column
		lastNode.Pos().Offset = lastPosition.Offset
		nodeMap.Add(lastNode)

		nodeMaps[index] = nodeMap
		return nodeMap, true
	}

	return &yamlNodeMaps{
		get:      get,
		maxIndex: len(fileNode.Docs),
		content:  content,
	}, nil
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

package rpath

import (
	"fmt"
	"io"
	"log"
	"regexp"
	"slices"
	"strconv"
	"strings"

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
	yBytes := ybase.Bytes(content)
	if err := p.Fill(yBytes); err != nil {
		return nil, err
	}
	fileNode, err := parser.ParseBytes(content, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	getNodeInfo := func(n *yamlNode) any {
		return map[string]any{
			"line":   n.line,
			"column": n.column,
			"offset": n.offset,
			"indent": n.indent,
			"type":   n.typ,
		}
	}
	getMeta := func(left, right *yamlNode) any {
		return map[string]any{
			"content": string(content[left.offset:right.offset]),
			"char":    fmt.Sprintf("%q", content[p.Offset]),
		}
	}

	for _, doc := range fileNode.Docs {
		m := newYamlPathMap(doc, yBytes)
		if r, ok := m.find(p.Offset); ok {
			return &Result{
				Path:     r.left.path,
				Position: p,
				Left:     getNodeInfo(r.left),
				Right:    getNodeInfo(r.right),
				Meta:     getMeta(r.left, r.right),
			}, nil
		}
	}
	return nil, ErrNotFound
}

func newYamlPathMap(root ast.Node, yBytes ybase.Bytes) yamlPathMap {
	r := yamlPathMap{
		d: map[string]*yamlPathEntry{},
		b: yBytes,
	}
	c := &yamlPathCollector{
		pathMap: r,
		yBytes:  yBytes,
	}
	ast.Walk(c, root)
	r.fillElementPairs()
	return r
}

type (
	yamlNode struct {
		path   string
		line   int
		column int
		offset int
		indent int
		typ    string
		// token value
		value          string
		hasPosition    bool
		isArrayElement bool
		// cut suffix [index] from path
		arrayPath  string
		arrayIndex int
	}

	yamlPathEntry struct {
		nodes []*yamlNode
	}
	// yaml path to nodes map
	yamlPathMap struct {
		d map[string]*yamlPathEntry
		b ybase.Bytes
	}
)

func (n *yamlNode) clone() *yamlNode {
	return &yamlNode{
		path:           n.path,
		line:           n.line,
		column:         n.column,
		offset:         n.offset,
		indent:         n.indent,
		typ:            n.typ,
		hasPosition:    n.hasPosition,
		isArrayElement: n.isArrayElement,
		arrayPath:      n.arrayPath,
		arrayIndex:     n.arrayIndex,
		value:          n.value,
	}
}

var (
	yamlArrayElementPathRegexp = regexp.MustCompile(`\[[0-9]+\]$`)
)

func newYamlNode(node ast.Node, yBytes ybase.Bytes) *yamlNode {
	n := &yamlNode{
		path:           node.GetPath(),
		typ:            node.Type().String(),
		isArrayElement: yamlArrayElementPathRegexp.MatchString(node.GetPath()),
	}
	if n.isArrayElement {
		x := yamlArrayElementPathRegexp.FindString(n.path)
		n.arrayPath, _ = strings.CutSuffix(n.path, x)
		i := strings.Trim(x, "[]")
		n.arrayIndex, _ = strconv.Atoi(i)
	}
	if node.GetToken() == nil {
		return n
	}
	n.value = node.GetToken().Value
	if p := node.GetToken().Position; p != nil {
		n.hasPosition = true
		n.line = p.Line
		n.column = p.Column
		n.indent = p.IndentLevel
		n.offset = p.Offset
		if offset, ok := yBytes.Offset(n.line, n.column); ok {
			// Position.Offset is not considering multibyte chars?
			n.offset = offset
		}
	}
	return n
}

func (e yamlPathEntry) valid() bool {
	return len(e.nodes) == 2
}

func (e yamlPathEntry) in(offset int) bool {
	if offset == 0 {
		// FIXME: first node offset is 1?
		offset = 1
	}
	r := inRange(
		offset,
		e.nodes[0].offset,
		e.nodes[1].offset,
	)
	OnDebug(func() {
		log.Printf("Yaml: in(%d) %v [%s] [%s]",
			offset,
			r,
			describeYamlNode(e.nodes[0]),
			describeYamlNode(e.nodes[1]),
		)
	})
	return r
}

func (e yamlPathEntry) size() int {
	return e.nodes[1].offset - e.nodes[0].offset
}

type yamlNodePair struct {
	left  *yamlNode
	right *yamlNode
}

func (m *yamlPathMap) find(offset int) (*yamlNodePair, bool) {
	OnDebug(func() {
		log.Printf("Yaml: find offset %d", offset)
	})

	var (
		result    *yamlPathEntry
		setResult = func(r *yamlPathEntry) {
			result = r
			OnDebug(func() {
				log.Printf("Yaml: found node pair: [%s] [%s]",
					describeYamlNode(r.nodes[0]),
					describeYamlNode(r.nodes[1]),
				)
			})
		}
	)

	for _, e := range m.d {
		if !(e.valid() && e.in(offset)) {
			continue
		}
		if result == nil {
			setResult(e)
			continue
		}
		if e.size() < result.size() {
			// more specific
			setResult(e)
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

// There are not enough pairs of nodes in the offset range, so complement them.
//
// - Add pair of not array element and closest next node
// - Add pair of first array element and closest next node
// - Add pair of first array element and closest previous node
// - Add pair of last array element and closest next node
// - Add pair of not first array element and closest next array element
func (m *yamlPathMap) fillElementPairs() {
	yamlNodes := []*yamlNode{}
	for _, n := range m.d {
		yamlNodes = append(yamlNodes, n.nodes...)
	}
	slices.SortStableFunc(yamlNodes, func(a, b *yamlNode) int {
		return a.offset - b.offset
	})

	arrayElementNodes := []*yamlNode{}
	for _, n := range yamlNodes {
		if n.isArrayElement {
			arrayElementNodes = append(arrayElementNodes, n)
		}
	}

	arrayElementMaxIndexMap := map[string]int{}
	arrayElementMap := map[string]map[int]*yamlNode{}
	for _, n := range arrayElementNodes {
		if _, ok := arrayElementMap[n.arrayPath]; !ok {
			arrayElementMap[n.arrayPath] = map[int]*yamlNode{}
		}
		arrayElementMap[n.arrayPath][n.arrayIndex] = n

		index := arrayElementMaxIndexMap[n.arrayPath]
		if index < n.arrayIndex {
			arrayElementMaxIndexMap[n.arrayPath] = n.arrayIndex
		}
	}

	var (
		findClosestPreviousNode = func(offset int) (*yamlNode, bool) {
			i, ok := FindClosestFloor(yamlNodes, offset, func(n *yamlNode, x int) int {
				return n.offset - x
			})
			if ok {
				return yamlNodes[i], true
			}
			return yamlNodes[0], false
		}
		findClosestNextNode = func(offset int) (*yamlNode, bool) {
			i, ok := FindClosestCeiling(yamlNodes, offset, func(n *yamlNode, x int) int {
				return n.offset - x
			})
			if ok {
				return yamlNodes[i], true
			}
			return yamlNodes[len(yamlNodes)-1], false
		}

		findClosestNodeExceptSamePath = func(node *yamlNode, findClosest func(int) (*yamlNode, bool)) *yamlNode {
			offset := node.offset
			for {
				x, ok := findClosest(offset)
				if !ok {
					return x
				}
				if x.path != node.path {
					return x
				}
				offset = x.offset
			}
		}

		getArrayElement = func(node *yamlNode, indexDelta int) (*yamlNode, bool) {
			m, ok := arrayElementMap[node.arrayPath]
			if !ok {
				return nil, false
			}
			if n, ok := m[node.arrayIndex+indexDelta]; ok {
				return n, true
			}
			return nil, false
		}
	)

	type pair struct {
		origin *yamlNode
		found  *yamlNode
	}
	type result struct {
		pairs []*pair
	}
	var (
		getElementPair = func(node *yamlNode) *result {
			var r result

			addResult := func(n *yamlNode, offsetDelta int) {
				x := node.clone()
				x.line = n.line
				x.column = n.column
				x.offset = n.offset + offsetDelta
				r.pairs = append(r.pairs, &pair{
					origin: n,
					found:  x,
				})
			}

			if !node.isArrayElement {
				addResult(findClosestNodeExceptSamePath(node, findClosestNextNode), -1)
				return &r
			}

			// node.offset is at the beginning of the value, at `v`
			// - value
			// so range (node, next_node) is [ to ]:
			// - [node
			// - ]next_node
			switch node.arrayIndex {
			case 0:
				addResult(findClosestNodeExceptSamePath(node, findClosestNextNode), -1)
				if arrayElementMaxIndexMap[node.arrayPath] == 0 {
					addResult(findClosestNodeExceptSamePath(node, findClosestPreviousNode), 1)
				}
			case arrayElementMaxIndexMap[node.arrayPath]:
				addResult(findClosestNodeExceptSamePath(node, findClosestNextNode), -1)
			default:
				if x, ok := getArrayElement(node, 1); ok {
					addResult(x, -1)
				}
			}

			return &r
		}
	)

	for _, n := range yamlNodes {
		r := getElementPair(n)
		for _, x := range r.pairs {
			OnDebug(func() {
				log.Printf("Yaml: fillNodes: %s -> %s | %s",
					describeYamlNode(n),
					describeYamlNode(x.found),
					describeYamlNode(x.origin))
			})
			m.add(x.found)
		}
	}
}

func (m *yamlPathMap) add(node *yamlNode) {
	if !node.hasPosition {
		return
	}

	OnDebug(func() {
		log.Printf("Yaml: %s", describeYamlNode(node))
	})

	e, ok := m.d[node.path]
	if !ok {
		m.d[node.path] = &yamlPathEntry{
			nodes: []*yamlNode{node},
		}
		return
	}
	e.nodes = append(e.nodes, node)

	// sort entries
	switch len(e.nodes) {
	case 0, 1:
		return
	case 2:
		if e.nodes[0].offset > e.nodes[1].offset {
			e.nodes[0], e.nodes[1] = e.nodes[1], e.nodes[0]
		}
	default:
		slices.SortStableFunc(e.nodes, func(a, b *yamlNode) int {
			return a.offset - b.offset
		})
		// keep the widest pair
		e.nodes = []*yamlNode{
			e.nodes[0],
			e.nodes[len(e.nodes)-1],
		}
	}
}

type yamlPathCollector struct {
	pathMap yamlPathMap
	yBytes  ybase.Bytes
}

func (v *yamlPathCollector) Visit(node ast.Node) ast.Visitor {
	yNode := newYamlNode(node, v.yBytes)
	v.pathMap.add(yNode)
	return v
}

func describeYamlNode(node *yamlNode) string {
	return fmt.Sprintf("node[%s] %d:%d(%d) %s `%s`",
		node.path,
		node.line,
		node.column,
		node.offset,
		node.typ,
		node.value,
	)
}

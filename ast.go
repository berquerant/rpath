package rpath

import (
	"log"
	"slices"
)

type Node interface {
	// Pos is the starting position of the node.
	Pos() *Position
	Path() string
	Describe() string
	Meta() any
	Clone() Node
}

type ItemNode interface {
	Node
	// Path except index.
	ItemPath() string
	ItemIndex() int
}

type PathNodeMapEntry struct {
	Nodes []Node
}

func (e PathNodeMapEntry) IsValid() bool {
	return len(e.Nodes) == 2
}

func (e PathNodeMapEntry) Size() int {
	left, right := e.Nodes[0], e.Nodes[1]
	return right.Pos().Offset - left.Pos().Offset
}

func (e PathNodeMapEntry) In(offset int) bool {
	left, right := e.Nodes[0], e.Nodes[1]
	if offset == 0 {
		// FIXME: first node offset is 1?
		offset = 1
	}
	r := inRange(offset, left.Pos().Offset, right.Pos().Offset)
	OnDebug(func() {
		log.Printf("In(%d): %v [%s] [%s]",
			offset, r, left.Describe(), right.Describe(),
		)
	})
	return r
}

// Path to Node list map.
type PathNodeMap struct {
	dict map[string]*PathNodeMapEntry
}

func NewPathNodeMap() *PathNodeMap {
	return &PathNodeMap{
		dict: map[string]*PathNodeMapEntry{},
	}
}

type PathNodePair struct {
	Left  Node
	Right Node
}

func (p PathNodePair) GetMeta(content []byte) map[string]any {
	return map[string]any{
		"content": string(content[p.Left.Pos().Offset:p.Right.Pos().Offset]),
	}
}

// Find finds node containing offset.
//
// If multiple nodes are found, returns the one with the smaller range.
func (m *PathNodeMap) Find(offset int) (*PathNodePair, bool) {
	OnDebug(func() {
		log.Printf("Find: offset %d", offset)
	})

	var (
		result    *PathNodeMapEntry
		setResult = func(r *PathNodeMapEntry) {
			result = r
			OnDebug(func() {
				log.Printf("Find: found pair: [%s] [%s]",
					result.Nodes[0].Describe(),
					result.Nodes[1].Describe(),
				)
			})
		}
	)

	for _, entry := range m.dict {
		if !(entry.IsValid() && entry.In(offset)) {
			continue
		}
		if result == nil || entry.Size() < result.Size() {
			setResult(entry)
		}
	}

	if result != nil {
		return &PathNodePair{
			Left:  result.Nodes[0],
			Right: result.Nodes[1],
		}, true
	}

	return nil, false
}

func (m *PathNodeMap) Add(node Node) {
	OnDebug(func() {
		log.Printf("Add: %s", node.Describe())
	})

	entry, ok := m.dict[node.Path()]
	if !ok {
		m.dict[node.Path()] = &PathNodeMapEntry{
			Nodes: []Node{node},
		}
		return
	}
	entry.Nodes = append(entry.Nodes, node)

	// sort nodes
	switch len(entry.Nodes) {
	case 0, 1:
		return
	case 2:
		if entry.Nodes[0].Pos().Offset > entry.Nodes[1].Pos().Offset {
			entry.Nodes[0], entry.Nodes[1] = entry.Nodes[1], entry.Nodes[0]
		}
	default:
		slices.SortStableFunc(entry.Nodes, func(a, b Node) int {
			return a.Pos().Offset - b.Pos().Offset
		})
		// keep the widest pair
		entry.Nodes = []Node{
			entry.Nodes[0],
			entry.Nodes[len(entry.Nodes)-1],
		}
	}
}

type PathNodeComplementor struct {
	nodeMap         *PathNodeMap
	nodes           []Node
	itemNodes       []ItemNode
	itemNodeMap     map[string]map[int]ItemNode
	itemMaxIndexMap map[string]int
}

func (c PathNodeComplementor) getMaxIndex(item ItemNode) int {
	return c.itemMaxIndexMap[item.ItemPath()]
}

// Complement generates missing nodes to find nodes by offset.
func (c PathNodeComplementor) Complement() []Node {
	result := []Node{}
	for _, n := range c.nodes {
		r := c.getItemPair(n)
		OnDebug(func() {
			log.Printf("Complement: %s count %d", n.Describe(), len(r))
		})
		for _, x := range r {
			OnDebug(func() {
				log.Printf("Complement: %s -> %s | %s",
					n.Describe(),
					x.found.Describe(),
					x.origin.Describe(),
				)
			})

			result = append(result, x.found)
		}
	}
	return result
}

type pathNodeComplementorNodePair struct {
	origin Node
	found  Node
}

func (c PathNodeComplementor) getItemPair(node Node) (result []*pathNodeComplementorNodePair) {
	addResult := func(n Node, offsetDelta int) {
		x := node.Clone()
		x.Pos().Line = n.Pos().Line
		x.Pos().Column = n.Pos().Column
		x.Pos().Offset = n.Pos().Offset + offsetDelta
		result = append(result, &pathNodeComplementorNodePair{
			origin: n,
			found:  x,
		})
	}

	item, ok := node.(ItemNode)
	if !ok {
		OnDebug(func() {
			log.Printf("Complement: node[%s] getItemPair `not Item` %s", node.Path(), node.Describe())
		})
		// add pair of not array element and closest next node
		addResult(c.findClosestNodeExceptSamePath(node, c.findClosestNextNode), -1)
		return
	}

	// node.offset is at the beginning of the value, at `v`
	// - value
	// so range (node, next_node) is [ to ]:
	// - [node
	// - ]next_node
	maxIndex := c.getMaxIndex(item)
	switch item.ItemIndex() {
	case 0:
		OnDebug(func() {
			log.Printf("Complement: node[%s] getItemPair `Item 0` Item[%d/%d] %s",
				item.ItemPath(), item.ItemIndex(), maxIndex, node.Describe())
		})
		// add pair of first array element and closest next node
		addResult(c.findClosestNodeExceptSamePath(node, c.findClosestNextNode), -1)
		// if maxIndex == 0 {
		// 	// add pair of first array element and closest previous node
		// 	addResult(c.findClosestNodeExceptSamePath(node, c.findClosestPreviousNode), 1)
		// }
	case maxIndex:
		OnDebug(func() {
			log.Printf("Complement: node[%s] getItemPair `Item TAIL` Item[%d/%d] %s",
				item.ItemPath(), item.ItemIndex(), maxIndex, node.Describe())
		})
		// add pair of last array element and closest next node
		addResult(c.findClosestNodeExceptSamePath(node, c.findClosestNextNode), -1)
	default:
		OnDebug(func() {
			log.Printf("Complement: node[%s] getItemPair `Item MID` Item[%d/%d] %s",
				item.ItemPath(), item.ItemIndex(), maxIndex, node.Describe())
		})
		// add pair of not first nor last array element and closest next array element
		if x, ok := c.getItemNode(item, 1); ok {
			addResult(x, -1)
		}
	}

	return
}

func (c PathNodeComplementor) findClosestPreviousNode(offset int) (Node, bool) {
	i, ok := FindClosestFloor(c.nodes, offset, func(n Node, x int) int {
		return n.Pos().Offset - x
	})
	if ok {
		return c.nodes[i], true
	}
	return c.nodes[0], false
}

func (c PathNodeComplementor) findClosestNextNode(offset int) (Node, bool) {
	i, ok := FindClosestCeiling(c.nodes, offset, func(n Node, x int) int {
		return n.Pos().Offset - x
	})
	if ok {
		return c.nodes[i], true
	}
	return c.nodes[len(c.nodes)-1], false
}

func (c PathNodeComplementor) findClosestNodeExceptSamePath(
	node Node,
	findClosest func(int) (Node, bool),
) Node {
	offset := node.Pos().Offset
	for {
		x, ok := findClosest(offset)
		if !ok {
			return x
		}
		if x.Path() != node.Path() {
			return x
		}
		offset = x.Pos().Offset
	}
}

func (c PathNodeComplementor) getItemNode(node ItemNode, indexDelta int) (ItemNode, bool) {
	m, ok := c.itemNodeMap[node.Path()]
	if !ok {
		return nil, false
	}
	if n, ok := m[node.ItemIndex()+indexDelta]; ok {
		return n, true
	}
	return nil, false
}

func NewPathNodeComplementor(nodeMap *PathNodeMap) *PathNodeComplementor {
	nodes := []Node{}
	for _, entry := range nodeMap.dict {
		nodes = append(nodes, entry.Nodes...)
	}
	slices.SortStableFunc(nodes, func(a, b Node) int {
		return a.Pos().Offset - b.Pos().Offset
	})

	itemNodes := []ItemNode{}
	for _, n := range nodes {
		if m, ok := n.(ItemNode); ok {
			itemNodes = append(itemNodes, m)
		}
	}

	var (
		itemNodeMap     = map[string]map[int]ItemNode{}
		itemMaxIndexMap = map[string]int{}
	)
	for _, n := range itemNodes {
		if _, ok := itemNodeMap[n.ItemPath()]; !ok {
			itemNodeMap[n.ItemPath()] = map[int]ItemNode{}
		}
		itemNodeMap[n.ItemPath()][n.ItemIndex()] = n
		index := itemMaxIndexMap[n.ItemPath()]
		if index < n.ItemIndex() {
			itemMaxIndexMap[n.ItemPath()] = n.ItemIndex()
		}
	}

	return &PathNodeComplementor{
		nodeMap:         nodeMap,
		nodes:           nodes,
		itemNodes:       itemNodes,
		itemNodeMap:     itemNodeMap,
		itemMaxIndexMap: itemMaxIndexMap,
	}
}

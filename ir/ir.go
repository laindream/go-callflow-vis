package ir

import (
	"fmt"
	"github.com/laindream/go-callflow-vis/render"
	"github.com/laindream/go-callflow-vis/util"
	"golang.org/x/tools/go/callgraph"
	"math"
	"strings"
	"unsafe"
)

type Callgraph struct {
	Root  *Node
	Nodes map[string]*Node
}

func (c *Callgraph) AddEdge(callerFn, calleeFn string, site *Site) {
	caller := c.Nodes[callerFn]
	callee := c.Nodes[calleeFn]
	if caller == nil || callee == nil {
		return
	}
	edge := &Edge{
		Caller: caller,
		Site:   site,
		Callee: callee,
	}
	callee.AddIn(edge)
	caller.AddOut(edge)
}

func (c *Callgraph) ResetSearched() {
	c.Root.ResetSearched()
	for _, v := range c.Nodes {
		v.ResetSearched()
	}
	return
}

func (c *Callgraph) DeleteNode(n *Node) *Callgraph {
	inOutNumMultiplier := len(n.In) * len(n.Out)
	inOutNumAdder := len(n.In) + len(n.Out)
	nodeRate := len(c.Nodes) * int(math.Sqrt(float64(len(c.Nodes))))
	if inOutNumMultiplier > nodeRate ||
		inOutNumAdder*inOutNumAdder > nodeRate {
		return GetFilteredCallgraph(c, func(s string) bool {
			return s != n.Func.Name
		})
	}
	n.DeleteIns()
	n.DeleteOuts()
	if n.Func != nil {
		delete(c.Nodes, n.Func.Addr)
	}
	return c
}

func ConvertToIR(graph *callgraph.Graph) *Callgraph {
	getPointerStr := func(p unsafe.Pointer) string {
		return fmt.Sprintf("%p", p)
	}
	var root *Node
	if graph.Root != nil {
		var name string
		var fp *Func
		var fSig string
		if graph.Root.Func != nil {
			name = graph.Root.Func.String()
			if graph.Root.Func.Signature != nil {
				fSig = graph.Root.Func.Signature.String()
			}
			if graph.Root.Func.Parent() != nil {
				fpSig := ""
				if graph.Root.Func.Parent().Signature != nil {
					fpSig = graph.Root.Func.Parent().Signature.String()
				}
				fp = &Func{
					Name:      graph.Root.Func.Parent().String(),
					Addr:      getPointerStr(unsafe.Pointer(graph.Root.Func.Parent())),
					Signature: fpSig,
				}
			}
		}
		root = &Node{
			Func: &Func{
				Name: name,
				Addr: getPointerStr(unsafe.Pointer(graph.Root.Func)),
				//Addr:      name,
				Parent:    fp,
				Signature: fSig,
			},
			ID: graph.Root.ID,
		}
	}
	nodes := make(map[string]*Node)
	for f, node := range graph.Nodes {
		if f == nil {
			continue
		}
		if node.Func == nil {
			continue
		}
		var fp *Func
		var fSig string
		if node.Func.Signature != nil {
			fSig = node.Func.Signature.String()
		}
		if node.Func.Parent() != nil {
			fpSig := ""
			if node.Func.Parent().Signature != nil {
				fpSig = node.Func.Parent().Signature.String()
			}
			fp = &Func{
				Name:      node.Func.Parent().String(),
				Addr:      getPointerStr(unsafe.Pointer(node.Func.Parent())),
				Signature: fpSig,
			}
		}
		funcIR := &Func{
			Name:      node.Func.String(),
			Addr:      getPointerStr(unsafe.Pointer(node.Func)),
			Parent:    fp,
			Signature: fSig,
		}
		nodes[funcIR.Addr] = &Node{
			Func: funcIR,
			ID:   node.ID,
		}
	}
	for attr, node := range graph.Nodes {
		if attr == nil {
			continue
		}
		if node.Func == nil {
			continue
		}
		nodeIR := nodes[getPointerStr(unsafe.Pointer(attr))]
		for _, edge := range node.Out {
			caller := nodes[getPointerStr(unsafe.Pointer(edge.Caller.Func))]
			callee := nodes[getPointerStr(unsafe.Pointer(edge.Callee.Func))]
			var site *Site
			if edge.Site != nil {
				site = &Site{
					Name: edge.Site.String(),
					Addr: getPointerStr(unsafe.Pointer(&edge.Site)),
				}
			}
			edgeIR := &Edge{
				Caller: caller,
				Site:   site,
				Callee: callee,
			}
			nodeIR.Out = append(caller.Out, edgeIR)
		}
		for _, edge := range node.In {
			caller := nodes[getPointerStr(unsafe.Pointer(edge.Caller.Func))]
			callee := nodes[getPointerStr(unsafe.Pointer(edge.Callee.Func))]
			var site *Site
			if edge.Site != nil {
				site = &Site{
					Name: edge.Site.String(),
					Addr: getPointerStr(unsafe.Pointer(&edge.Site)),
				}
			}
			edgeIR := &Edge{
				Caller: caller,
				Site:   site,
				Callee: callee,
			}
			nodeIR.In = append(callee.In, edgeIR)
		}
	}
	return &Callgraph{
		Root:  root,
		Nodes: nodes,
	}
}

func GetFilteredCallgraph(graph *Callgraph, filter func(string) bool) *Callgraph {
	var root *Node
	if graph.Root != nil {
		var name string
		var fp *Func
		var fSig string
		if graph.Root.Func != nil {
			name = graph.Root.Func.Name
			if graph.Root.Func.Signature != "" {
				fSig = graph.Root.Func.Signature
			}
			if graph.Root.Func.Parent != nil {
				fpSig := ""
				if graph.Root.Func.Parent.Signature != "" {
					fpSig = graph.Root.Func.Parent.Signature
				}
				fp = &Func{
					Name:      graph.Root.Func.Parent.Name,
					Addr:      graph.Root.Func.Parent.Addr,
					Signature: fpSig,
				}
			}
		}
		root = &Node{
			Func: &Func{
				Name: name,
				Addr: graph.Root.Func.Addr,
				//Addr:      name,
				Parent:    fp,
				Signature: fSig,
			},
			ID: graph.Root.ID,
		}
	}
	nodes := make(map[string]*Node)
	for f, node := range graph.Nodes {
		if f == "" {
			continue
		}
		if node.Func == nil {
			continue
		}
		if !filter(node.Func.Name) {
			continue
		}
		var fp *Func
		var fSig string
		if node.Func.Signature != "" {
			fSig = node.Func.Signature
		}
		if node.Func.Parent != nil {
			fpSig := ""
			if node.Func.Parent.Signature != "" {
				fpSig = node.Func.Parent.Signature
			}
			fp = &Func{
				Name:      node.Func.Parent.Name,
				Addr:      node.Func.Parent.Addr,
				Signature: fpSig,
			}
		}
		funcIR := &Func{
			Name:      node.Func.Name,
			Addr:      node.Func.Addr,
			Parent:    fp,
			Signature: fSig,
		}
		nodes[funcIR.Addr] = &Node{
			Func: funcIR,
			ID:   node.ID,
		}
	}
	for attr, node := range graph.Nodes {
		if attr == "" {
			continue
		}
		if node.Func == nil {
			continue
		}
		if !filter(node.Func.Name) {
			continue
		}
		nodeIR := nodes[attr]
		for _, edge := range node.Out {
			if !filter(edge.Callee.Func.Name) {
				continue
			}
			caller := nodes[edge.Caller.Func.Addr]
			callee := nodes[edge.Callee.Func.Addr]
			var site *Site
			if edge.Site != nil {
				site = &Site{
					Name: edge.Site.Name,
					Addr: edge.Site.Addr,
				}
			}
			edgeIR := &Edge{
				Caller: caller,
				Site:   site,
				Callee: callee,
			}
			nodeIR.Out = append(caller.Out, edgeIR)
		}
		for _, edge := range node.In {
			if !filter(edge.Caller.Func.Name) {
				continue
			}
			caller := nodes[edge.Caller.Func.Addr]
			callee := nodes[edge.Callee.Func.Addr]
			var site *Site
			if edge.Site != nil {
				site = &Site{
					Name: edge.Site.Name,
					Addr: edge.Site.Addr,
				}
			}
			edgeIR := &Edge{
				Caller: caller,
				Site:   site,
				Callee: callee,
			}
			nodeIR.In = append(callee.In, edgeIR)
		}
	}
	return &Callgraph{
		Root:  root,
		Nodes: nodes,
	}
}

type Func struct {
	Name      string
	Addr      string
	Parent    *Func
	Signature string
}

type Site struct {
	Name string
	Addr string
}

type Node struct {
	Func                *Func
	ID                  int
	In                  []*Edge
	Out                 []*Edge
	humanReadableInMap  map[string]*Edge
	humanReadableOutMap map[string]*Edge
	inMap               map[string]*Edge
	outMap              map[string]*Edge
	searched            bool
	path                []*Edge
	tags                map[string]bool
}

func (n *Node) ToRenderNode() *render.Node {
	return &render.Node{
		ID:     n.ID,
		Set:    -1,
		Name:   util.GetFuncSimpleName(n.Func.Name),
		Detail: n.Func.Name,
	}
}

func (n *Node) ResetTags() {
	n.tags = nil
}

func (n *Node) AddTag(tag string) {
	if n.tags == nil {
		n.tags = make(map[string]bool)
	}
	n.tags[tag] = true
}

func (n *Node) HasTag(tag string) bool {
	if n.tags == nil {
		return false
	}
	return n.tags[tag]
}

func (n *Node) UpdateHumanReadableMap() {
	n.humanReadableInMap = make(map[string]*Edge)
	for i, _ := range n.In {
		n.humanReadableInMap[n.In[i].ReadableString()] = n.In[i]
	}
	n.humanReadableOutMap = make(map[string]*Edge)
	for i, _ := range n.Out {
		n.humanReadableOutMap[n.Out[i].ReadableString()] = n.Out[i]
	}
}

func (n *Node) UpdateInOutMap() {
	n.inMap = make(map[string]*Edge)
	for i, _ := range n.In {
		n.inMap[n.In[i].String()] = n.In[i]
	}
	n.outMap = make(map[string]*Edge)
	for i, _ := range n.Out {
		n.outMap[n.Out[i].String()] = n.Out[i]
	}
}

func (n *Node) AddIn(e *Edge) {
	if len(n.inMap) == 0 {
		n.UpdateInOutMap()
	}
	if _, ok := n.inMap[e.String()]; ok {
		return
	}
	n.inMap[e.String()] = e
	n.In = append(n.In, e)
}

func (n *Node) AddOut(e *Edge) {
	if len(n.outMap) == 0 {
		n.UpdateInOutMap()
	}
	if _, ok := n.outMap[e.String()]; ok {
		return
	}
	n.outMap[e.String()] = e
	n.Out = append(n.Out, e)
}

func (n *Node) AddEnhancementIn(e *Edge) {
	if len(n.humanReadableInMap) == 0 {
		n.UpdateHumanReadableMap()
	}
	if _, ok := n.humanReadableInMap[e.ReadableString()]; ok {
		return
	}
	n.humanReadableInMap[e.ReadableString()] = e
	n.In = append(n.In, e)
}

func (n *Node) AddEnhancementOut(e *Edge) {
	if len(n.humanReadableOutMap) == 0 {
		n.UpdateHumanReadableMap()
	}
	if _, ok := n.humanReadableOutMap[e.ReadableString()]; ok {
		return
	}
	n.humanReadableOutMap[e.ReadableString()] = e
	n.Out = append(n.Out, e)
}

func (n *Node) SetPath(path []*Edge) {
	n.path = path
}

func (n *Node) GetPath() []*Edge {
	return n.path
}

func (n *Node) ClearPath() {
	n.path = nil
}

func (n *Node) IsSearched() bool {
	return n.searched
}

func (n *Node) SetSearched() {
	n.searched = true
}

func (n *Node) ResetSearched() {
	n.searched = false
}

func (n *Node) DeleteIns() {
	for _, e := range n.In {
		removeOutEdge(e)
	}
	n.In = nil
}

func removeOutEdge(edge *Edge) {
	caller := edge.Caller
	n := len(caller.Out)
	for i, e := range caller.Out {
		if e.String() == edge.String() {
			caller.Out[i] = caller.Out[n-1]
			caller.Out[n-1] = nil
			caller.Out = caller.Out[:n-1]
			return
		}
	}
}

func (n *Node) DeleteOuts() {
	for _, e := range n.Out {
		removeInEdge(e)
	}
	n.Out = nil
}

func removeInEdge(edge *Edge) {
	caller := edge.Callee
	n := len(caller.In)
	for i, e := range caller.In {
		if e.String() == edge.String() {
			caller.In[i] = caller.In[n-1]
			caller.In[n-1] = nil
			caller.In = caller.In[:n-1]
			return
		}
	}
}

type Edge struct {
	Caller *Node
	Site   *Site
	Callee *Node
}

func (e *Edge) ToRenderEdge() *render.Edge {
	return &render.Edge{
		From:   e.Caller.ID,
		To:     e.Callee.ID,
		Name:   util.GetSiteSimpleName(e.Site.Name),
		Detail: e.Site.Name,
	}
}

func (e *Edge) ReadableString() string {
	if e == nil {
		return ""
	}
	callerFuncName := ""
	if e.Caller != nil && e.Caller.Func != nil {
		callerFuncName = e.Caller.Func.Name
	}
	siteName := ""
	if e.Site != nil {
		siteName = e.Site.Name
		if strings.Contains(siteName, "->Skip(") {
			siteName = "Skip()"
		}
	}
	calleeFuncName := ""
	if e.Callee != nil && e.Callee.Func != nil {
		calleeFuncName = e.Callee.Func.Name
	}
	return fmt.Sprintf("%s-|%s|->%s=", callerFuncName, siteName, calleeFuncName)
}

func (e *Edge) String() string {
	if e == nil {
		return ""
	}
	callerFuncAddr := ""
	if e.Caller != nil && e.Caller.Func != nil {
		callerFuncAddr = e.Caller.Func.Addr
	}
	siteAddr := ""
	if e.Site != nil {
		siteAddr = e.Site.Addr
		if strings.Contains(siteAddr, "->Skip(") {
			siteAddr = "Skip()"
		}
	}
	calleeFuncAddr := ""
	if e.Callee != nil && e.Callee.Func != nil {
		calleeFuncAddr = e.Callee.Func.Addr
	}
	return fmt.Sprintf("%s-|%s|->%s=", callerFuncAddr, siteAddr, calleeFuncAddr)
}

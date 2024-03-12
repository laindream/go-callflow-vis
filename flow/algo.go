package flow

import (
	"errors"
	"fmt"
	"github.com/eapache/queue/v2"
	"github.com/laindream/go-callflow-vis/cache"
	"github.com/laindream/go-callflow-vis/ir"
	"github.com/laindream/go-callflow-vis/log"
)

func (f *Flow) resetCallgraphIR() error {
	return cache.GetFury().Unmarshal(f.furyBuffer, &f.callgraph)
}

func (f *Flow) GetMinNodeSet() (map[*ir.Node]bool, error) {
	minNodeSet := make(map[*ir.Node]bool)
	if f == nil {
		return nil, errors.New("flow is nil")
	}
	if len(f.Layers) < 2 {
		return nil, errors.New("number of layers must be at least 2")
	}
	for i, _ := range f.Layers {
		if i == len(f.Layers)-1 {
			continue
		}
		startNodes := f.Layers[i].GetOutAllNodeSet(f.callgraph)
		endNodes := f.Layers[i+1].GetInAllNodeSet(f.callgraph)
		nodeSet, _ := f.findReachableNodesIR(startNodes, endNodes, nil)
		for k, _ := range nodeSet {
			minNodeSet[k] = true
		}
	}
	f.resetLayer()
	return minNodeSet, nil
}

func (f *Flow) findAllBipartite(isCallgraphJustReset bool) error {
	if f == nil {
		return errors.New("flow is nil")
	}
	if len(f.Layers) < 2 {
		return errors.New("number of layers must be at least 2")
	}
	issueFuncs := make(map[string]bool)
	for i, _ := range f.Layers {
		if i == len(f.Layers)-1 {
			continue
		}
		startNodes := f.Layers[i].GetOutAllNodeSet(f.callgraph)
		endNodes := f.Layers[i+1].GetInAllNodeSet(f.callgraph)
		var examplePath map[*ir.Node]map[*ir.Node][]*ir.Edge
		_, examplePath = f.findReachableNodesIR(startNodes, endNodes, nil)
		f.Layers[i].ExamplePath = examplePath
		for _, v := range examplePath {
			for _, v2 := range v {
				isCheckPass, issueFunc := f.checkCallEdgeChain(v2)
				if !isCheckPass && issueFunc != nil {
					issueFuncs[issueFunc.Addr] = true
				}
			}
		}
		starts, ends := GetStartAndEndFromExamplePath(examplePath)
		for j, _ := range f.Layers[i].Entities {
			f.Layers[i].Entities[j].TrimOutNodeSet(starts, f.callgraph)
		}
		for j, _ := range f.Layers[i+1].Entities {
			f.Layers[i+1].Entities[j].TrimInNodeSet(ends, f.callgraph)
		}
		if i == 0 {
			for j, _ := range f.Layers[i].Entities {
				f.Layers[i].Entities[j].UpdateInSiteNodeSetWithNodeSet(f.callgraph)
			}
		}
		if i == len(f.Layers)-2 {
			for j, _ := range f.Layers[i+1].Entities {
				f.Layers[i+1].Entities[j].UpdateOutSiteNodeSetWithNodeSet(f.callgraph)
			}
		}
	}
	if len(issueFuncs) > 0 {
		addCount := 0
		for k, _ := range issueFuncs {
			if f.allIssueFuncs == nil {
				f.allIssueFuncs = make(map[string]bool)
			}
			if _, ok := f.allIssueFuncs[k]; !ok {
				f.allIssueFuncs[k] = true
				addCount++
			}
		}
		log.GetLogger().Debugf("Find Incremental Issue Funcs:%d, Total Issue Funcs:%d, Regenerate Flow",
			addCount, len(f.allIssueFuncs))
		f.skipNodesIR(f.allIssueFuncs)
		f.resetLayer()
		return f.findAllBipartite(false)
	}
	if !isCallgraphJustReset && len(issueFuncs) == 0 {
		log.GetLogger().Debugf("No Incremental Issue Funcs, Try No Issue Check")
		if err := f.resetCallgraphIR(); err != nil {
			return err
		}
		f.skipNodesIR(f.allIssueFuncs)
		f.resetLayer()
		return f.findAllBipartite(true)
	}
	for i := len(f.Layers) - 2; i >= 0; i-- {
		for j, _ := range f.Layers[i].Entities {
			originalEntityIn := make(map[*ir.Node]bool)
			for k, _ := range f.Layers[i].Entities[j].GetInAllNodeSetOnlyRead(f.callgraph) {
				originalEntityIn[k] = true
			}
			originalEntityNode := make(map[*ir.Node]bool)
			for k, _ := range f.Layers[i].Entities[j].GetNodeSet(f.callgraph) {
				originalEntityNode[k] = true
			}
			f.Layers[i].Entities[j].UpdateInSiteNodeSetWithNodeSet(f.callgraph)
			f.Layers[i].Entities[j].TrimInNodeSet(originalEntityIn, f.callgraph)
			f.Layers[i].Entities[j].TrimNodeSet(originalEntityNode, f.callgraph)
		}
		if i == 0 {
			break
		}
		filterOutSet := make(map[*ir.Node]bool)
		for k, _ := range f.Layers[i-1].ExamplePath {
			for k2, _ := range f.Layers[i-1].ExamplePath[k] {
				if f.Layers[i].GetInAllNodeSetOnlyRead(f.callgraph) != nil &&
					f.Layers[i].GetInAllNodeSetOnlyRead(f.callgraph)[k2] {
					continue
				}
				delete(f.Layers[i-1].ExamplePath[k], k2)
			}
			if len(f.Layers[i-1].ExamplePath[k]) == 0 {
				delete(f.Layers[i-1].ExamplePath, k)
			}
		}
		for k, _ := range f.Layers[i-1].ExamplePath {
			filterOutSet[k] = true
		}
		for j, _ := range f.Layers[i-1].Entities {
			f.Layers[i-1].Entities[j].TrimOutNodeSet(filterOutSet, f.callgraph)
		}
	}
	return nil
}

func (f *Flow) skipNodesIR(issueFuncs map[string]bool) {
	hasDoSkip := true
	for hasDoSkip {
		_, hasDoSkip = f.skipNodeIR(issueFuncs)
	}
}

func (f *Flow) skipNodeIR(issueFuncs map[string]bool) (hasFound, hasDoSkip bool) {
	isFuncTarget := func(name string) bool {
		for k, _ := range issueFuncs {
			if k == name {
				return true
			}
		}
		return false
	}
	var nodeToSkip *ir.Node
	for k, v := range f.callgraph.Nodes {
		if v.Func != nil && isFuncTarget(k) {
			nodeToSkip = v
			break
		}
	}
	if nodeToSkip == nil {
		return false, false
	}
	hasFound = true
	cacheIn := make([]*ir.Edge, 0)
	cacheOut := make([]*ir.Edge, 0)
	for _, v := range nodeToSkip.In {
		cacheIn = append(cacheIn, v)
	}
	for _, v := range nodeToSkip.Out {
		cacheOut = append(cacheOut, v)
	}
	f.callgraph = f.callgraph.DeleteNode(nodeToSkip)
	for _, in := range cacheIn {
		for _, out := range cacheOut {
			if in.Caller == nil || out.Callee == nil ||
				in.Caller.Func == nil || out.Callee.Func == nil {
				continue
			}
			inCallerFuncAddr := in.Caller.Func.Addr
			outCalleeFuncAddr := out.Callee.Func.Addr
			doMerge := false
			if isFuncTarget(outCalleeFuncAddr) || isFuncTarget(inCallerFuncAddr) {
				doMerge = true
			}
			if out.Callee.Func.Parent != nil && out.Callee.Func.Parent.Name == in.Caller.Func.Name {
				doMerge = true
			}
			mergedSite := &ir.Site{
				Name: fmt.Sprintf("%s->Skip(%s)->%s", in.Site.Name, nodeToSkip.Func.Name, out.Site.Name),
				Addr: fmt.Sprintf("%s->Skip(%s)->%s", in.Site.Addr, nodeToSkip.Func.Addr, out.Site.Addr),
			}
			if doMerge {
				f.callgraph.AddEdge(inCallerFuncAddr, outCalleeFuncAddr, mergedSite)
				hasDoSkip = true
			}
		}
	}
	if !hasDoSkip && hasFound {
		return f.skipNodeIR(issueFuncs)
	}
	return hasFound, hasDoSkip
}

func (f *Flow) checkCallEdgeChain(path []*ir.Edge) (isCheckPass bool, issueFunc *ir.Func) {
	if len(path) == 0 {
		return false, nil
	}
	fChain := make([]*ir.Func, 0)
	for i, _ := range path {
		var lastFunc *ir.Func
		if len(fChain) > 0 {
			lastFunc = fChain[len(fChain)-1]
		}
		if path[i].Caller != nil &&
			path[i].Caller.Func != nil &&
			path[i].Caller.Func != lastFunc {
			fChain = append(fChain, path[i].Caller.Func)
		}
		if path[i].Callee != nil &&
			path[i].Callee.Func != nil &&
			path[i].Callee.Func != lastFunc {
			fChain = append(fChain, path[i].Callee.Func)
		}
	}
	fChainMap := make(map[string]bool)
	for i, _ := range fChain {
		if fChain[i].Parent != nil {
			if _, ok := fChainMap[fChain[i].Parent.Addr]; !ok {
				if i == 0 {
					fChainMap[fChain[i].Parent.Addr] = true
				} else {
					return false, fChain[i-1]
				}
			}
		}
		fChainMap[fChain[i].Addr] = true
	}
	return true, nil
}

func (f *Flow) findReachableNodesIR(starts map[*ir.Node]bool, ends map[*ir.Node]bool,
	innerContainSet map[*ir.Node]bool) (containSet map[*ir.Node]bool, examplePath map[*ir.Node]map[*ir.Node][]*ir.Edge) {
	containSet = f.searchReachableNodesFromEnds(ends, innerContainSet)
	containSet = f.searchReachableNodesFromStarts(starts, containSet)
	examplePath = make(map[*ir.Node]map[*ir.Node][]*ir.Edge)
	for k, _ := range starts {
		ep := f.findExamplePath(k, ends, containSet)
		if len(ep) > 0 {
			examplePath[k] = ep
		}
	}
	return containSet, examplePath
}

func (f *Flow) findExamplePath(src *ir.Node, dsts map[*ir.Node]bool,
	containSet map[*ir.Node]bool) (examplePath map[*ir.Node][]*ir.Edge) {
	examplePath = make(map[*ir.Node][]*ir.Edge)
	q := queue.New[*ir.Node]()
	visited := make(map[*ir.Node]bool)
	visited[src] = true
	q.Add(src)
	for q.Length() > 0 {
		node := q.Remove()
		if dsts[node] && len(node.GetPath()) > 0 {
			examplePath[node] = node.GetPath()
			continue
		}
		for _, e := range node.Out {
			w := e.Callee
			if len(containSet) == 0 || (len(containSet) != 0 && containSet[w]) {
				if _, ok := visited[w]; ok {
					continue
				}
				newPath := make([]*ir.Edge, 0)
				newPath = append(newPath, node.GetPath()...)
				w.SetPath(append(newPath, e))
				visited[w] = true
				q.Add(w)
			}
		}
	}
	for _, v := range f.callgraph.Nodes {
		v.ClearPath()
	}
	return examplePath
}

func (f *Flow) searchReachableNodesFromEnds(ends map[*ir.Node]bool, containSet map[*ir.Node]bool) map[*ir.Node]bool {
	var (
		visited = make(map[*ir.Node]bool)
		q       = queue.New[*ir.Node]()
	)
	layerCount := 0
	layerList := make([]map[*ir.Node]bool, 0)
	layerList = append(layerList, make(map[*ir.Node]bool))
	for nodes, _ := range ends {
		q.Add(nodes)
		layerCount++
		layerList[0][nodes] = true
		visited[nodes] = true
	}
	for q.Length() > 0 {
		innerLayerCount := 0
		stepMap := make(map[*ir.Node]bool)
		for i := 0; i < layerCount; i++ {
			v := q.Remove()
			for _, edge := range v.In {
				w := edge.Caller
				if len(containSet) == 0 || (len(containSet) != 0 && containSet[w]) {
					if _, ok := visited[w]; ok {
						continue
					}
					visited[w] = true
					q.Add(w)
					innerLayerCount++
					stepMap[w] = true
				}
			}
		}
		layerCount = innerLayerCount
		if len(stepMap) > 0 {
			layerList = append(layerList, stepMap)
		}
	}
	return visited
}

func (f *Flow) searchReachableNodesFromStarts(starts map[*ir.Node]bool, containSet map[*ir.Node]bool) map[*ir.Node]bool {
	var (
		visited = make(map[*ir.Node]bool)
		q       = queue.New[*ir.Node]()
	)
	layerCount := 0
	layerList := make([]map[*ir.Node]bool, 0)
	layerList = append(layerList, make(map[*ir.Node]bool))
	for nodes, _ := range starts {
		q.Add(nodes)
		layerCount++
		layerList[0][nodes] = true
		visited[nodes] = true
	}
	for q.Length() > 0 {
		innerLayerCount := 0
		stepMap := make(map[*ir.Node]bool)
		for i := 0; i < layerCount; i++ {
			v := q.Remove()
			for _, edge := range v.Out {
				w := edge.Callee
				if len(containSet) == 0 || (len(containSet) != 0 && containSet[w]) {
					if _, ok := visited[w]; ok {
						continue
					}
					visited[w] = true
					q.Add(w)
					innerLayerCount++
					stepMap[w] = true
				}
			}
		}
		layerCount = innerLayerCount
		if len(stepMap) > 0 {
			layerList = append(layerList, stepMap)
		}
	}
	return visited
}

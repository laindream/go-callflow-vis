package flow

import (
	"fmt"
	"github.com/awalterschulze/gographviz"
	"github.com/laindream/go-callflow-vis/cache"
	"github.com/laindream/go-callflow-vis/config"
	"github.com/laindream/go-callflow-vis/ir"
	"github.com/laindream/go-callflow-vis/log"
	"github.com/laindream/go-callflow-vis/mode"
	"github.com/laindream/go-callflow-vis/render"
	"github.com/laindream/go-callflow-vis/util"
	"strconv"
)

func NewFlow(config *config.Config, callGraph *ir.Callgraph) (*Flow, error) {
	if config == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if len(config.Layers) == 0 {
		return nil, fmt.Errorf("no layers in config")
	}
	if callGraph == nil {
		return nil, fmt.Errorf("callgraph is nil")
	}
	layers := make([]*Layer, 0)
	for _, l := range config.Layers {
		layer := &Layer{
			Name:     l.Name,
			Entities: make([]*Entity, 0),
		}
		for _, e := range l.Entities {
			entity := &Entity{
				Entity: e,
			}
			layer.Entities = append(layer.Entities, entity)
		}
		layers = append(layers, layer)
	}
	f := &Flow{
		pkgPrefix: config.PackagePrefix,
		callgraph: callGraph,
		Layers:    layers,
	}
	log.GetLogger().Debugf("NewFlow: Generate Min Graph...")
	err := f.UpdateMinGraph()
	if err != nil {
		return nil, err
	}
	err = f.initFuryBuffer()
	if err != nil {
		return nil, err
	}
	return f, nil
}

type Flow struct {
	pkgPrefix          string
	furyBuffer         []byte
	callgraph          *ir.Callgraph
	allIssueFuncs      map[string]bool
	Layers             []*Layer
	isCompleteGenerate bool
}

func (f *Flow) initFuryBuffer() error {
	bytes, err := cache.GetFury().Marshal(f.callgraph)
	if err != nil {
		return err
	}
	f.furyBuffer = bytes
	return nil
}

func (f *Flow) reset() {
	f.allIssueFuncs = nil
	f.resetLayer()
}

func (f *Flow) resetLayer() {
	for _, l := range f.Layers {
		l.ResetLayer()
	}
}

func (f *Flow) UpdateMinGraph() error {
	minSet, err := f.GetMinNodeSet()
	if err != nil {
		return err
	}
	funcNameSet := make(map[string]bool)
	for n, _ := range minSet {
		if n.Func == nil {
			continue
		}
		funcNameSet[n.Func.Name] = true
	}
	f.callgraph = ir.GetFilteredCallgraph(f.callgraph, func(funcName string) bool {
		return funcNameSet[funcName]
	})
	return nil
}

func (f *Flow) Generate() error {
	log.GetLogger().Debugf("Flow.Generate: Start...")
	if f.isCompleteGenerate {
		return nil
	}
	err := f.findAllBipartite()
	if err != nil {
		f.reset()
		return err
	}
	f.isCompleteGenerate = true
	f.PrintFlow()
	log.GetLogger().Debugf("Flow.Generate: Done")
	return nil
}

func (f *Flow) PrintFlow() {
	log.GetLogger().Debugf("Flow issueFuncs:%d", len(f.allIssueFuncs))
	for i, _ := range f.Layers {
		inAllNode := len(f.Layers[i].GetInAllNodeSetOnlyRead(f.callgraph))
		outAllNode := len(f.Layers[i].GetOutAllNodeSetOnlyRead(f.callgraph))
		start, end := GetStartAndEndFromExamplePath(f.Layers[i].ExamplePath)
		log.GetLogger().Debugf("Layers[%d] InAllNode:%d, OutAllNode:%d, start:%d, end:%d",
			i, inAllNode, outAllNode, len(start), len(end))
	}
}

func (f *Flow) SavePaths(path string, separator string) error {
	if separator == "" {
		separator = ","
	}
	for i, _ := range f.Layers {
		if i == len(f.Layers)-1 {
			continue
		}
		t := fmt.Sprintf("%s%s%s%s%s", "\"Caller\"", separator, "\"Callee\"", separator, "\"ExamplePath\"\n")
		for from, _ := range f.Layers[i].ExamplePath {
			for to, p := range f.Layers[i].ExamplePath[from] {
				if len(p) == 0 {
					continue
				}
				path := ""
				for _, e := range p {
					path += fmt.Sprintf("%s", e.ReadableString())
				}
				t += fmt.Sprintf("%s%s%s%s%s", "\""+util.Escape(from.Func.Name)+"\"", separator, "\""+util.Escape(to.Func.Name)+"\"", separator, "\""+util.Escape(path)+"\"")
				t += "\n"
			}
		}
		if err := util.WriteToFile(t, fmt.Sprintf("%s/%d-%d.csv", path, i, i+1)); err != nil {
			return err
		}
	}
	return nil
}

func (f *Flow) GetRenderGraph() *render.Graph {
	nodes := make(map[string]*render.Node)
	edges := make(map[string]*render.Edge)
	for _, l := range f.Layers {
		for k, _ := range l.ExamplePath {
			for k2, _ := range l.ExamplePath[k] {
				for _, e := range l.ExamplePath[k][k2] {
					edges[e.String()] = e.ToRenderEdge(f.pkgPrefix)
					nodes[e.Caller.Func.Addr] = e.Caller.ToRenderNode(f.pkgPrefix)
					nodes[e.Callee.Func.Addr] = e.Callee.ToRenderNode(f.pkgPrefix)
				}
			}
		}
		for k, _ := range l.GetInToOutEdgeSet(f.callgraph) {
			for k2, _ := range l.GetInToOutEdgeSet(f.callgraph)[k] {
				for _, e := range l.GetInToOutEdgeSet(f.callgraph)[k][k2] {
					for _, e2 := range e {
						if e2.Callee == nil && e2.Caller == nil {
							continue
						}
						if e2.Callee == nil && e2.Caller != nil {
							nodes[e2.Caller.Func.Addr] = e2.Caller.ToRenderNode(f.pkgPrefix)
							continue
						}
						if e2.Callee != nil && e2.Caller == nil {
							nodes[e2.Callee.Func.Addr] = e2.Callee.ToRenderNode(f.pkgPrefix)
							continue
						}
						edges[e2.String()] = e2.ToRenderEdge(f.pkgPrefix)
						nodes[e2.Caller.Func.Addr] = e2.Caller.ToRenderNode(f.pkgPrefix)
						nodes[e2.Callee.Func.Addr] = e2.Callee.ToRenderNode(f.pkgPrefix)
					}
				}
			}
		}
	}
	setIndex := 0
	for _, l := range f.Layers {
		layerInSet := make(map[*ir.Node]bool)
		layerNodeSet := make(map[*ir.Node]bool)
		layerOutSet := make(map[*ir.Node]bool)
		for _, entity := range l.Entities {
			for n, _ := range entity.InNodeSet {
				layerInSet[n] = true
			}
			for n, _ := range entity.NodeSet {
				layerNodeSet[n] = true
			}
			for n, _ := range entity.OutNodeSet {
				layerOutSet[n] = true
			}
		}
		if len(layerInSet) > 0 {
			for n, _ := range layerInSet {
				nodes[n.Func.Addr] = n.ToRenderNode(f.pkgPrefix)
				nodes[n.Func.Addr].Set = setIndex
			}
			setIndex++
		}
		if len(layerNodeSet) > 0 {
			for n, _ := range layerNodeSet {
				nodes[n.Func.Addr] = n.ToRenderNode(f.pkgPrefix)
				nodes[n.Func.Addr].Set = setIndex
			}
			setIndex++
		}
		if len(layerOutSet) > 0 {
			for n, _ := range layerOutSet {
				nodes[n.Func.Addr] = n.ToRenderNode(f.pkgPrefix)
				nodes[n.Func.Addr].Set = setIndex
			}
			setIndex++
		}
	}
	nodeSet := make([]*render.Node, 0)
	for _, n := range nodes {
		nodeSet = append(nodeSet, n)
	}
	edgeSet := make([]*render.Edge, 0)
	for _, e := range edges {
		edgeSet = append(edgeSet, e)
	}
	return &render.Graph{
		NodeSet: nodeSet,
		EdgeSet: edgeSet,
	}
}

func (f *Flow) GetDot(isSimple bool) string {
	mainGraph := gographviz.NewGraph()
	graphName := "Flow"
	mainGraph.SetName(graphName)
	mainGraph.SetDir(true)
	mainGraph.AddAttr(graphName, "rankdir", "LR")
	mainGraph.AddAttr(graphName, "newrank", "true")
	nodes := make(map[string]*ir.Node)
	edges := make(map[string]*ir.Edge)
	for _, l := range f.Layers {
		for k, _ := range l.ExamplePath {
			for k2, _ := range l.ExamplePath[k] {
				if isSimple {
					simpleEdge := getSimpleEdgeForPath(l.ExamplePath[k][k2])
					if simpleEdge == nil {
						continue
					}
					edges[simpleEdge.String()] = simpleEdge
					nodes[simpleEdge.Caller.Func.Addr] = simpleEdge.Caller
					nodes[simpleEdge.Callee.Func.Addr] = simpleEdge.Callee
					continue
				}
				for _, e := range l.ExamplePath[k][k2] {
					edges[e.String()] = e
					nodes[e.Caller.Func.Addr] = e.Caller
					nodes[e.Callee.Func.Addr] = e.Callee
				}
			}
		}
		for k, _ := range l.GetInToOutEdgeSet(f.callgraph) {
			for k2, _ := range l.GetInToOutEdgeSet(f.callgraph)[k] {
				for _, e := range l.GetInToOutEdgeSet(f.callgraph)[k][k2] {
					for _, e2 := range e {
						if e2.Callee == nil && e2.Caller == nil {
							continue
						}
						if e2.Callee == nil && e2.Caller != nil {
							nodes[e2.Caller.Func.Addr] = e2.Caller
							continue
						}
						if e2.Callee != nil && e2.Caller == nil {
							nodes[e2.Callee.Func.Addr] = e2.Callee
							continue
						}
						edges[e2.String()] = e2
						nodes[e2.Caller.Func.Addr] = e2.Caller
						nodes[e2.Callee.Func.Addr] = e2.Callee
					}
				}
			}
		}
	}
	for _, n := range nodes {
		if n.Func == nil {
			continue
		}
		mainGraph.AddNode(graphName, strconv.Itoa(n.ID), map[string]string{
			"label": "\"" + util.Escape(util.GetFuncSimpleName(n.Func.Name, f.pkgPrefix)) + "\"",
		})
	}
	for _, e := range edges {
		if e.Callee == nil || e.Caller == nil {
			continue
		}
		callerID := strconv.Itoa(e.Caller.ID)
		calleeID := strconv.Itoa(e.Callee.ID)
		mainGraph.AddEdge(callerID, calleeID, true, map[string]string{
			"label": "\"" + util.Escape(util.GetSiteSimpleName(e.Site.Name, f.pkgPrefix)) + "\"",
			//"constraint": "false",
		})
	}
	for i, l := range f.Layers {
		layerInSet := make(map[*ir.Node]bool)
		layerNodeSet := make(map[*ir.Node]bool)
		layerOutSet := make(map[*ir.Node]bool)
		for _, entity := range l.Entities {
			for n, _ := range entity.InNodeSet {
				layerInSet[n] = true
			}
			for n, _ := range entity.NodeSet {
				layerNodeSet[n] = true
			}
			for n, _ := range entity.OutNodeSet {
				layerOutSet[n] = true
			}
		}
		attr := map[string]string{
			"rank":  "same",
			"style": "invis",
		}
		if len(layerInSet) > 0 {
			subGraphName := fmt.Sprintf("\"cluster_%d-in\"", i)
			mainGraph.AddSubGraph(graphName, subGraphName, attr)
			for n, _ := range layerInSet {
				nodeID := strconv.Itoa(n.ID)
				mainGraph.AddNode(subGraphName, nodeID, map[string]string{
					"label": "\"" + util.Escape(util.GetFuncSimpleName(n.Func.Name, f.pkgPrefix)) + "\"",
					"color": "green",
				})
			}
		}
		if len(layerNodeSet) > 0 {
			subGraphName := fmt.Sprintf("cluster_%d", i)
			mainGraph.AddSubGraph(graphName, subGraphName, attr)
			for n, _ := range layerNodeSet {
				nodeID := strconv.Itoa(n.ID)
				mainGraph.AddNode(subGraphName, nodeID, map[string]string{
					"label": "\"" + util.Escape(util.GetFuncSimpleName(n.Func.Name, f.pkgPrefix)) + "\"",
					"color": "red",
				})
			}
		}
		if len(layerOutSet) > 0 {
			subGraphName := fmt.Sprintf("\"cluster_%d-out\"", i)
			mainGraph.AddSubGraph(graphName, subGraphName, attr)
			for n, _ := range layerOutSet {
				nodeID := strconv.Itoa(n.ID)
				mainGraph.AddNode(subGraphName, nodeID, map[string]string{
					"label": "\"" + util.Escape(util.GetFuncSimpleName(n.Func.Name, f.pkgPrefix)) + "\"",
					"color": "yellow",
				})
			}
		}
	}
	return mainGraph.String()
}

func (f *Flow) SaveDot(filename string, isSimple bool) error {
	dotString := f.GetDot(isSimple)
	return util.WriteToFile(dotString, filename)
}

type Layer struct {
	Name        string
	Entities    []*Entity
	NodeSet     map[*ir.Node]bool
	ExamplePath map[*ir.Node]map[*ir.Node][]*ir.Edge
}

func (l *Layer) GetInToOutEdgeSet(callgraphIR *ir.Callgraph) map[*ir.Node]map[*ir.Node][][]*ir.Edge {
	inToOutSet := make(map[*ir.Node]map[*ir.Node][][]*ir.Edge)
	for _, e := range l.Entities {
		entityInToOutSet := e.GetInToOutEdgeSet(callgraphIR)
		for k, _ := range entityInToOutSet {
			if _, ok := inToOutSet[k]; !ok {
				inToOutSet[k] = make(map[*ir.Node][][]*ir.Edge)
			}
			for k2, _ := range entityInToOutSet[k] {
				inToOutSet[k][k2] = entityInToOutSet[k][k2]
			}
		}
	}
	return inToOutSet
}

func (l *Layer) ResetLayer() {
	l.NodeSet = nil
	l.ExamplePath = nil
	for _, e := range l.Entities {
		e.ResetEntity()
	}
}

func (l *Layer) GetExamplePath(callgraphIR *ir.Callgraph) map[*ir.Node]map[*ir.Node][]*ir.Edge {
	if callgraphIR == nil {
		return nil
	}
	if l.ExamplePath != nil {
		return l.ExamplePath
	}
	examplePath := make(map[*ir.Node]map[*ir.Node][]*ir.Edge)
	for _, v := range l.Entities {
		for k, _ := range v.ExamplePath {
			if _, ok := examplePath[k]; !ok {
				examplePath[k] = make(map[*ir.Node][]*ir.Edge)
			}
			for k2, _ := range v.ExamplePath[k] {
				examplePath[k][k2] = v.ExamplePath[k][k2]
			}
		}
	}
	l.ExamplePath = examplePath
	return examplePath
}

func (l *Layer) GetStartAndEndFromExamplePath() (startNodeSet, endNodeSet map[*ir.Node]bool) {
	if l.ExamplePath == nil {
		return
	}
	return GetStartAndEndFromExamplePath(l.ExamplePath)
}

func GetStartAndEndFromExamplePath(examplePath map[*ir.Node]map[*ir.Node][]*ir.Edge) (startNodeSet, endNodeSet map[*ir.Node]bool) {
	if examplePath == nil {
		return
	}
	startNodeSet = make(map[*ir.Node]bool)
	endNodeSet = make(map[*ir.Node]bool)
	for from, v := range examplePath {
		for to, _ := range v {
			if len(examplePath[from][to]) > 0 {
				startNodeSet[from] = true
				endNodeSet[to] = true
			}
		}
	}
	return
}

func (l *Layer) GetNodeSet(callgraphIR *ir.Callgraph) map[*ir.Node]bool {
	if callgraphIR == nil {
		return nil
	}
	nodesSet := make(map[*ir.Node]bool)
	for i, _ := range l.Entities {
		for v, _ := range l.Entities[i].GetNodeSet(callgraphIR) {
			if matchesEntityIR(v, l.Entities[i]) {
				nodesSet[v] = true
			}
		}
	}
	l.NodeSet = nodesSet
	return nodesSet
}

func (l *Layer) GetOutEdgeSet(callgraphIR *ir.Callgraph) map[string]*ir.Edge {
	if callgraphIR == nil {
		return nil
	}
	edgesSet := make(map[string]*ir.Edge)
	for i, _ := range l.Entities {
		if l.Entities[i].OutSite == nil {
			continue
		}
		for n, _ := range l.Entities[i].OutNodeSet {
			for _, e := range n.In {
				if !isSiteMatchIR(e.Site, l.Entities[i].OutSite) {
					continue
				}
				edgesSet[e.String()] = e
			}
		}
	}
	return edgesSet
}

func (l *Layer) GetOutAllNodeSet(callgraphIR *ir.Callgraph) map[*ir.Node]bool {
	if callgraphIR == nil {
		return nil
	}
	nodesSet := make(map[*ir.Node]bool)
	for i, _ := range l.Entities {
		for n, _ := range l.Entities[i].GetOutAllNodeSet(callgraphIR) {
			nodesSet[n] = true
		}
	}
	return nodesSet
}

func (l *Layer) GetOutAllNodeSetOnlyRead(callgraphIR *ir.Callgraph) map[*ir.Node]bool {
	if callgraphIR == nil {
		return nil
	}
	nodesSet := make(map[*ir.Node]bool)
	for i, _ := range l.Entities {
		for n, _ := range l.Entities[i].GetOutAllNodeSetOnlyRead(callgraphIR) {
			nodesSet[n] = true
		}
	}
	return nodesSet
}

func (l *Layer) GetInEdgeSet(callgraphIR *ir.Callgraph) map[string]*ir.Edge {
	if callgraphIR == nil {
		return nil
	}
	edgesSet := make(map[string]*ir.Edge)
	for i, _ := range l.Entities {
		if l.Entities[i].InSite == nil {
			continue
		}
		for n, _ := range l.Entities[i].InNodeSet {
			for _, e := range n.Out {
				if !isSiteMatchIR(e.Site, l.Entities[i].InSite) {
					continue
				}
				edgesSet[e.String()] = e
			}
		}
	}
	return edgesSet
}

func (l *Layer) GetInAllNodeSet(callgraphIR *ir.Callgraph) map[*ir.Node]bool {
	if callgraphIR == nil {
		return nil
	}
	nodesSet := make(map[*ir.Node]bool)
	for i, _ := range l.Entities {
		for n, _ := range l.Entities[i].GetInAllNodeSet(callgraphIR) {
			nodesSet[n] = true
		}
	}
	return nodesSet
}

func (l *Layer) GetInAllNodeSetOnlyRead(callgraphIR *ir.Callgraph) map[*ir.Node]bool {
	if callgraphIR == nil {
		return nil
	}
	nodesSet := make(map[*ir.Node]bool)
	for i, _ := range l.Entities {
		for n, _ := range l.Entities[i].GetInAllNodeSetOnlyRead(callgraphIR) {
			nodesSet[n] = true
		}
	}
	return nodesSet
}

type Entity struct {
	*config.Entity
	NodeSet     map[*ir.Node]bool
	InNodeSet   map[*ir.Node]bool
	OutNodeSet  map[*ir.Node]bool
	ExamplePath map[*ir.Node]map[*ir.Node][]*ir.Edge
}

func (e *Entity) GetInToOutEdgeSet(callgraphIR *ir.Callgraph) map[*ir.Node]map[*ir.Node][][]*ir.Edge {
	inToOutSet := make(map[*ir.Node]map[*ir.Node][][]*ir.Edge)
	if e.InSite == nil && e.OutSite == nil {
		for k, _ := range e.GetNodeSet(callgraphIR) {
			oneNodeEdge := &ir.Edge{Caller: k}
			oneNodeEdgeSet := make(map[*ir.Node][][]*ir.Edge)
			oneNodeEdgeSet[k] = append(make([][]*ir.Edge, 0), append(make([]*ir.Edge, 0), oneNodeEdge))
			inToOutSet[k] = oneNodeEdgeSet
		}
		return inToOutSet
	}
	if e.InSite == nil && e.OutSite != nil {
		for k, _ := range e.OutNodeSet {
			for _, eOut := range k.In {
				if eOut.Caller != nil && e.GetNodeSet(callgraphIR)[eOut.Caller] {
					nodeToOutEdge := &ir.Edge{Caller: eOut.Caller, Callee: k, Site: eOut.Site}
					if _, ok := inToOutSet[eOut.Caller]; !ok {
						inToOutSet[eOut.Caller] = make(map[*ir.Node][][]*ir.Edge)
					}
					inToOutSet[eOut.Caller][k] = append(make([][]*ir.Edge, 0), append(make([]*ir.Edge, 0), nodeToOutEdge))
				}
			}
		}
	}
	if e.InSite != nil && e.OutSite == nil {
		for k, _ := range e.InNodeSet {
			for _, eIn := range k.Out {
				if eIn.Callee != nil && e.GetNodeSet(callgraphIR)[eIn.Callee] {
					inToNodeEdge := &ir.Edge{Caller: k, Callee: eIn.Callee, Site: eIn.Site}
					if _, ok := inToOutSet[k]; !ok {
						inToOutSet[k] = make(map[*ir.Node][][]*ir.Edge)
					}
					inToOutSet[k][eIn.Callee] = append(make([][]*ir.Edge, 0), append(make([]*ir.Edge, 0), inToNodeEdge))
				}
			}
		}
	}
	if e.InSite != nil && e.OutSite != nil {
		for k, _ := range e.OutNodeSet {
			for _, eOut := range k.In {
				if eOut.Caller != nil && e.GetNodeSet(callgraphIR)[eOut.Caller] {
					node := eOut.Caller
					for _, eIn := range node.In {
						if eIn.Caller != nil && e.InNodeSet[eIn.Caller] {
							inToNodeEdge := &ir.Edge{Caller: eIn.Caller, Callee: node, Site: eIn.Site}
							nodeToOutEdge := &ir.Edge{Caller: node, Callee: k, Site: eOut.Site}
							if _, ok := inToOutSet[eIn.Caller]; !ok {
								inToOutSet[eIn.Caller] = make(map[*ir.Node][][]*ir.Edge)
							}
							if _, ok := inToOutSet[eIn.Caller][k]; !ok {
								inToOutSet[eIn.Caller][k] = make([][]*ir.Edge, 0)
							}
							path := append(make([]*ir.Edge, 0), inToNodeEdge, nodeToOutEdge)
							inToOutSet[eIn.Caller][k] = append(inToOutSet[eIn.Caller][k], path)
						}
					}
				}
			}
		}
	}
	return inToOutSet
}

func (e *Entity) ResetEntity() {
	e.NodeSet = nil
	e.InNodeSet = nil
	e.OutNodeSet = nil
	e.ExamplePath = nil
}

func (e *Entity) AddExamplePath(ep map[*ir.Node]map[*ir.Node][]*ir.Edge) {
	if e.ExamplePath == nil {
		e.ExamplePath = ep
	}
	for k, _ := range ep {
		if _, ok := e.ExamplePath[k]; !ok {
			e.ExamplePath[k] = make(map[*ir.Node][]*ir.Edge)
		}
		for k2, _ := range ep[k] {
			e.ExamplePath[k][k2] = ep[k][k2]
		}
	}
}

func (e *Entity) GetNodeSet(callgraphIR *ir.Callgraph) map[*ir.Node]bool {
	if callgraphIR == nil {
		return nil
	}
	if e.NodeSet != nil {
		return e.NodeSet
	}
	nodesSet := make(map[*ir.Node]bool)
	for _, v := range callgraphIR.Nodes {
		if matchesEntityIR(v, e) {
			nodesSet[v] = true
		}
	}
	e.NodeSet = nodesSet
	return e.NodeSet
}

func (e *Entity) TrimNodeSet(nodeSet map[*ir.Node]bool, callgraphIR *ir.Callgraph) {
	if e.NodeSet == nil {
		e.GetNodeSet(callgraphIR)
	}
	for n, _ := range e.NodeSet {
		if _, ok := nodeSet[n]; !ok {
			delete(e.NodeSet, n)
		}
	}
}

func (e *Entity) TrimInNodeSet(in map[*ir.Node]bool, callgraphIR *ir.Callgraph) {
	if e.InSite != nil {
		if e.InNodeSet == nil {
			e.UpdateInSiteNodeSetWithNodeSet(callgraphIR)
		}
		for n, _ := range e.InNodeSet {
			if _, ok := in[n]; !ok {
				delete(e.InNodeSet, n)
			}
		}
		e.UpdateNodeSetWithInSiteNodeSet(callgraphIR)
		return
	}
	e.TrimNodeSet(in, callgraphIR)
}

func (e *Entity) TrimOutNodeSet(out map[*ir.Node]bool, callgraphIR *ir.Callgraph) {
	if e.OutSite != nil {
		if e.OutNodeSet == nil {
			e.UpdateOutSiteNodeSetWithNodeSet(callgraphIR)
		}
		for n, _ := range e.OutNodeSet {
			if _, ok := out[n]; !ok {
				delete(e.OutNodeSet, n)
			}
		}
		e.UpdateNodeSetWithOutSiteNodeSet(callgraphIR)
		return
	}
	e.TrimNodeSet(out, callgraphIR)
}

func (e *Entity) GetInAllNodeSet(callgraphIR *ir.Callgraph) map[*ir.Node]bool {
	if callgraphIR == nil {
		return nil
	}
	if e.InSite == nil {
		return e.GetNodeSet(callgraphIR)
	}
	if e.InNodeSet == nil {
		e.UpdateInSiteNodeSetWithNodeSet(callgraphIR)
	}
	return e.InNodeSet
}

func (e *Entity) GetInAllNodeSetOnlyRead(callgraphIR *ir.Callgraph) map[*ir.Node]bool {
	if callgraphIR == nil {
		return nil
	}
	if e.InSite == nil {
		return e.NodeSet
	}
	return e.InNodeSet
}

func (e *Entity) GetOutAllNodeSet(callgraphIR *ir.Callgraph) map[*ir.Node]bool {
	if callgraphIR == nil {
		return nil
	}
	if e.OutSite == nil {
		return e.GetNodeSet(callgraphIR)
	}
	if e.OutNodeSet == nil {
		e.UpdateOutSiteNodeSetWithNodeSet(callgraphIR)
	}
	return e.OutNodeSet
}

func (e *Entity) GetOutAllNodeSetOnlyRead(callgraphIR *ir.Callgraph) map[*ir.Node]bool {
	if callgraphIR == nil {
		return nil
	}
	if e.OutSite == nil {
		return e.NodeSet
	}
	return e.OutNodeSet
}

func (e *Entity) UpdateNodeSetWithInSiteNodeSet(callgraphIR *ir.Callgraph) {
	if callgraphIR == nil {
		return
	}
	if e.InSite == nil {
		e.GetNodeSet(callgraphIR)
		return
	}
	if e.InNodeSet == nil {
		e.UpdateInSiteNodeSetWithNodeSet(callgraphIR)
	}
	nodeSet := make(map[*ir.Node]bool)
	for n, _ := range e.InNodeSet {
		for _, eOut := range n.Out {
			if matchesEntityIR(eOut.Callee, e) {
				nodeSet[eOut.Callee] = true
			}
		}
	}
	if len(nodeSet) > 0 {
		e.NodeSet = nodeSet
	}
}

func (e *Entity) UpdateNodeSetWithOutSiteNodeSet(callgraphIR *ir.Callgraph) {
	if callgraphIR == nil {
		return
	}
	if e.OutSite == nil {
		e.GetNodeSet(callgraphIR)
		return
	}
	if e.OutNodeSet == nil {
		e.UpdateOutSiteNodeSetWithNodeSet(callgraphIR)
	}
	nodeSet := make(map[*ir.Node]bool)
	for n, _ := range e.OutNodeSet {
		for _, eIn := range n.In {
			if matchesEntityIR(eIn.Caller, e) {
				nodeSet[eIn.Caller] = true
			}
		}
	}
	if len(nodeSet) > 0 {
		e.NodeSet = nodeSet
	}
}

func (e *Entity) UpdateInSiteNodeSetWithNodeSet(callgraphIR *ir.Callgraph) map[*ir.Node]bool {
	if callgraphIR == nil {
		return nil
	}
	if e.InSite == nil {
		return nil
	}
	nodesSet := make(map[*ir.Node]bool)
	for v, _ := range e.GetNodeSet(callgraphIR) {
		if matchesEntityIR(v, e) {
			for _, eIn := range v.In {
				if isSiteMatchIR(eIn.Site, e.InSite) {
					nodesSet[eIn.Caller] = true
				}
			}
		}
	}
	if len(nodesSet) > 0 {
		e.InNodeSet = nodesSet
	}
	return nodesSet
}

func (e *Entity) UpdateOutSiteNodeSetWithNodeSet(callgraphIR *ir.Callgraph) map[*ir.Node]bool {
	if callgraphIR == nil {
		return nil
	}
	if e.OutSite == nil {
		return nil
	}
	nodesSet := make(map[*ir.Node]bool)
	for v, _ := range e.GetNodeSet(callgraphIR) {
		if matchesEntityIR(v, e) {
			for _, eOut := range v.Out {
				if isSiteMatchIR(eOut.Site, e.OutSite) {
					nodesSet[eOut.Callee] = true
				}
			}
		}
	}
	if len(nodesSet) > 0 {
		e.OutNodeSet = nodesSet
	}
	return nodesSet
}

func matchesEntityIR(n *ir.Node, entity *Entity) bool {
	if n == nil || n.Func == nil || n.Func.Name == "" || n.Func.Signature == "" {
		return false
	}
	if entity == nil || (entity.Name == nil && entity.InSite == nil && entity.OutSite == nil) {
		return false
	}
	fName := n.Func.Name
	nameCheckPass := true
	if entity.Name != nil {
		nameCheckPass = false
		if entity.Name.Match(fName) {
			nameCheckPass = true
		}
	}
	inSiteCheckPass := true
	if entity.InSite != nil {
		inSiteCheckPass = false
		for _, e := range n.In {
			if isSiteMatchIR(e.Site, entity.InSite) {
				inSiteCheckPass = true
				break
			}
		}
	}
	outSiteCheckPass := true
	if entity.OutSite != nil {
		outSiteCheckPass = false
		for _, e := range n.Out {
			if isSiteMatchIR(e.Site, entity.OutSite) {
				outSiteCheckPass = true
				break
			}
		}

	}
	signatureCheckPass := true
	if entity.Signature != nil {
		signatureCheckPass = false
		fSig := n.Func.Signature
		if entity.Signature.Match(fSig) {
			signatureCheckPass = true
		}
	}
	if nameCheckPass && inSiteCheckPass && outSiteCheckPass && signatureCheckPass {
		return true
	}
	return false
}

func isSiteMatchIR(s *ir.Site, site *mode.Mode) bool {
	if s == nil || s.Name == "" {
		return false
	}
	if site == nil {
		return false
	}
	return site.Match(s.Name)
}

func getSimpleEdgeForPath(path []*ir.Edge) *ir.Edge {
	if len(path) == 0 {
		return nil
	}
	if len(path) == 1 {
		return path[0]
	}
	caller := path[0].Caller
	callee := path[len(path)-1].Callee
	site := fmt.Sprintf("%s->...->%s", path[0].Site.Name, path[len(path)-1].Site.Name)
	return &ir.Edge{Caller: caller, Callee: callee, Site: &ir.Site{Name: site}}
}

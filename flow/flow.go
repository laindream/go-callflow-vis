package flow

import (
	"encoding/json"
	"fmt"
	"github.com/awalterschulze/gographviz"
	"github.com/laindream/go-callflow-vis/cache"
	"github.com/laindream/go-callflow-vis/config"
	"github.com/laindream/go-callflow-vis/ir"
	"github.com/laindream/go-callflow-vis/log"
	"github.com/laindream/go-callflow-vis/render"
	"github.com/laindream/go-callflow-vis/util"
	"strconv"
)

func NewFlow(config *config.Config, callGraph *ir.Callgraph, fastMode bool) (*Flow, error) {
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
		fastMode:  fastMode,
	}
	f.CheckFlowEntities()
	log.GetLogger().Debugf("NewFlow: Generate Min Graph...")
	err := f.UpdateMinGraph()
	if err != nil {
		return nil, err
	}
	log.GetLogger().Debugf("NewFlow: Generate Min Graph Nodes:%d", len(f.callgraph.Nodes))
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
	fastMode           bool
}

func (f *Flow) CheckFlowEntities() {
	for _, l := range f.Layers {
		for _, e := range l.Entities {
			if len(e.GetNodeSet(f.callgraph)) == 0 {
				log.GetLogger().Warnf("No Node Matched for Entity:%s", e.String())
			}
		}
	}
	f.resetLayer()
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
		if n.GetFunc() == nil {
			continue
		}
		funcNameSet[n.GetFunc().GetName()] = true
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
	f.PrintOriginalFlow()
	err := f.findAllBipartite(false)
	if err != nil {
		f.reset()
		return err
	}
	log.GetLogger().Debugf("Flow.Generate: Done")
	f.PrintFlow()
	f.isCompleteGenerate = true
	return nil
}

func (f *Flow) PrintOriginalFlow() {
	for i, _ := range f.Layers {
		inAllNode := len(f.Layers[i].GetInAllNodeSet(f.callgraph))
		outAllNode := len(f.Layers[i].GetOutAllNodeSet(f.callgraph))
		log.GetLogger().Debugf("Orignial Layers[%d] InAllNode:%d, OutAllNode:%d",
			i, inAllNode, outAllNode)
	}
	f.resetLayer()
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
				t += fmt.Sprintf("%s%s%s%s%s", "\""+util.Escape(from.GetFunc().GetName())+"\"", separator, "\""+util.Escape(to.GetFunc().GetName())+"\"", separator, "\""+util.Escape(path)+"\"")
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
					nodes[e.GetCaller().GetFunc().GetAddr()] = e.GetCaller().ToRenderNode(f.pkgPrefix)
					nodes[e.GetCallee().GetFunc().GetAddr()] = e.GetCallee().ToRenderNode(f.pkgPrefix)
				}
			}
		}
		for k, _ := range l.GetInToOutEdgeSet(f.callgraph) {
			for k2, _ := range l.GetInToOutEdgeSet(f.callgraph)[k] {
				for _, e := range l.GetInToOutEdgeSet(f.callgraph)[k][k2] {
					for _, e2 := range e {
						if e2.GetCallee() == nil && e2.GetCaller() == nil {
							continue
						}
						if e2.GetCallee() == nil && e2.GetCaller() != nil {
							nodes[e2.GetCaller().GetFunc().GetAddr()] = e2.GetCaller().ToRenderNode(f.pkgPrefix)
							continue
						}
						if e2.GetCallee() != nil && e2.GetCaller() == nil {
							nodes[e2.GetCallee().GetFunc().GetAddr()] = e2.GetCallee().ToRenderNode(f.pkgPrefix)
							continue
						}
						edges[e2.String()] = e2.ToRenderEdge(f.pkgPrefix)
						nodes[e2.GetCaller().GetFunc().GetAddr()] = e2.GetCaller().ToRenderNode(f.pkgPrefix)
						nodes[e2.GetCallee().GetFunc().GetAddr()] = e2.GetCallee().ToRenderNode(f.pkgPrefix)
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
				nodes[n.GetFunc().GetAddr()] = n.ToRenderNode(f.pkgPrefix)
				nodes[n.GetFunc().GetAddr()].Set = setIndex
			}
			setIndex++
		}
		if len(layerNodeSet) > 0 {
			for n, _ := range layerNodeSet {
				nodes[n.GetFunc().GetAddr()] = n.ToRenderNode(f.pkgPrefix)
				nodes[n.GetFunc().GetAddr()].Set = setIndex
			}
			setIndex++
		}
		if len(layerOutSet) > 0 {
			for n, _ := range layerOutSet {
				nodes[n.GetFunc().GetAddr()] = n.ToRenderNode(f.pkgPrefix)
				nodes[n.GetFunc().GetAddr()].Set = setIndex
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
					simpleEdge := util.GetSimpleEdgeForPath(l.ExamplePath[k][k2])
					if simpleEdge == nil {
						continue
					}
					edges[simpleEdge.String()] = simpleEdge
					nodes[simpleEdge.GetCaller().GetFunc().GetAddr()] = simpleEdge.GetCaller()
					nodes[simpleEdge.GetCallee().GetFunc().GetAddr()] = simpleEdge.GetCallee()
					continue
				}
				for _, e := range l.ExamplePath[k][k2] {
					edges[e.String()] = e
					nodes[e.GetCaller().GetFunc().GetAddr()] = e.GetCaller()
					nodes[e.GetCallee().GetFunc().GetAddr()] = e.GetCallee()
				}
			}
		}
		for k, _ := range l.GetInToOutEdgeSet(f.callgraph) {
			for k2, _ := range l.GetInToOutEdgeSet(f.callgraph)[k] {
				for _, e := range l.GetInToOutEdgeSet(f.callgraph)[k][k2] {
					for _, e2 := range e {
						if e2.GetCallee() == nil && e2.GetCaller() == nil {
							continue
						}
						if e2.GetCallee() == nil && e2.GetCaller() != nil {
							nodes[e2.GetCaller().GetFunc().GetAddr()] = e2.GetCaller()
							continue
						}
						if e2.GetCallee() != nil && e2.GetCaller() == nil {
							nodes[e2.GetCallee().GetFunc().GetAddr()] = e2.GetCallee()
							continue
						}
						edges[e2.String()] = e2
						nodes[e2.GetCaller().GetFunc().GetAddr()] = e2.GetCaller()
						nodes[e2.GetCallee().GetFunc().GetAddr()] = e2.GetCallee()
					}
				}
			}
		}
	}
	for _, n := range nodes {
		if n.GetFunc() == nil {
			continue
		}
		mainGraph.AddNode(graphName, strconv.Itoa(n.GetID()), map[string]string{
			"label": "\"" + util.Escape(util.GetFuncSimpleName(n.GetFunc().GetName(), f.pkgPrefix)) + "\"",
		})
	}
	for _, e := range edges {
		if e.GetCallee() == nil || e.GetCaller() == nil {
			continue
		}
		callerID := strconv.Itoa(e.GetCaller().GetID())
		calleeID := strconv.Itoa(e.GetCallee().GetID())
		mainGraph.AddEdge(callerID, calleeID, true, map[string]string{
			"label": "\"" + util.Escape(util.GetSiteSimpleName(e.GetSite().GetName(), f.pkgPrefix)) + "\"",
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
				nodeID := strconv.Itoa(n.GetID())
				mainGraph.AddNode(subGraphName, nodeID, map[string]string{
					"label": "\"" + util.Escape(util.GetFuncSimpleName(n.GetFunc().GetName(), f.pkgPrefix)) + "\"",
					"color": "green",
				})
			}
		}
		if len(layerNodeSet) > 0 {
			subGraphName := fmt.Sprintf("cluster_%d", i)
			mainGraph.AddSubGraph(graphName, subGraphName, attr)
			for n, _ := range layerNodeSet {
				nodeID := strconv.Itoa(n.GetID())
				mainGraph.AddNode(subGraphName, nodeID, map[string]string{
					"label": "\"" + util.Escape(util.GetFuncSimpleName(n.GetFunc().GetName(), f.pkgPrefix)) + "\"",
					"color": "red",
				})
			}
		}
		if len(layerOutSet) > 0 {
			subGraphName := fmt.Sprintf("\"cluster_%d-out\"", i)
			mainGraph.AddSubGraph(graphName, subGraphName, attr)
			for n, _ := range layerOutSet {
				nodeID := strconv.Itoa(n.GetID())
				mainGraph.AddNode(subGraphName, nodeID, map[string]string{
					"label": "\"" + util.Escape(util.GetFuncSimpleName(n.GetFunc().GetName(), f.pkgPrefix)) + "\"",
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
		e.ResetEntityData()
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

func (e *Entity) String() string {
	eStr, _ := json.Marshal(e.Entity)
	return string(eStr)
}

func (e *Entity) GetInToOutEdgeSet(callgraphIR *ir.Callgraph) map[*ir.Node]map[*ir.Node][][]*ir.Edge {
	inToOutSet := make(map[*ir.Node]map[*ir.Node][][]*ir.Edge)
	if !e.IsCheckIn() && !e.IsCheckOut() {
		for k, _ := range e.GetNodeSet(callgraphIR) {
			oneNodeEdge := &ir.Edge{Caller: k}
			oneNodeEdgeSet := make(map[*ir.Node][][]*ir.Edge)
			oneNodeEdgeSet[k] = append(make([][]*ir.Edge, 0), append(make([]*ir.Edge, 0), oneNodeEdge))
			inToOutSet[k] = oneNodeEdgeSet
		}
		return inToOutSet
	}
	if !e.IsCheckIn() && e.IsCheckOut() {
		for k, _ := range e.OutNodeSet {
			for _, eOut := range k.GetIn() {
				if eOut.GetCaller() != nil && e.GetNodeSet(callgraphIR)[eOut.GetCaller()] {
					nodeToOutEdge := &ir.Edge{Caller: eOut.GetCaller(), Callee: k, Site: eOut.GetSite()}
					if _, ok := inToOutSet[eOut.GetCaller()]; !ok {
						inToOutSet[eOut.GetCaller()] = make(map[*ir.Node][][]*ir.Edge)
					}
					inToOutSet[eOut.GetCaller()][k] = append(make([][]*ir.Edge, 0), append(make([]*ir.Edge, 0), nodeToOutEdge))
				}
			}
		}
	}
	if e.IsCheckIn() && !e.IsCheckOut() {
		for k, _ := range e.InNodeSet {
			for _, eIn := range k.GetOut() {
				if eIn.GetCallee() != nil && e.GetNodeSet(callgraphIR)[eIn.GetCallee()] {
					inToNodeEdge := &ir.Edge{Caller: k, Callee: eIn.GetCallee(), Site: eIn.GetSite()}
					if _, ok := inToOutSet[k]; !ok {
						inToOutSet[k] = make(map[*ir.Node][][]*ir.Edge)
					}
					inToOutSet[k][eIn.GetCallee()] = append(make([][]*ir.Edge, 0), append(make([]*ir.Edge, 0), inToNodeEdge))
				}
			}
		}
	}
	if e.IsCheckIn() && e.IsCheckOut() {
		for k, _ := range e.OutNodeSet {
			for _, eOut := range k.GetIn() {
				if eOut.GetCaller() != nil && e.GetNodeSet(callgraphIR)[eOut.GetCaller()] {
					node := eOut.GetCaller()
					for _, eIn := range node.GetIn() {
						if eIn.GetCaller() != nil && e.InNodeSet[eIn.GetCaller()] {
							inToNodeEdge := &ir.Edge{Caller: eIn.GetCaller(), Callee: node, Site: eIn.GetSite()}
							nodeToOutEdge := &ir.Edge{Caller: node, Callee: k, Site: eOut.GetSite()}
							if _, ok := inToOutSet[eIn.GetCaller()]; !ok {
								inToOutSet[eIn.GetCaller()] = make(map[*ir.Node][][]*ir.Edge)
							}
							if _, ok := inToOutSet[eIn.GetCaller()][k]; !ok {
								inToOutSet[eIn.GetCaller()][k] = make([][]*ir.Edge, 0)
							}
							path := append(make([]*ir.Edge, 0), inToNodeEdge, nodeToOutEdge)
							inToOutSet[eIn.GetCaller()][k] = append(inToOutSet[eIn.GetCaller()][k], path)
						}
					}
				}
			}
		}
	}
	return inToOutSet
}

func (e *Entity) ResetEntityData() {
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
	for _, n := range callgraphIR.Nodes {
		if e.ShouldNodePass(n) {
			nodesSet[n] = true
		}
	}
	e.NodeSet = nodesSet
	return e.NodeSet
}

func (e *Entity) TrimNodeSet(nodeSet map[*ir.Node]bool, callgraphIR *ir.Callgraph) {
	for n, _ := range e.GetNodeSet(callgraphIR) {
		if _, ok := nodeSet[n]; !ok {
			delete(e.NodeSet, n)
		}
	}
}

func (e *Entity) TrimInNodeSet(in map[*ir.Node]bool, callgraphIR *ir.Callgraph) {
	if e.IsCheckIn() {
		if e.InNodeSet == nil {
			e.UpdateInNodeSetWithNodeSet(callgraphIR)
		}
		for n, _ := range e.InNodeSet {
			if _, ok := in[n]; !ok {
				delete(e.InNodeSet, n)
			}
		}
		e.UpdateNodeSetWithInNodeSet(callgraphIR)
		return
	}
	e.TrimNodeSet(in, callgraphIR)
}

func (e *Entity) TrimOutNodeSet(out map[*ir.Node]bool, callgraphIR *ir.Callgraph) {
	if e.IsCheckOut() {
		if e.OutNodeSet == nil {
			e.UpdateOutNodeSetWithNodeSet(callgraphIR)
		}
		for n, _ := range e.OutNodeSet {
			if _, ok := out[n]; !ok {
				delete(e.OutNodeSet, n)
			}
		}
		e.UpdateNodeSetWithOutNodeSet(callgraphIR)
		return
	}
	e.TrimNodeSet(out, callgraphIR)
}

func (e *Entity) GetInAllNodeSet(callgraphIR *ir.Callgraph) map[*ir.Node]bool {
	if callgraphIR == nil {
		return nil
	}
	if !e.IsCheckIn() {
		return e.GetNodeSet(callgraphIR)
	}
	if e.InNodeSet == nil {
		e.UpdateInNodeSetWithNodeSet(callgraphIR)
	}
	return e.InNodeSet
}

func (e *Entity) GetInAllNodeSetOnlyRead(callgraphIR *ir.Callgraph) map[*ir.Node]bool {
	if callgraphIR == nil {
		return nil
	}
	if !e.IsCheckIn() {
		return e.NodeSet
	}
	return e.InNodeSet
}

func (e *Entity) GetOutAllNodeSet(callgraphIR *ir.Callgraph) map[*ir.Node]bool {
	if callgraphIR == nil {
		return nil
	}
	if !e.IsCheckOut() {
		return e.GetNodeSet(callgraphIR)
	}
	if e.OutNodeSet == nil {
		e.UpdateOutNodeSetWithNodeSet(callgraphIR)
	}
	return e.OutNodeSet
}

func (e *Entity) GetOutAllNodeSetOnlyRead(callgraphIR *ir.Callgraph) map[*ir.Node]bool {
	if callgraphIR == nil {
		return nil
	}
	if !e.IsCheckOut() {
		return e.NodeSet
	}
	return e.OutNodeSet
}

func (e *Entity) UpdateNodeSetWithInNodeSet(callgraphIR *ir.Callgraph) {
	if callgraphIR == nil {
		return
	}
	if !e.IsCheckIn() {
		e.GetNodeSet(callgraphIR)
		return
	}
	if e.InNodeSet == nil {
		e.UpdateInNodeSetWithNodeSet(callgraphIR)
	}
	nodeSet := make(map[*ir.Node]bool)
	for n, _ := range e.InNodeSet {
		for _, eOut := range n.GetOut() {
			if e.ShouldInPass(eOut) && e.ShouldNodeSanityPass(eOut.GetCallee()) {
				nodeSet[eOut.GetCallee()] = true
			}
		}
	}
	e.NodeSet = nodeSet
}

func (e *Entity) UpdateNodeSetWithOutNodeSet(callgraphIR *ir.Callgraph) {
	if callgraphIR == nil {
		return
	}
	if !e.IsCheckOut() {
		e.GetNodeSet(callgraphIR)
		return
	}
	if e.OutNodeSet == nil {
		e.UpdateOutNodeSetWithNodeSet(callgraphIR)
	}
	nodeSet := make(map[*ir.Node]bool)
	for n, _ := range e.OutNodeSet {
		for _, eIn := range n.GetIn() {
			if e.ShouldOutPass(eIn) && e.ShouldNodeSanityPass(eIn.GetCaller()) {
				nodeSet[eIn.GetCaller()] = true
			}
		}
	}
	e.NodeSet = nodeSet
}

func (e *Entity) UpdateInNodeSetWithNodeSet(callgraphIR *ir.Callgraph) map[*ir.Node]bool {
	if callgraphIR == nil {
		return nil
	}
	if !e.IsCheckIn() {
		return make(map[*ir.Node]bool)
	}
	nodesSet := make(map[*ir.Node]bool)
	for v, _ := range e.GetNodeSet(callgraphIR) {
		for _, eIn := range v.GetIn() {
			if e.ShouldInPass(eIn) {
				nodesSet[eIn.GetCaller()] = true
			}
		}
	}
	e.InNodeSet = nodesSet
	return nodesSet
}

func (e *Entity) UpdateOutNodeSetWithNodeSet(callgraphIR *ir.Callgraph) map[*ir.Node]bool {
	if callgraphIR == nil {
		return nil
	}
	if !e.IsCheckOut() {
		return make(map[*ir.Node]bool)
	}
	nodesSet := make(map[*ir.Node]bool)
	for v, _ := range e.GetNodeSet(callgraphIR) {
		for _, eOut := range v.GetOut() {
			if e.ShouldOutPass(eOut) {
				nodesSet[eOut.GetCallee()] = true
			}
		}
	}
	e.OutNodeSet = nodesSet
	return nodesSet
}

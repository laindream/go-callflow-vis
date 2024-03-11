package analysis

import (
	"errors"
	"fmt"
	"github.com/laindream/go-callflow-vis/cache"
	"github.com/laindream/go-callflow-vis/config"
	"github.com/laindream/go-callflow-vis/flow"
	"github.com/laindream/go-callflow-vis/ir"
	"github.com/laindream/go-callflow-vis/util"
	"golang.org/x/tools/go/callgraph"
)

type CallgraphType string

const (
	CallGraphTypeStatic CallgraphType = "static"
	CallGraphTypeCha                  = "cha"
	CallGraphTypeRta                  = "rta"
	CallGraphTypePta                  = "pta"
	CallGraphTypeVta                  = "vta"
)

type Analysis struct {
	*ProgramAnalysisParam
	callgraph *ir.Callgraph
	config    *config.Config
	flow      *flow.Flow
	cachePath string
	cache     *cache.FileCache
}

type ProgramAnalysisParam struct {
	Algo  CallgraphType `json:"algo"`
	Tests bool          `json:"tests"`
	Dir   string        `json:"dir"`
	Args  []string      `json:"args"`
	Build []string      `json:"build"`
}

func NewAnalysis(
	config *config.Config,
	cachePath string,
	algo CallgraphType,
	tests bool,
	dir string,
	args []string,
	build []string) *Analysis {
	if config == nil {
		fmt.Printf("Analysis.NewAnalysis: config is nil\n")
		return nil
	}
	return &Analysis{
		config: config,
		cache:  cache.NewFileCache(cachePath),
		ProgramAnalysisParam: &ProgramAnalysisParam{
			Algo:  algo,
			Tests: tests,
			Dir:   dir,
			Args:  args,
			Build: build,
		},
	}
}

func (a *Analysis) Run() error {
	filterCacheKey := fmt.Sprintf("%s_filter_%s", a.GetCacheKeyPrefix(), util.GetHash(a.config))
	var filterCacheObj *ir.Callgraph
	err := a.cache.Get(filterCacheKey, &filterCacheObj)
	if err == nil && filterCacheObj != nil {
		fmt.Printf("Analysis.FilterCallGraph: cache hit: %s\n", filterCacheKey)
		a.callgraph = filterCacheObj
	}
	if a.callgraph == nil {
		err = a.InitCallgraph()
		if err != nil {
			return err
		}
		a.FilterCallGraph()
	}
	err = a.GenerateFlow()
	if err != nil {
		return err
	}
	return nil
}

func (a *Analysis) InitCallgraph() error {
	cacheKey := a.GetCacheKeyPrefix()
	var cacheObj *ir.Callgraph
	err := a.cache.Get(cacheKey, &cacheObj)
	if err == nil && cacheObj != nil {
		fmt.Printf("Analysis.InitCallgraph: cache hit: %s\n", cacheKey)
		a.callgraph = cacheObj
		return nil
	}
	programAnalysis, err := RunAnalysis(a.Tests, a.Build, a.Args, a.Dir)
	if err != nil {
		return err
	}
	var cg *callgraph.Graph
	cg, err = a.ComputeCallgraph(programAnalysis)
	if err != nil {
		return err
	}
	cg.DeleteSyntheticNodes()
	a.callgraph = ir.ConvertToIR(cg)
	err = a.cache.Set(cacheKey, a.callgraph)
	if err != nil {
		fmt.Printf("Analysis.InitCallgraph cache set error: %v\n", err)
	}
	return nil
}

func (a *Analysis) GetCacheKeyPrefix() string {
	programAnalysisParamHash := util.GetHash(a.ProgramAnalysisParam)
	return fmt.Sprintf("%s_%s", "callgraph", programAnalysisParamHash)
}

func (a *Analysis) GenerateFlow() error {
	if a.callgraph == nil {
		return errors.New("callgraph is nil")
	}
	if a.config == nil {
		return errors.New("config is nil")
	}
	f, err := flow.NewFlow(a.config, a.callgraph)
	if err != nil {
		return err
	}
	err = f.Generate()
	if err != nil {
		return err
	}
	a.flow = f
	return nil
}

func (a *Analysis) GetFlow() *flow.Flow {
	return a.flow
}

func (a *Analysis) FilterCallGraph() {
	cacheKey := fmt.Sprintf("%s_filter_%s", a.GetCacheKeyPrefix(), util.GetHash(a.config))
	var cacheObj *ir.Callgraph
	err := a.cache.Get(cacheKey, &cacheObj)
	if err == nil && cacheObj != nil {
		fmt.Printf("Analysis.FilterCallGraph: cache hit: %s\n", cacheKey)
		a.callgraph = cacheObj
		return
	}
	if a.callgraph == nil {
		fmt.Printf("Analysis.FilterCallGraph: callgraph is nil\n")
		return
	}
	a.callgraph = ir.GetFilteredCallgraph(a.callgraph, func(funcName string) bool {
		if !a.config.Focus.Match(funcName) ||
			a.config.Ignore.Match(funcName) {
			return false
		}
		return true
	})
	err = a.cache.Set(cacheKey, a.callgraph)
	if err != nil {
		fmt.Printf("Analysis.FilterCallGraph cache set error: %v\n", err)
	}
}

func (a *Analysis) TrimCallGraph() {
	cacheKey := fmt.Sprintf("%s_trim_%s", a.GetCacheKeyPrefix(), util.GetHash(a.config))
	var cacheObj *ir.Callgraph
	err := a.cache.Get(cacheKey, &cacheObj)
	if err == nil && cacheObj != nil {
		fmt.Printf("Analysis.TrimCallGraph: cache hit: %s\n", cacheKey)
		a.callgraph = cacheObj
		return
	}
	if a.callgraph == nil {
		fmt.Printf("Analysis.TrimCallGraph: callgraph is nil\n")
		return
	}
	nodesToTrim := make(map[*ir.Node]bool)
	for i, _ := range a.callgraph.Nodes {
		if a.callgraph.Nodes[i] != nil && a.callgraph.Nodes[i].Func != nil &&
			(!a.config.Focus.Match(a.callgraph.Nodes[i].Func.Name) ||
				a.config.Ignore.Match(a.callgraph.Nodes[i].Func.Name)) {
			nodesToTrim[a.callgraph.Nodes[i]] = true
		}
	}
	for i, _ := range nodesToTrim {
		a.callgraph.DeleteNode(i)
	}
	err = a.cache.Set(cacheKey, a.callgraph)
	if err != nil {
		fmt.Printf("Analysis.TrimCallGraph cache set error: %v\n", err)
	}
}

package analysis

import (
	"errors"
	"fmt"
	"github.com/laindream/go-callflow-vis/cache"
	"github.com/laindream/go-callflow-vis/config"
	"github.com/laindream/go-callflow-vis/flow"
	"github.com/laindream/go-callflow-vis/ir"
	"github.com/laindream/go-callflow-vis/log"
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
	fastMode  bool
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
	build []string,
	fastMode bool) *Analysis {
	if config == nil {
		fmt.Printf("Analysis.NewAnalysis: config is nil\n")
		return nil
	}
	return &Analysis{
		config:   config,
		cache:    cache.NewFileCache(cachePath),
		fastMode: fastMode,
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
		log.GetLogger().Debugf("Analysis.Run: cache hit: %s", filterCacheKey)
		a.callgraph = filterCacheObj
	}
	if a.callgraph == nil {
		err = a.InitCallgraph()
		if err != nil {
			log.GetLogger().Errorf("Analysis.Run: init callgraph error: %v", err)
			return err
		}
		err = a.FilterCallGraph()
		if err != nil {
			log.GetLogger().Errorf("Analysis.Run: filter callgraph error: %v", err)
			return err
		}
	}
	err = a.GenerateFlow()
	if err != nil {
		log.GetLogger().Errorf("Analysis.Run: generate flow error: %v", err)
		return err
	}
	return nil
}

func (a *Analysis) InitCallgraph() error {
	cacheKey := a.GetCacheKeyPrefix()
	var cacheObj *ir.Callgraph
	err := a.cache.Get(cacheKey, &cacheObj)
	if err == nil && cacheObj != nil {
		log.GetLogger().Debugf("Analysis.InitCallgraph: cache hit: %s", cacheKey)
		a.callgraph = cacheObj
		return nil
	}
	log.GetLogger().Debugf("Analysis.InitCallgraph: Program Analysis Start...")
	programAnalysis, err := RunAnalysis(a.Tests, a.Build, a.Args, a.Dir)
	if err != nil {
		log.GetLogger().Errorf("Analysis.InitCallgraph: program analysis error: %v", err)
		return err
	}
	var cg *callgraph.Graph
	log.GetLogger().Debugf("Analysis.InitCallgraph: Callgraph Compute Start(algo:%s)...", a.Algo)
	cg, err = a.ComputeCallgraph(programAnalysis)
	if err != nil {
		log.GetLogger().Errorf("Analysis.InitCallgraph: callgraph compute error: %v", err)
		return err
	}
	log.GetLogger().Debugf("Analysis.InitCallgraph: Callgraph Convert Start...")
	cg.DeleteSyntheticNodes()
	a.callgraph = ir.ConvertToIR(cg)
	log.GetLogger().Debugf("Analysis.InitCallgraph: cache set: %s", cacheKey)
	err = a.cache.Set(cacheKey, a.callgraph)
	if err != nil {
		log.GetLogger().Errorf("Analysis.InitCallgraph cache set error: %v", err)
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
	f, err := flow.NewFlow(a.config, a.callgraph, a.fastMode)
	if err != nil {
		log.GetLogger().Errorf("Analysis.GenerateFlow: flow new error: %v", err)
		return err
	}
	err = f.Generate()
	if err != nil {
		log.GetLogger().Errorf("Analysis.GenerateFlow: flow generate error: %v", err)
		return err
	}
	a.flow = f
	return nil
}

func (a *Analysis) GetFlow() *flow.Flow {
	return a.flow
}

func (a *Analysis) FilterCallGraph() error {
	cacheKey := fmt.Sprintf("%s_filter_%s", a.GetCacheKeyPrefix(), util.GetHash(a.config))
	var cacheObj *ir.Callgraph
	err := a.cache.Get(cacheKey, &cacheObj)
	if err == nil && cacheObj != nil {
		log.GetLogger().Debugf("Analysis.FilterCallGraph: cache hit: %s", cacheKey)
		a.callgraph = cacheObj
		return nil
	}
	if a.callgraph == nil {
		return errors.New("callgraph is nil")
	}
	log.GetLogger().Debugf("Analysis.FilterCallGraph: Filter Callgraph Start...")
	a.callgraph = ir.GetFilteredCallgraph(a.callgraph, func(funcName string) bool {
		if (len(a.config.Focus) != 0 && !a.config.Focus.Match(funcName)) ||
			(len(a.config.Ignore) != 0 && a.config.Ignore.Match(funcName)) {
			return false
		}
		return true
	})
	log.GetLogger().Debugf("Analysis.FilterCallGraph: cache set: %s", cacheKey)
	err = a.cache.Set(cacheKey, a.callgraph)
	if err != nil {
		log.GetLogger().Errorf("Analysis.FilterCallGraph cache set error: %v", err)
	}
	return nil
}

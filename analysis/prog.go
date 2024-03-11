package analysis

import (
	"fmt"
	"go/build"
	"golang.org/x/tools/go/callgraph/vta"
	"strings"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/callgraph/static"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

type ProgramAnalysis struct {
	Prog  *ssa.Program
	Pkgs  []*ssa.Package
	Mains []*ssa.Package
}

const pkgLoadMode = packages.NeedName |
	packages.NeedFiles |
	packages.NeedCompiledGoFiles |
	packages.NeedImports |
	packages.NeedDeps |
	packages.NeedExportsFile |
	packages.NeedTypes |
	packages.NeedSyntax |
	packages.NeedTypesInfo |
	packages.NeedTypesSizes |
	packages.NeedModule

func getBuildFlags() []string {
	buildFlagTags := getBuildFlagTags(build.Default.BuildTags)
	if len(buildFlagTags) == 0 {
		return nil
	}

	return []string{buildFlagTags}
}

func getBuildFlagTags(buildTags []string) string {
	if len(buildTags) > 0 {
		return "-tags=" + strings.Join(buildTags, ",")
	}
	return ""
}

func RunAnalysis(withTests bool, buildFlags []string, pkgPatterns []string, queryDir string) (*ProgramAnalysis, error) {
	if len(pkgPatterns) == 0 {
		return nil, fmt.Errorf("no package patterns provided")
	}
	cfg := &packages.Config{
		Mode:       packages.LoadAllSyntax,
		Tests:      withTests,
		BuildFlags: getBuildFlags(),
		Dir:        queryDir,
	}
	//if gopath != "" {
	//	cfg.Env = append(os.Environ(), "GOPATH="+gopath) // to enable testing
	//}
	initial, err := packages.Load(cfg, pkgPatterns...)
	if err != nil {
		return nil, fmt.Errorf("loading packages: %v", err)
	}
	if packages.PrintErrors(initial) > 0 {
		return nil, fmt.Errorf("packages contain errors")
	}

	// Create and build SSA-form program representation.
	mode := ssa.InstantiateGenerics // instantiate generics by default for soundness
	prog, pkgs := ssautil.AllPackages(initial, mode)
	prog.Build()

	return &ProgramAnalysis{
		Prog: prog,
		Pkgs: pkgs,
		//Mains: mains,
	}, nil
}

func (a *Analysis) ComputeCallgraph(data *ProgramAnalysis) (*callgraph.Graph, error) {
	switch a.Algo {
	case CallGraphTypeStatic:
		return static.CallGraph(data.Prog), nil
	case CallGraphTypeCha:
		return cha.CallGraph(data.Prog), nil
	case CallGraphTypePta:
		return nil, fmt.Errorf("pointer analysis is no longer supported (see Go issue #59676)")
	case CallGraphTypeRta:
		mains, err := mainPackages(data.Pkgs)
		if err != nil {
			return nil, err
		}
		var roots []*ssa.Function
		for _, main := range mains {
			roots = append(roots, main.Func("init"), main.Func("main"))
		}
		return rta.Analyze(roots, true).CallGraph, nil
	case CallGraphTypeVta:
		return vta.CallGraph(ssautil.AllFunctions(data.Prog), cha.CallGraph(data.Prog)), nil
	default:
		return nil, fmt.Errorf("unknown callgraph type: %v", a.Algo)
	}
}

func mainPackages(pkgs []*ssa.Package) ([]*ssa.Package, error) {
	var mains []*ssa.Package
	for _, p := range pkgs {
		if p != nil && p.Pkg.Name() == "main" && p.Func("main") != nil {
			mains = append(mains, p)
		}
	}
	if len(mains) == 0 {
		return nil, fmt.Errorf("no main packages")
	}
	return mains, nil
}

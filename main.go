package main

import (
	"flag"
	"fmt"
	"github.com/laindream/go-callflow-vis/analysis"
	"github.com/laindream/go-callflow-vis/config"
	"os"
	"strings"
)

const Usage = `usage...
`

var (
	webFlag        = flag.Bool("web", false, "Output an index.html with graph data embedded instead of raw JSON")
	testFlag       = flag.Bool("tests", false, "Consider tests files as entry points for call-graph")
	goRootFlag     = flag.Bool("go-root", false, "Include packages part of the Go root")
	unexportedFlag = flag.Bool("unexported", false, "Include unexported function calls")
	queryDir       = flag.String("query-dir", "", "Directory to query from for go packages. Current dir if empty")
	cacheDir       = flag.String("cache-dir", "", "Directory to store cache files")
	configPath     = flag.String("config", "", "Path to the layer configuration file (e.g., config.toml)")
	callgraphAlgo  = flag.String("algo", analysis.CallGraphTypeCha, fmt.Sprintf("The algorithm used to construct the call graph. Possible values inlcude: %q, %q, %q, %q, %q",
		analysis.CallGraphTypeStatic, analysis.CallGraphTypeCha, analysis.CallGraphTypeRta, analysis.CallGraphTypePta, analysis.CallGraphTypeVta))
	buildFlag = flag.String("build", "", "Build flags to pass to Go build tool. Separated with spaces")
	outDir    = flag.String("out-dir", ".", "Output directory for the generated files")
)

func main() {
	flag.Parse()

	args := flag.Args()

	if flag.NArg() == 0 {
		_, _ = fmt.Fprintf(os.Stderr, Usage)
		flag.PrintDefaults()
		os.Exit(2)
	}
	var buildFlags []string
	if len(*buildFlag) > 0 {
		buildFlags = strings.Split(*buildFlag, " ")
	}
	conf, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("failed to load config: %v\n", err)
		os.Exit(1)
	}
	a := analysis.NewAnalysis(
		conf,
		*cacheDir,
		analysis.CallgraphType(*callgraphAlgo),
		*testFlag,
		*queryDir,
		args,
		buildFlags,
	)
	err = a.Run()
	if err != nil {
		fmt.Printf("failed to run analysis: %v\n", err)
		os.Exit(1)
	}
	f := a.GetFlow()
	if f == nil {
		fmt.Printf("get flow failed\n")
		os.Exit(1)
	}
	out := *outDir
	if strings.HasSuffix(out, "/") {
		out = out[:len(out)-1]
	}
	err = f.SavePaths(fmt.Sprintf("%s/path_out", out), "")
	if err != nil {
		fmt.Printf("failed to save paths: %v\n", err)
	}
	err = f.SaveGraph(fmt.Sprintf("%s/graph_out/simple_callgraph.dot", out), true)
	if err != nil {
		fmt.Printf("failed to save simple graph: %v\n", err)
	}
	err = f.SaveGraph(fmt.Sprintf("%s/graph_out/complete_callgraph.dot", out), false)
	if err != nil {
		fmt.Printf("failed to save complete graph: %v\n", err)
	}
}

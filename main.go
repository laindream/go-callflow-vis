package main

import (
	"embed"
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/laindream/go-callflow-vis/analysis"
	"github.com/laindream/go-callflow-vis/config"
	"github.com/laindream/go-callflow-vis/log"
	"github.com/pkg/browser"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"time"
)

const Usage = `Usage: go-callflow-vis [OPTIONS] PACKAGE...
Examples: (When you are in the root directory of the project)
    go-callflow-vis -config ./config.toml .
Options:
    -config <config_file> (Required): Path to the layer configuration file (e.g., config.toml).
    -cache-dir <cache_dir> (Optional, Default: ./go_callflow_vis_cache): Directory to store cache files.
    -out-dir <out_dir> (Optional, Default: .): Output directory for the generated files.
    -algo <algo> (Optional, Default: cha): The algorithm used to construct the call graph. Possible values include: static, cha, rta, pta, vta.
    -fast (Optional): Use fast mode to generate flow, which may lose some connectivity.
    -query-dir <query_dir> (Optional, Default: ""): Directory to query from for Go packages. Uses the current directory if empty.
    -tests (Optional): Consider test files as entry points for the call graph.
    -build <build_flags> (Optional, Default: ""): Build flags to pass to the Go build tool. Flags should be separated with spaces.
    -skip-browser (Optional): Skip opening the browser automatically.
    -web (Optional): Serve the web visualization interface.
    -web-host <web_host> (Optional, Default: localhost): Host to serve the web interface on.
    -web-port <web_port> (Optional, Default: 45789): Port to serve the web interface on.
    -debug (Optional): Print debug information.
Arguments:
    PACKAGE...: One or more Go packages to analyze.

`

var (
	configPath    = flag.String("config", "", "(Required)Path to the layer configuration file (e.g., config.toml)")
	cacheDir      = flag.String("cache-dir", "", "Directory to store cache files")
	outDir        = flag.String("out-dir", ".", "Output directory for the generated files")
	callgraphAlgo = flag.String("algo", analysis.CallGraphTypeCha, fmt.Sprintf("The algorithm used to construct the call graph. Possible values inlcude: %q, %q, %q, %q, %q",
		analysis.CallGraphTypeStatic, analysis.CallGraphTypeCha, analysis.CallGraphTypeRta, analysis.CallGraphTypePta, analysis.CallGraphTypeVta))
	fastFlag    = flag.Bool("fast", false, "Use fast mode to generate flow, which may lose some connectivity")
	queryDir    = flag.String("query-dir", "", "Directory to query from for go packages. Current dir if empty")
	testFlag    = flag.Bool("tests", false, "Consider tests files as entry points for call-graph")
	buildFlag   = flag.String("build", "", "Build flags to pass to Go build tool. Separated with spaces")
	skipBrowser = flag.Bool("skip-browser", false, "Skip opening browser")
	webFlag     = flag.Bool("web", false, "Serve web visualisation")
	webHost     = flag.String("web-host", "localhost", "Host to serve the web on")
	webPort     = flag.String("web-port", "45789", "Port to serve the web on")
	debugFlag   = flag.Bool("debug", false, "Print debug information")
)

//go:embed static
var FS embed.FS

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
	if *debugFlag {
		log.SetLogger(*debugFlag)
	}
	conf, err := config.LoadConfig(*configPath)
	if err != nil {
		log.GetLogger().Errorf("failed to load config: %v", err)
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
		*fastFlag,
	)
	err = a.Run()
	if err != nil {
		log.GetLogger().Errorf("failed to run analysis: %v", err)
		os.Exit(1)
	}
	f := a.GetFlow()
	if f == nil {
		log.GetLogger().Errorf("failed to get flow")
		os.Exit(1)
	}
	out := *outDir
	if strings.HasSuffix(out, "/") {
		out = out[:len(out)-1]
	}
	log.GetLogger().Debugf("saving paths to %s", fmt.Sprintf("%s/path_out", out))
	err = f.SavePaths(fmt.Sprintf("%s/path_out", out), "")
	if err != nil {
		log.GetLogger().Errorf("failed to save paths: %v", err)
	}
	log.GetLogger().Debugf("saving simple callgraph to %s", fmt.Sprintf("%s/graph_out/simple_callgraph.dot", out))
	err = f.SaveDot(fmt.Sprintf("%s/graph_out/simple_callgraph.dot", out), true)
	if err != nil {
		log.GetLogger().Errorf("failed to save simple graph: %v", err)
	}
	log.GetLogger().Debugf("saving complete callgraph to %s", fmt.Sprintf("%s/graph_out/complete_callgraph.dot", out))
	err = f.SaveDot(fmt.Sprintf("%s/graph_out/complete_callgraph.dot", out), false)
	if err != nil {
		log.GetLogger().Errorf("failed to save complete graph: %v", err)
	}
	if *webFlag {
		gin.SetMode(gin.ReleaseMode)
		r := gin.Default()
		r.GET("/graph", func(c *gin.Context) {
			graph := f.GetRenderGraph()
			c.JSON(200, graph)
		})
		r.GET("/dot", func(c *gin.Context) {
			graph := f.GetDot(false)
			c.String(200, graph)
		})
		r.GET("/dot_simple", func(c *gin.Context) {
			graph := f.GetDot(true)
			c.String(200, graph)
		})
		renderGraph, _ := fs.Sub(FS, "static/render")
		r.StaticFS("/render/graph", http.FS(renderGraph))
		renderDot, _ := fs.Sub(FS, "static/dot")
		r.StaticFS("/render/dot", http.FS(renderDot))
		renderDotSimple, _ := fs.Sub(FS, "static/dot_simple")
		r.StaticFS("/render/dot_simple", http.FS(renderDotSimple))
		if !*skipBrowser {
			go openBrowser(fmt.Sprintf("http://%s:%s/render/dot/", *webHost, *webPort))
		}
		log.GetLogger().Infof("serving web on %s:%s", *webHost, *webPort)
		r.Run(fmt.Sprintf("%s:%s", *webHost, *webPort))
	}
}

func openBrowser(url string) {
	time.Sleep(time.Millisecond * 100)
	if err := browser.OpenURL(url); err != nil {
		log.GetLogger().Errorf("failed to open browser: %v", err)
	}
}

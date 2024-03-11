package main

import (
	"embed"
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/laindream/go-callflow-vis/analysis"
	"github.com/laindream/go-callflow-vis/config"
	"github.com/pkg/browser"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const Usage = `usage...
`

var (
	webFlag       = flag.Bool("web", false, "Output an index.html with graph data embedded instead of raw JSON")
	webHost       = flag.String("web-host", "localhost", "Host to serve the web interface on")
	webPort       = flag.String("web-port", "45789", "Port to serve the web interface on")
	testFlag      = flag.Bool("tests", false, "Consider tests files as entry points for call-graph")
	queryDir      = flag.String("query-dir", "", "Directory to query from for go packages. Current dir if empty")
	cacheDir      = flag.String("cache-dir", "", "Directory to store cache files")
	configPath    = flag.String("config", "", "Path to the layer configuration file (e.g., config.toml)")
	callgraphAlgo = flag.String("algo", analysis.CallGraphTypeCha, fmt.Sprintf("The algorithm used to construct the call graph. Possible values inlcude: %q, %q, %q, %q, %q",
		analysis.CallGraphTypeStatic, analysis.CallGraphTypeCha, analysis.CallGraphTypeRta, analysis.CallGraphTypePta, analysis.CallGraphTypeVta))
	buildFlag = flag.String("build", "", "Build flags to pass to Go build tool. Separated with spaces")
	outDir    = flag.String("out-dir", ".", "Output directory for the generated files")
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
	err = f.SaveDot(fmt.Sprintf("%s/graph_out/simple_callgraph.dot", out), true)
	if err != nil {
		fmt.Printf("failed to save simple graph: %v\n", err)
	}
	err = f.SaveDot(fmt.Sprintf("%s/graph_out/complete_callgraph.dot", out), false)
	if err != nil {
		fmt.Printf("failed to save complete graph: %v\n", err)
	}
	if *webFlag {
		r := gin.Default()

		r.GET("/graph", func(c *gin.Context) {
			graph := f.GetRenderGraph()
			c.JSON(200, graph)
		})
		r.GET("/dot", func(c *gin.Context) {
			graph := f.GetDot(false)
			c.String(200, graph)
		})

		renderGraph, _ := fs.Sub(FS, "static/render")
		r.StaticFS("/render/graph", http.FS(renderGraph))
		renderDot, _ := fs.Sub(FS, "static/dot")
		r.StaticFS("/render/dot", http.FS(renderDot))
		go openBrowser(fmt.Sprintf("http://%s:%s/render/dot/", *webHost, *webPort))
		r.Run(fmt.Sprintf("%s:%s", *webHost, *webPort))
	}
}

func openBrowser(url string) {
	time.Sleep(time.Millisecond * 100)
	if err := browser.OpenURL(url); err != nil {
		log.Printf("OpenURL error: %v", err)
	}
}

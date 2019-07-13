// gocyto: Go call-graph analysis and visualization
//
package main

import (
	"flag"
	"fmt"
	"github.com/protolambda/gocyto/analysis"
	"github.com/protolambda/gocyto/render"
	"os"
)

var (
	testFlag = flag.Bool("tests", false, "Consider tests files as entry points for call-graph")
)

func main() {
	flag.Parse()

	args := flag.Args()
	if flag.NArg() != 1 {
		_, _ = fmt.Fprintf(os.Stderr, "todo usage msg")
		flag.PrintDefaults()
		os.Exit(2)
	}

	tests := *testFlag

	aProg, err := analysis.RunAnalysis(tests, []string{}, args)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "could not run program analysis: %v", err)
		os.Exit(1)
	}

	callGraph := analysis.PointerAnalysis.LoadCallgraph(aProg)
	cytoGraph := render.NewCytoGraph()

	opts := &render.RenderOptions{
		IncludeGoRoot: false, // TODO flag
		IncludeUnexported: false, // TODO flag
	}

	if err := cytoGraph.LoadCallGraph(callGraph, opts); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "could not call graph: %v", err)
		os.Exit(1)
	}

	// TODO
	// cytoGraph.WriteJson()
}

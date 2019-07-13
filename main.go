// gocyto: Go call-graph analysis and visualization
//
package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/protolambda/gocyto/analysis"
	"github.com/protolambda/gocyto/render"
	"os"
	"strings"
)

var (
	testFlag       = flag.Bool("tests", false, "Consider tests files as entry points for call-graph")
	goRootFlag     = flag.Bool("go-root", false, "Include packages part of the Go root")
	unexportedFlag = flag.Bool("unexported", false, "Include unexported function calls")
	modeFlag       = flag.String("mode", "pointer", "Type of analysis to run. One of: pointer, cha, rta, static")
	buildFlag      = flag.String("build", "", "Build flags to pass to Go build tool. Separated with spaces")
	outFlag        = flag.String("out", "", "Output file, if none is specified, output to std out")
)

const usage = `
Gocyto: Callgraph analysis and visualization for Go. - build by @protolambda

https://github.com/protolambda/gocyto

Usage:

gocyto [options...] <package path(s)>

Options:

`

func main() {
	flag.Parse()

	args := flag.Args()
	if flag.NArg() == 0 {
		_, _ = fmt.Fprintf(os.Stderr, usage)
		flag.PrintDefaults()
		os.Exit(2)
	}

	buildFlags := strings.Split(*buildFlag, " ")

	var mode analysis.AnalysisMode
	switch *modeFlag {
	case "pointer":
		mode = analysis.PointerAnalysis
	case "cha":
		mode = analysis.ClassHierarchyAnalysis
	case "rta":
		mode = analysis.RapidTypeAnalysis
	case "static":
		mode = analysis.StaticAnalysis
	default:
		_, _ = fmt.Fprintf(os.Stderr, "analysis mode not recognized")
		os.Exit(2)
	}

	check := func(err error, msg string) {
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, msg, err)
			os.Exit(1)
		}
	}
	aProg, err := analysis.RunAnalysis(*testFlag, buildFlags, args)
	check(err, "could not run program analysis: %v")

	callGraph := mode.LoadCallgraph(aProg)
	cytoGraph := render.NewCytoGraph()

	opts := &render.RenderOptions{
		IncludeGoRoot:     *goRootFlag,
		IncludeUnexported: *unexportedFlag,
	}

	check(cytoGraph.LoadCallGraph(callGraph, opts), "could not call graph: %v")

	outPath := *outFlag
	if outPath == "" {
		check(cytoGraph.WriteJson(os.Stdout), "could not write graph JSON to std out: %v")
	} else {
		f, err := os.Create(outPath)
		check(err, "could not create file: %v")
		defer f.Close()
		w := bufio.NewWriter(f)

		check(cytoGraph.WriteJson(f), "could not write graph JSON to file: %v")
		check(w.Flush(), "could not flush output to file")
	}
}

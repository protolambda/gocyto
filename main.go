// gocyto: Go call-graph analysis and visualization
//
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"github.com/protolambda/gocyto/analysis"
	"github.com/protolambda/gocyto/render"
	"html/template"
	"io"
	"os"
	"strings"
)

var (
	webFlag        = flag.Bool("web", false, "Output an index.html with graph data embedded instead of raw JSON")
	testFlag       = flag.Bool("tests", false, "Consider tests files as entry points for call-graph")
	goRootFlag     = flag.Bool("go-root", false, "Include packages part of the Go root")
	unexportedFlag = flag.Bool("unexported", false, "Include unexported function calls")
	queryDir       = flag.String("query-dir", "", "Directory to query from for go packages. Current dir if empty")
	modeFlag       = flag.String("mode", "pointer", "Type of analysis to run. One of: pointer, cha, rta, static")
	buildFlag      = flag.String("build", "", "Build flags to pass to Go build tool. Separated with spaces")
	outFlag        = flag.String("out", "", "Output file, if none is specified, output to std out")
)

const usage = `
Gocyto: Callgraph analysis and visualization for Go - by @protolambda

https://github.com/protolambda/gocyto

Usage:

gocyto [options...] <package path(s)>

Options:

`

type WebData struct {
	Packages  string
	GraphJSON template.JS
}

func main() {
	flag.Parse()

	args := flag.Args()
	if flag.NArg() == 0 {
		_, _ = fmt.Fprintf(os.Stderr, usage)
		flag.PrintDefaults()
		os.Exit(2)
	}

	var buildFlags []string
	if len(*buildFlag) > 0 {
		buildFlags = strings.Split(*buildFlag, " ")
	}

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
	aProg, err := analysis.RunAnalysis(*testFlag, buildFlags, args, *queryDir)
	check(err, "could not run program analysis: %v")

	callGraph := mode.ComputeCallgraph(aProg)
	cytoGraph := render.NewCytoGraph()

	opts := &render.RenderOptions{
		IncludeGoRoot:     *goRootFlag,
		IncludeUnexported: *unexportedFlag,
	}

	check(cytoGraph.LoadCallGraph(callGraph, opts), "could not call graph: %v")

	writeAsHtml := func(w io.Writer) {
		tmpl := template.Must(template.ParseFiles("index.gohtml"))
		var buf bytes.Buffer
		graphW := bufio.NewWriter(&buf)
		check(cytoGraph.WriteJson(graphW), "could not write graph to buffer: %v")
		check(graphW.Flush(), "could not flush graph buffer: %v")

		var pkgListText bytes.Buffer
		for _, p := range aProg.Mains {
			pkgListText.WriteString(p.Pkg.Path())
			pkgListText.WriteString("\n")
		}

		check(
			tmpl.Execute(w,
				WebData{
					Packages:  pkgListText.String(),
					GraphJSON: template.JS(buf.String()),
				}),
			"could not write index.html to output: %v")
	}
	outPath := *outFlag
	web := *webFlag
	if outPath == "" {
		if web {
			writeAsHtml(os.Stdout)
		} else {
			check(cytoGraph.WriteJson(os.Stdout), "could not write graph JSON to std out: %v")
		}
	} else {
		f, err := os.Create(outPath)
		check(err, "could not create file: %v")
		defer f.Close()
		w := bufio.NewWriter(f)

		if web {
			writeAsHtml(w)
		} else {
			check(cytoGraph.WriteJson(f), "could not write graph JSON to file: %v")
		}
		check(w.Flush(), "could not flush output to file: %v")
	}
}

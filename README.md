# Gocyto

A Go [SSA](https://godoc.org/golang.org/x/tools/go/ssa) callgraph builder and visualizer, by @protolambda.

Features:
- output to generic Cytoscape JSON format. (list of nodes, list of edges)
- use different [SSA analysis types](#supported-callgraph-analysis-types)
- support for Go-modules (powered by `golang.org/x/tools/go/packages`)
- graph data is nested: packages > types / globals > attached functions
- all edges/nodes enhanced with `classes` to style/filter the graph with

**Work in progress.**

## executable

Provide a Go package pattern to load the packages, and produce the call-graph.

```bash
gocyto github.com/user/project/some/package/...
```

TODO; tests flag, and analysis type switch


## `gocyto/analysis`

To easily load packages into a SSA program, and construct callgraphs.

Loading packages:

```go
program, err := analysis.RunAnalysis(withTests, buildFlags, packagePatterns)
```

Constructing a callgraph:

```go
analysis.PointerAnalysis.ComputeCallgraph(program)
```

### Supported callgraph analysis types:

- [`PointerAnalysis`](golang.org/x/tools/go/pointer)
- [`StaticAnalysis`](golang.org/x/tools/go/callgraph/static)
- [`ClassHierarchyAnalysis`](golang.org/x/tools/go/callgraph/cha)
- [`RapidTypeAnalysis`](golang.org/x/tools/go/callgraph/rta)

## `gocyto/render`

Loads call-graph into a Cyto-graph object. After loading your graph (or multiple),
 the data can be output to JSON to load with [cytoscape](http://js.cytoscape.org/#notation/elements-json).

Constructing a cyto graph:

```go
// Base object, manages nodes, edges and keeps track of a [full-name -> ID] map for shorter IDs
cytoGraph := render.NewCytoGraph()

// more options to be decided on later, PRs welcome
opts := &render.RenderOptions{
    IncludeGoRoot: false,
    IncludeUnexported: false,
}

// add call graph from SSA analysis to cyto graph
err := cytoGraph.LoadCallGraph(callGraph, opts)

// add more call graphs if you like
```

## Comparison

[`go-callvis`](https://github.com/TrueFurby/go-callvis)
- Similar purpose
- bloated/hacky code
- uses deprecated SSA package loading
- no re-usable library code
- an ugly non-go Graphviz dependency
- no Go module support.
- limited styling
- hacky build-tags support (overwriting the default Go build flags during runtime...)

[`prospect`](https://github.com/CorgiMan/prospect/blob/master/main.go)
- minimal
- outdated, 4 years old
- limited callgraph information extracted
- looks like the origin of godoc callgraph tool (???)

[`callgraph`](https://github.com/golang/tools/blob/master/cmd/callgraph/main.go)
- digraph and graphviz output support
- doesn't add extra information (description/classes) to the calls
- supports same set of analysis algorithms

[`godoc/analysis](https://godoc.org/golang.org/x/tools/godoc/analysis)
- different visualization; go-doc complement, tree-view/code navigation
- Powers [call-graph navigation](https://golang.org/lib/godoc/analysis/help.html)


## License

MIT License, see LICENSE file.

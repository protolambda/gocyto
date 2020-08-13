package analysis

import (
	"bytes"
	"errors"
	"fmt"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/callgraph/static"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/pointer"
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

type AnalysisMode uint64

const (
	PointerAnalysis AnalysisMode = iota
	StaticAnalysis
	ClassHierarchyAnalysis
	RapidTypeAnalysis
)

func RunAnalysis(withTests bool, buildFlags []string, pkgPatterns []string, queryDir string) (*ProgramAnalysis, error) {
	conf := &packages.Config{
		Mode:       pkgLoadMode,
		Tests:      withTests,
		BuildFlags: buildFlags,
		Dir:        queryDir,
	}
	pkgPatterns = append(pkgPatterns)
	loaded, err := packages.Load(conf, pkgPatterns...)
	if err != nil {
		return nil, fmt.Errorf("failed packages load: %v", err)
	}
	prog, initialPkgs := ssautil.Packages(loaded, 0)

	var errorMsg bytes.Buffer
	for i, p := range initialPkgs {
		if p == nil && loaded[i].Name != "" {
			errorMsg.WriteString("failed to get SSA for pkg: ")
			errorMsg.WriteString(loaded[i].PkgPath)
			errorMsg.WriteString("\n")
		}
	}
	if errorMsg.Len() != 0 {
		return nil, errors.New(errorMsg.String())
	}

	prog.Build()

	pkgs := prog.AllPackages()
	mains := ssautil.MainPackages(pkgs)

	return &ProgramAnalysis{
		Prog:  prog,
		Pkgs:  pkgs,
		Mains: mains,
	}, nil
}

func (mode AnalysisMode) ComputeCallgraph(data *ProgramAnalysis) *callgraph.Graph {
	switch mode {
	case PointerAnalysis:
		ptrcfg := &pointer.Config{
			Mains:          data.Mains,
			BuildCallGraph: true,
		}
		result, err := pointer.Analyze(ptrcfg)
		if err != nil { // not a user-input problem if it fails, see Analyze doc.
			panic(fmt.Errorf("pointer analysis failed %v", err))
		}
		return result.CallGraph
	case StaticAnalysis:
		return static.CallGraph(data.Prog)
	case ClassHierarchyAnalysis:
		return cha.CallGraph(data.Prog)
	case RapidTypeAnalysis:
		var roots []*ssa.Function
		for _, m := range data.Mains {
			roots = append(roots, m.Func("init"), m.Func("main"))
		}
		return rta.Analyze(roots, true).CallGraph
	default:
		return nil
	}
}

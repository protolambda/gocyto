package render

import (
	"encoding/json"
	"fmt"
	"go/build"
	"go/types"
	. "golang.org/x/tools/go/callgraph"
	"io"
	"strconv"
	"strings"
)

type RenderOptions struct {
	IncludeGoRoot     bool
	IncludeUnexported bool
}

func isShared(edge *Edge) bool {
	return edge.Caller.Func.Pkg == nil
}

func isSynthetic(edge *Edge) bool {
	return edge.Callee.Func.Synthetic != ""
}

func inGoRoot(node *Node) bool {
	pkg, _ := build.Import(node.Func.Pkg.Pkg.Path(), "", 0)
	return pkg.Goroot
}

func isUnexported(node *Node) bool {
	obj := node.Func.Object()
	return obj != nil && !obj.Exported()
}

func isGlobal(node *Node) bool {
	return node.Func.Parent() == nil
}

type CytoID string

type CytoNode struct {
	Id          CytoID
	Label       string
	Description *string // optional description
	Parent      CytoID
	Classes     []string
}

type CytoEdge struct {
	Id      CytoID
	Source  CytoID
	Target  CytoID
	Classes []string
}

type CytoGraph struct {
	idCounter uint64
	idMap     map[string]CytoID
	Nodes     map[CytoID]*CytoNode
	Edges     map[CytoID]*CytoEdge
}

func NewCytoGraph() *CytoGraph {
	return &CytoGraph{
		idCounter: 0,
		idMap:     make(map[string]CytoID),
		Nodes:     make(map[CytoID]*CytoNode),
		Edges:     make(map[CytoID]*CytoEdge),
	}
}

func (cg *CytoGraph) GetID(fullName string, isNode bool) (isNew bool, id CytoID) {
	if id, ok := cg.idMap[fullName]; ok {
		return false, id
	} else {
		cg.idCounter++
		id := "e"
		if isNode {
			id = "n"
		}
		id += strconv.FormatUint(cg.idCounter, 16)
		cID := CytoID(id)
		cg.idMap[fullName] = cID
		return true, cID
	}
}

func nodeFullName(node *Node) string {
	return node.Func.RelString(node.Func.Pkg.Pkg)
}

func (cg *CytoGraph) ProcessNode(node *Node) CytoID {
	funcName := nodeFullName(node)
	fullName := fmt.Sprintf("func ~ %s", funcName)
	isNew, id := cg.GetID(fullName, true)
	// just return ID directly if the node already exits
	if !isNew {
		return id
	}

	// node does not exist, create one, with the new id.
	cNode := &CytoNode{Id: id}

	cNode.Parent = cg.ProcessPkg(node.Func.Pkg.Pkg)

	if last := strings.LastIndex(funcName, "."); last >= 0 {
		cNode.Label = funcName[last:]
	} else {
		cNode.Label = funcName
	}

	// if it is attached to a type, overwrite the parent node. (type will have package as parent in turn)
	if recv := node.Func.Signature.Recv(); recv != nil {
		cNode.Parent = cg.ProcessRecv(recv)
	}

	if inGoRoot(node) {
		cNode.Classes = append(cNode.Classes, "go_root")
	}
	if isGlobal(node) {
		cNode.Classes = append(cNode.Classes, "global")
	}
	if isUnexported(node) {
		cNode.Classes = append(cNode.Classes, "unexported")
	}
	// TODO: maybe add (free/local) variables to the graph?

	cg.Nodes[id] = cNode
	return id
}

func (cg *CytoGraph) ProcessRecv(recv *types.Var) CytoID {
	pkg := recv.Pkg()
	fullName := fmt.Sprintf("recv ~ %s ~ %s", pkg.Path(), recv.Name())
	isNew, id := cg.GetID(fullName, true)
	// just return ID directly if the node already exits
	if !isNew {
		return id
	}

	// node does not exist, create one, with the new id.
	cNode := &CytoNode{
		Id:     id,
		Parent: cg.ProcessPkg(recv.Pkg()),
		Label:  recv.Name(),
	}
	if recv.Embedded() {
		cNode.Classes = append(cNode.Classes, "embedded")
	}
	if recv.IsField() {
		cNode.Classes = append(cNode.Classes, "field")
	}
	if !recv.Exported() {
		cNode.Classes = append(cNode.Classes, "unexported")
	}

	cg.Nodes[id] = cNode
	return id
}

func (cg *CytoGraph) ProcessPkg(pkg *types.Package) CytoID {
	fullName := fmt.Sprintf("pkg ~ %s", pkg.Path())
	isNew, id := cg.GetID(fullName, true)
	// just return ID directly if the node already exits
	if !isNew {
		return id
	}

	// node does not exist, create one, with the new id.
	path := pkg.Path()
	cNode := &CytoNode{
		Id:          id,
		Label:       pkg.Name(),
		Description: &path,
		Classes:     []string{"package"},
	}
	cg.Nodes[id] = cNode
	return id
}

func (cg *CytoGraph) ProcessEdge(edge *Edge) CytoID {
	fullName := fmt.Sprintf("call @%d ~ %s -> %s",
		edge.Pos(), nodeFullName(edge.Caller), nodeFullName(edge.Callee))
	isNew, id := cg.GetID(fullName, true)
	// just return ID directly if the node already exits
	if !isNew {
		return id
	}

	// process both ends of edge
	idCaller := cg.ProcessNode(edge.Caller)
	idCallee := cg.ProcessNode(edge.Callee)

	cEdge := &CytoEdge{
		Id:     id,
		Source: idCaller,
		Target: idCallee,
		// description precisely says what kind of edge this is, e.g. "concurrent static function closure call"
		Classes: strings.Split(edge.Description(), " "),
	}
	cg.Edges[id] = cEdge
	return id
}

func (cg *CytoGraph) LoadCallGraph(g *Graph, opts *RenderOptions) error {
	g.DeleteSyntheticNodes()

	return GraphVisitEdges(g, func(edge *Edge) error {

		if isSynthetic(edge) || isShared(edge) {
			return nil
		}

		if !opts.IncludeGoRoot && inGoRoot(edge.Callee) {
			return nil
		}

		if !opts.IncludeUnexported && isUnexported(edge.Callee) {
			return nil
		}

		cg.ProcessEdge(edge)
		return nil
	})
}

type CytoJsonOut struct {
	Nodes []*CytoNode
	Edges []*CytoEdge
}

func (cg *CytoGraph) WriteJson(w io.Writer) error {
	out := CytoJsonOut{}
	for _, n := range cg.Nodes {
		out.Nodes = append(out.Nodes, n)
	}
	for _, e := range cg.Edges {
		out.Edges = append(out.Edges, e)
	}
	enc := json.NewEncoder(w)
	return enc.Encode(out)
}

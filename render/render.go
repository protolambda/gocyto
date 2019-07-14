package render

import (
	"encoding/json"
	"fmt"
	"github.com/lucasb-eyer/go-colorful"
	"go/build"
	"go/types"
	. "golang.org/x/tools/go/callgraph"
	"hash/fnv"
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

type NodeData struct {
	Id          CytoID  `json:"id"`
	Label       string  `json:"label"`
	Description *string `json:"description,omitempty"` // optional description
	Parent      CytoID  `json:"parent"`
	Color       string  `json:"color"`
}

type CytoNode struct {
	Data    NodeData `json:"data"`
	Classes []string `json:"classes"`
}

type EdgeData struct {
	Id     CytoID `json:"id"`
	Source CytoID `json:"source"`
	Target CytoID `json:"target"`
}

type CytoEdge struct {
	Data    EdgeData `json:"data"`
	Classes []string `json:"classes"`
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

func stringToIntHash(v string) uint32 {
	hasher := fnv.New32()
	_, _ = hasher.Write([]byte(v))
	return hasher.Sum32()
}

func tupleToIntHashes(tup *types.Tuple) []uint32 {
	var res []uint32
	count := tup.Len()

	for i := 0; i < count; i++ {
		p := tup.At(i)
		res = append(res, stringToIntHash(p.Type().String()))
	}
	return res
}

var defaultColor = MustParseHex("#3D4CC4")

func integersToColor(values ...uint32) colorful.Color {
	if len(values) == 0 {
		return defaultColor
	}
	mix := colorful.Color{R: 0.0, G: 0.0, B: 0.0}
	for _, p := range values {
		c := keypoints.GetInterpolatedColorFor(float64(p) / float64(^uint32(0)))
		t := float64(1.0) / float64(len(values))
		if t == 1.0 {
			return c
		}
		mix = c.BlendHcl(mix, 0.3)
	}
	return mix
}

func signatureToColorHex(signature *types.Signature) string {
	params := integersToColor(tupleToIntHashes(signature.Params())...)
	results := integersToColor(tupleToIntHashes(signature.Results())...)
	return params.BlendHcl(results, 0.5).Hex()
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
	cNode := &CytoNode{Data: NodeData{Id: id}}

	cNode.Data.Parent = cg.ProcessPkg(node.Func.Pkg.Pkg)

	if last := strings.LastIndex(funcName, "."); last >= 0 {
		cNode.Data.Label = funcName[last:]
	} else {
		cNode.Data.Label = funcName
	}

	cNode.Data.Color = signatureToColorHex(node.Func.Signature)

	// if it is attached to a type, overwrite the parent node. (type will have package as parent in turn)
	if recv := node.Func.Signature.Recv(); recv != nil {
		cNode.Data.Parent = cg.ProcessRecv(recv)
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
	fullName := fmt.Sprintf("recv ~ %s ~ %s", pkg.Path(), recv.Type().String())
	isNew, id := cg.GetID(fullName, true)
	// just return ID directly if the node already exits
	if !isNew {
		return id
	}

	// node does not exist, create one, with the new id.
	cNode := &CytoNode{
		Data: NodeData{
			Id:     id,
			Parent: cg.ProcessPkg(recv.Pkg()),
			Label:  recv.Type().String(),
		},
	}

	cNode.Data.Color = integersToColor(stringToIntHash(cNode.Data.Label)).Hex()

	// strip package name from type
	if last := strings.LastIndex(cNode.Data.Label, "."); last >= 0 {
		cNode.Data.Label = cNode.Data.Label[last+1:]
	}

	cNode.Classes = append(cNode.Classes, "type")

	if recv.Embedded() {
		cNode.Classes = append(cNode.Classes, "embedded")
	}
	if recv.IsField() {
		cNode.Classes = append(cNode.Classes, "field")
	}
	if !recv.Exported() {
		cNode.Classes = append(cNode.Classes, "unexported2") // TODO
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
		Data: NodeData{
			Id:          id,
			Label:       pkg.Name(),
			Description: &path,
		},
		Classes: []string{"package"},
	}
	cNode.Data.Color = integersToColor(stringToIntHash(cNode.Data.Label)).Hex()
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
		Data: EdgeData{
			Id:     id,
			Source: idCaller,
			Target: idCallee,
		},
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
	Nodes []*CytoNode `json:"nodes"`
	Edges []*CytoEdge `json:"edges"`
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

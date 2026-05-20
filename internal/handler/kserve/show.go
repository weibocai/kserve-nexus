package kserve

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/awalterschulze/gographviz"
	ksvcv1alpha1 "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
)

type isvcGraphNodeStatus string

const (
	isvcGraphNodeStatusTrue    isvcGraphNodeStatus = "True"
	isvcGraphNodeStatusFalse   isvcGraphNodeStatus = "False"
	isvcGraphNodeStatusUnknown isvcGraphNodeStatus = "Unknown"
	isvcGraphNodeStatusWarning isvcGraphNodeStatus = "Warning"
)

type graphNodeKind string

const (
	graphNodeKindStep graphNodeKind = "graphStep"
	graphNodeKindNode graphNodeKind = "graphNode"
)

const (
	NodeColorTrue  = `"#88ff0022"`
	NodeColorFalse = `"#FF0000"`
)

type graphNode struct {
	Id    string `json:"id"`
	Label string `json:"label,omitempty"`
	Name  struct {
		Namespace string        `json:"namespace"`
		Name      string        `json:"name"`
		Kind      graphNodeKind `json:"kind"`
	} `json:"name"`
	Parent      string              `json:"parent,omitempty"`
	Description map[string]string   `json:"description,omitempty"`
	Status      isvcGraphNodeStatus `json:"status"`
}

type graphEdge struct {
	Id          string            `json:"id"`
	Source      string            `json:"source"`
	Target      string            `json:"target"`
	Label       string            `json:"label"`
	Kind        string            `json:"kind"`
	Description map[string]string `json:"description,omitempty"`
}

type simpleObject struct {
	Pk          string
	Namespace   string
	Name        string
	Kind        string
	Parent      []*simpleObject
	Belong      *simpleObject
	Status      isvcGraphNodeStatus
	Description map[string]string `json:"description,omitempty"`
}

func (s *simpleObject) GetPk() string {
	return strings.ReplaceAll(s.Pk, "-", "_")
}

func (s *simpleObject) AddParent(p ...*simpleObject) {
	s.Parent = append(s.Parent, p...)
}
func (s *simpleObject) SetName(name string) {
	s.Name = name
}

func (s *simpleObject) SetStatus(status isvcGraphNodeStatus) {
	s.Status = status
}

func (s *simpleObject) ToGraphNode() graphNode {
	var node = graphNode{Id: s.Pk, Label: "", Name: struct {
		Namespace string        `json:"namespace"`
		Name      string        `json:"name"`
		Kind      graphNodeKind `json:"kind"`
	}{Name: s.Name, Namespace: s.Namespace, Kind: graphNodeKind(s.Kind)}, Status: s.Status}
	if s.Belong != nil {
		node.Parent = s.Belong.Pk
	}
	return node
}
func (s *simpleObject) ToGraphEdge(hadEdgeAdd map[string]bool) []graphEdge {
	edges := make([]graphEdge, 0)
	for index := range s.Parent {
		if s.Parent[index] == nil {
			continue
		}
		pk := s.Parent[index].Pk + "&&" + s.Pk
		if _, ok := hadEdgeAdd[pk]; ok {
			continue
		}
		hadEdgeAdd[pk] = true
		edge := graphEdge{Id: pk, Source: s.Parent[index].Pk, Target: s.Pk, Kind: string(s.Parent[index].Status)}
		if label, ok := s.Description["eLabel"]; ok {
			edge.Label = label
		}
		edges = append(edges, edge)
	}
	return edges
}

func (s *simpleObject) ToDigraphNode() map[string]string {
	var node = graphNode{Id: s.Pk, Label: "", Name: struct {
		Namespace string        `json:"namespace"`
		Name      string        `json:"name"`
		Kind      graphNodeKind `json:"kind"`
	}{Name: s.Name, Namespace: s.Namespace, Kind: graphNodeKind(s.Kind)}, Status: s.Status}
	if s.Belong != nil {
		node.Parent = s.Belong.Pk
	}
	fillcolor := NodeColorTrue
	if s.Status != isvcGraphNodeStatusTrue {
		fillcolor = NodeColorFalse
	}
	return map[string]string{
		"fillcolor": fillcolor,
		"label": fmt.Sprintf(`<<table border="0" cellborder="1" cellspacing="0" cellpadding="3">
    <tr> <td port="name" sides="ltr">%s</td> </tr>
    <tr> <td port="namespace" sides="ltr"> %s</td> </tr>
    <tr> <td port="kind" sides="lbr"> %s</td> </tr>
</table>>`, s.Name, s.Namespace, s.Kind),
		"shape": "plain",
	}
}
func (s *simpleObject) ToDigraphEdge(hadEdgeAdd map[string]bool) []graphEdge {
	edges := make([]graphEdge, 0)
	for index := range s.Parent {
		if s.Parent[index] == nil {
			continue
		}
		pk := s.Parent[index].Pk + "&&" + s.Pk
		if _, ok := hadEdgeAdd[pk]; ok {
			continue
		}
		hadEdgeAdd[pk] = true
		edge := graphEdge{Id: pk, Source: s.Parent[index].Pk, Target: s.Pk, Kind: string(s.Parent[index].Status)}
		if label, ok := s.Description["eLabel"]; ok {
			edge.Label = label
		}
		edges = append(edges, edge)
	}
	return edges
}

type simpleObjectMap map[string]*simpleObject

func (s *simpleObjectMap) AddNodes(name, namespace, kind string, status isvcGraphNodeStatus, belong *simpleObject, parent ...*simpleObject) *simpleObject {
	pk := namespace + "_" + name + "_" + kind
	if obj, ok := (*s)[pk]; !ok {
		(*s)[pk] = &simpleObject{Pk: pk, Namespace: namespace, Name: name, Kind: kind, Status: status, Parent: parent, Belong: belong}
	} else {
		obj.Belong = belong
		obj.AddParent(parent...)
	}
	return (*s)[pk]
}
func (s *simpleObjectMap) AddGraph(namespace string, step ksvcv1alpha1.InferenceStep, nodeType ksvcv1alpha1.InferenceRouterType, parent ...*simpleObject) *simpleObject {
	var name, kind = "", graphNodeKindStep
	if step.StepName != "" {
		name = step.StepName
	} else if step.NodeName != "" {
		name = step.NodeName
		kind = graphNodeKindNode
	} else if step.ServiceName != "" {
		name = step.ServiceName
	} else {
		name = ""
	}
	_kind := string(kind)
	var pk = namespace + ";" + name + ";" + _kind
	obj, ok := (*s)[pk]
	if !ok {
		(*s)[pk] = &simpleObject{Pk: pk, Namespace: namespace, Name: name, Kind: _kind, Status: isvcGraphNodeStatusTrue, Parent: parent, Description: make(map[string]string)}
		obj = (*s)[pk]
	} else {
		obj.AddParent(parent...)
	}
	eLabel := ""
	if nodeType == ksvcv1alpha1.Switch || (nodeType == ksvcv1alpha1.Sequence && step.Condition != "") {
		eLabel = "Condition: " + step.Condition
	}
	if nodeType == ksvcv1alpha1.Splitter {
		eLabel = eLabel + "\nWeight: " + strconv.FormatInt(*step.Weight, 10) + "%"
	}
	if eLabel != "" {
		for i := range parent {
			obj.Description[parent[i].Pk] += "\n" + eLabel
		}
	}

	return obj
}

func simpleObject2graphNode(objs simpleObjectMap) ([]graphNode, []graphEdge) {
	hadNodeAdd := make(map[string]bool)
	hadEdgeAdd := make(map[string]bool)
	nodes := make([]graphNode, 0)
	edges := make([]graphEdge, 0)

	for _, obj := range objs {
		if _, ok := hadNodeAdd[obj.Pk]; !ok {
			nodes = append(nodes, obj.ToGraphNode())
		}
		edges = append(edges, obj.ToGraphEdge(hadEdgeAdd)...)
	}
	return nodes, edges
}

func simpleObject2DigraphNode(name string, objs simpleObjectMap) (string, error) {
	name = strings.ReplaceAll(name, "-", "_")
	initGraph := fmt.Sprintf(`digraph %s {}`, name)
	graph := gographviz.NewGraph()
	if err := graph.SetName(name); err != nil {
		return "", err
	}
	_ = graph.SetDir(true)
	cIndex := 1
	cIndex2pk := make(map[string]string)
	for _, obj := range objs {
		_name := name
		if obj.Belong != nil {
			_name = "cluster" + strconv.Itoa(cIndex)
			if index, ok := cIndex2pk[obj.Belong.Pk]; ok {
				_name = index
			} else {
				cIndex2pk[obj.Belong.Pk] = _name
			}
			if !graph.IsSubGraph(_name) {
				cIndex = cIndex + 1
				if err := graph.AddSubGraph(name, _name, map[string]string{}); err != nil {
					return initGraph, err
				}
			}

		}
		if !graph.IsNode(_name) {
			if err := graph.AddNode(_name, obj.GetPk(), obj.ToDigraphNode()); err != nil {
				return initGraph, err
			}
		}
		if obj.Parent == nil {
			continue
		}
		for j := range obj.Parent {
			if obj.Parent[j] == nil {
				continue
			}
			if err := graph.AddPortEdge(obj.Parent[j].GetPk(), "kind", obj.GetPk(), "name", true, obj.Description); err != nil {
				return initGraph, err
			}
		}
	}
	gs := fmt.Sprintf(`digraph %s {
	graph [
		labelloc = t
		fontname = "Helvetica,Arial,sans-serif"
		fontsize = 10
		layout = dot
		newrank = true
	]
	node [
		style=filled
		shape=rect
		fontsize = 10
		pencolor="#00000044" // frames color
		fontname="Helvetica,Arial,sans-serif"
		shape=plaintext
	]
	edge [
		arrowsize=0.5
		fontname="Helvetica,Arial,sans-serif"
		labeldistance=3
		labelfontcolor="#00000080"
		penwidth=2
		style=dotted // dotted style symbolizes data transfer
	]
	`, name) + strings.ReplaceAll(graph.String(), fmt.Sprintf("digraph %s {", name), "")
	gss := strings.Split(gs, "\n")
	newGss := make([]string, 0)
	for i := range gss {
		if strings.TrimSpace(gss[i]) == "" || strings.TrimSpace(gss[i]) == ";" {
			continue
		}
		newGss = append(newGss, gss[i])
	}
	return strings.Join(newGss, "\n"), nil
}

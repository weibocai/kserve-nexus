package kserve

import (
	"strconv"

	ksvcv1alpha1 "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
)

type GraphNodeStatus string

const (
	GraphNodeStatusTrue    GraphNodeStatus = "True"
	GraphNodeStatusFalse   GraphNodeStatus = "False"
	GraphNodeStatusUnknown GraphNodeStatus = "Unknown"
	GraphNodeStatusWarning GraphNodeStatus = "Warning"
)

type graphNodeKind string

const (
	graphNodeKindStep graphNodeKind = "graphStep"
	graphNodeKindNode graphNodeKind = "graphNode"
)

type GraphNodeColor string

const (
	GraphNodeColorTrue    GraphNodeColor = `"#88ff0022"`
	GraphNodeColorFalse   GraphNodeColor = `"#FF0000"`
	GraphNodeColorUnknown GraphNodeColor = `"#808080"`
	GraphNodeColorWarning GraphNodeColor = `"#FFFF00"`
)

type GraphEdge struct {
	Id          string            `json:"id"`
	Source      string            `json:"source"`
	Target      string            `json:"target"`
	Label       string            `json:"label"`
	Kind        string            `json:"kind"`
	Description map[string]string `json:"description,omitempty"`
}

// GraphNode 图展示节点
type GraphNode struct {
	Pk          string            `json:"id"`        // 节点唯一标识
	Namespace   string            `json:"namespace"` // 命名空间
	Name        string            `json:"name"`      // 节点名称
	Kind        string            `json:"kind"`      // 节点类型
	Parent      []*GraphNode      // 父级节点
	Belong      *GraphNode        `json:"belong,omitempty"`      // 嵌套图
	Color       GraphNodeColor    `json:"color"`                 // 节点颜色
	Status      GraphNodeStatus   `json:"status"`                // 节点状态
	Description map[string]string `json:"description,omitempty"` // 节点描述
}

// AddParent 添加父级节点
func (s *GraphNode) AddParent(p ...*GraphNode) {
	s.Parent = append(s.Parent, p...)
}

// SetName 设置节点名称
func (s *GraphNode) SetName(name string) {
	s.Name = name
}

// SetStatus 设置节点状态
func (s *GraphNode) SetStatus(status GraphNodeStatus) {
	s.Status = status
	switch status {
	case GraphNodeStatusTrue:
		s.Color = GraphNodeColorTrue
	case GraphNodeStatusFalse:
		s.Color = GraphNodeColorFalse
	case GraphNodeStatusUnknown:
		s.Color = GraphNodeColorUnknown
	case GraphNodeStatusWarning:
		s.Color = GraphNodeColorWarning
	}
}

// ToGraphEdge 添加节点的关系：边、包含
func (s *GraphNode) ToGraphEdge(hadEdgeAdd map[string]bool) []*GraphEdge {
	edges := make([]*GraphEdge, 0)
	for index := range s.Parent {
		if s.Parent[index] == nil {
			continue
		}
		pk := s.Parent[index].Pk + "&&" + s.Pk
		if _, ok := hadEdgeAdd[pk]; ok {
			continue
		}
		hadEdgeAdd[pk] = true
		edge := &GraphEdge{Id: pk, Source: s.Parent[index].Pk, Target: s.Pk, Kind: string(s.Parent[index].Status)}
		if label, ok := s.Description["eLabel"]; ok {
			edge.Label = label
		}
		edges = append(edges, edge)
	}
	return edges
}

// GraphNodeMap 节点结构
type GraphNodeMap map[string]*GraphNode

// AddNodes 添加节点
func (s *GraphNodeMap) AddNodes(name, namespace, kind string, status GraphNodeStatus, belong *GraphNode, parent ...*GraphNode) *GraphNode {
	pk := namespace + "_" + name + "_" + kind
	if obj, ok := (*s)[pk]; !ok {
		(*s)[pk] = &GraphNode{Pk: pk, Namespace: namespace, Name: name, Kind: kind, Status: status, Parent: parent, Belong: belong}
	} else {
		if belong != nil {
			obj.Belong = belong
		}
		obj.AddParent(parent...)
	}
	return (*s)[pk]
}
func (s *GraphNodeMap) AddGraph(namespace string, step ksvcv1alpha1.InferenceStep, nodeType ksvcv1alpha1.InferenceRouterType, parent ...*GraphNode) *GraphNode {
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
		(*s)[pk] = &GraphNode{Pk: pk, Namespace: namespace, Name: name, Kind: _kind, Status: GraphNodeStatusTrue, Parent: parent, Description: make(map[string]string)}
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

func GraphNode2graphNode(objs GraphNodeMap) ([]*GraphNode, []*GraphEdge) {
	hadEdgeAdd := make(map[string]bool)
	nodes := make([]*GraphNode, 0)
	edges := make([]*GraphEdge, 0)

	for _, obj := range objs {
		nodes = append(nodes, obj)
		edges = append(edges, obj.ToGraphEdge(hadEdgeAdd)...)
	}
	return nodes, edges
}

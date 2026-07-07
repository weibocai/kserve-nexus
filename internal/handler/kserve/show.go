package kserve

import (
	"encoding/json"
)

const (
	InferenceGraphNodeKind = "InferenceGraph"
)

type GraphNodeStatus string

const (
	GraphNodeStatusTrue    GraphNodeStatus = "True"
	GraphNodeStatusFalse   GraphNodeStatus = "False"
	GraphNodeStatusUnknown GraphNodeStatus = "Unknown"
	GraphNodeStatusWarning GraphNodeStatus = "Warning"
)

type GraphNodeColor string

const (
	GraphNodeColorTrue           GraphNodeColor = `"#88ff0022"`
	GraphNodeColorFalse          GraphNodeColor = `"#FF0000"`
	GraphNodeColorUnknown        GraphNodeColor = `"#808080"`
	GraphNodeColorWarning        GraphNodeColor = `"#FFFF00"`
	GraphNodeColorInferenceGraph GraphNodeColor = `"#0dff00"`
)

// GraphNodeStatus2GraphNodeColor 节点状态转换成颜色
func GraphNodeStatus2GraphNodeColor(status GraphNodeStatus, kind string) GraphNodeColor {
	if kind == InferenceGraphNodeKind {
		return GraphNodeColorInferenceGraph
	}
	switch status {
	case GraphNodeStatusTrue:
		return GraphNodeColorTrue
	case GraphNodeStatusFalse:
		return GraphNodeColorFalse
	case GraphNodeStatusUnknown:
		return GraphNodeColorUnknown
	case GraphNodeStatusWarning:
		return GraphNodeColorWarning
	default:
		return GraphNodeColorUnknown
	}
}

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
	Pk          string            `json:"id"`                    // 节点唯一标识
	Namespace   string            `json:"namespace"`             // 命名空间
	Name        string            `json:"name"`                  // 节点名称
	Kind        string            `json:"kind"`                  // 节点类型
	Belong      string            `json:"belong,omitempty"`      // 嵌套图
	Color       GraphNodeColor    `json:"color"`                 // 节点颜色
	Status      GraphNodeStatus   `json:"status"`                // 节点状态
	Description map[string]string `json:"description,omitempty"` // 节点描述
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

// GraphNodeMap 节点结构
type GraphNodeMap struct {
	Nodes map[string]*GraphNode // 点的集合
	Edges map[string]*GraphEdge // 边的集合
}

// GetNodes 获取所有节点
func (s *GraphNodeMap) GetNodes() []*GraphNode {
	nodes := make([]*GraphNode, len(s.Nodes))
	index := 0
	for _, obj := range s.Nodes {
		nodes[index] = obj
		index++
	}
	return nodes
}

// GetEdges 获取所有边列表
func (s *GraphNodeMap) GetEdges() []*GraphEdge {
	edges := make([]*GraphEdge, len(s.Edges))
	index := 0
	for _, obj := range s.Edges {
		edges[index] = obj
		index++
	}
	return edges
}

// MarshalJSON 节点+边结构转换成json
func (s *GraphNodeMap) MarshalJSON() ([]byte, error) {

	return json.Marshal(map[string]interface{}{"nodes": s.GetNodes(), "edges": s.GetEdges()})
}

func (s *GraphNodeMap) AddEdges(target *GraphNode, label string, parents ...*GraphNode) {
	for _, p := range parents {
		if p == nil {
			continue
		}
		pk := p.Pk + "&&" + target.Pk
		if obj, ok := s.Edges[pk]; ok {
			if label != "" {
				obj.Label = label
			}
			continue
		}
		s.Edges[pk] = &GraphEdge{Id: pk, Source: p.Pk, Target: target.Pk, Label: label}
	}
}

// AddNodes 添加节点，如果边有标签，则使用这个函数
func (s *GraphNodeMap) AddNodesLabel(name, namespace, kind, label string, status GraphNodeStatus, belong *GraphNode, parent ...*GraphNode) *GraphNode {
	pk := namespace + "_" + name + "_" + kind
	if _, ok := s.Nodes[pk]; !ok {
		s.Nodes[pk] = &GraphNode{Pk: pk, Namespace: namespace, Name: name, Kind: kind, Status: status, Description: make(map[string]string)}
	}
	obj := s.Nodes[pk]
	if belong != nil {
		obj.Belong = belong.Pk
	}
	s.AddEdges(obj, label, parent...)
	return obj
}

// AddNodes 添加节点，如果边没有标签，则使用这个函数
func (s *GraphNodeMap) AddNodes(name, namespace, kind string, status GraphNodeStatus, belong *GraphNode, parent ...*GraphNode) *GraphNode {
	return s.AddNodesLabel(name, namespace, kind, "", status, belong, parent...)
}

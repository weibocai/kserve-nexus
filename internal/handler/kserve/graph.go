package kserve

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	ksvcv1alpha1 "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	ksvcconstants "github.com/kserve/kserve/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kserve-nexus/internal/middleware"
	"github.com/kserve-nexus/pkg/utils"
)

func (kh *Handler) getGraph(ctx context.Context, namespace string, graph []map[string]string) ([]map[string]string, error) {
	var gls ksvcv1alpha1.InferenceGraphList
	if err := kh.kc.List(ctx, &gls, &client.ListOptions{Namespace: namespace}); err != nil {
		return graph, err
	}
	for i := range gls.Items {
		ready := "False"
		for j := range gls.Items[i].Status.Conditions {
			if gls.Items[i].Status.Conditions[j].Type == "Ready" {
				ready = string(gls.Items[i].Status.Conditions[j].Status)
			}
		}
		_graph := map[string]string{
			"namespace": gls.Items[i].GetNamespace(), "name": gls.Items[i].GetName(), "url": gls.Items[i].Status.URL.String(), "Ready": ready,
		}
		graph = append(graph, _graph)
	}
	return graph, nil
}

// ListGraph 获取推理图列表
func (kh *Handler) ListGraph(c *gin.Context) {
	graph := make([]map[string]string, 0)
	namespace := c.DefaultQuery("namespace", "all")
	if namespace == "all" {
		namespaces, err := kh.listNamespaces(c.Request.Context())
		if err != nil {
			c.Status(http.StatusInternalServerError)
			//nolint:gocritic
			_ = c.Error(err)
			return
		}
		for index := range namespaces {
			graph, err = kh.getGraph(c.Request.Context(), namespaces[index], graph)
			if err != nil {
				c.Status(http.StatusInternalServerError)
				_ = c.Error(err)
				return
			}
		}
		middleware.SuccessJson(c, graph)
		return
	}
	gl, err := kh.getGraph(c.Request.Context(), namespace, graph)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		_ = c.Error(err)
		return
	}
	middleware.SuccessJson(c, gl)
	return
}

// GetStepServiceName 获取推理步骤服务名称，这里会进一步改进，通过service url获取对于的服务信息
func GetStepServiceName(step ksvcv1alpha1.InferenceStep) string {
	if step.ServiceName != "" {
		return step.ServiceName
	}
	return step.ServiceURL
}

// InferenceGraph2GraphNode 推理图节点转换成GraphNode
func InferenceGraph2GraphNode(namespace string, step ksvcv1alpha1.InferenceStep, nodeMap *GraphNodeMap) *GraphNode {
	var node *GraphNode
	if step.NodeName == "" {
		node = nodeMap.AddNodes(GetStepServiceName(step), namespace, utils.GetCrdKey("isvc"), GraphNodeStatusTrue, nil)
	} else {
		node = nodeMap.AddNodes(step.NodeName, namespace, InferenceGraphNodeKind, GraphNodeStatusTrue, nil)
	}
	if step.StepName != "" {
		node.Description["StepName"] = step.StepName
	}
	return node
}

// InferenceGraph2GraphEdge 推理图边转换成GraphEdge
func InferenceGraph2GraphEdge(node ksvcv1alpha1.InferenceRouter, index int, source, target *GraphNode, nodeMap *GraphNodeMap) {
	label := ""
	if node.RouterType == ksvcv1alpha1.Switch || (node.RouterType == ksvcv1alpha1.Sequence && node.Steps[index].Condition != "") {
		label = "Condition: " + node.Steps[index].Condition
	}
	if node.RouterType == ksvcv1alpha1.Splitter {
		label = "Weight: " + strconv.FormatInt(*node.Steps[index].Weight, 10) + "%"
	}
	nodeMap.AddEdges(target, label, source)
}

// GraphStep2Show 推理图步骤转换成GraphNode
func GraphStep2Show(namespace string, graph ksvcv1alpha1.InferenceGraphSpec, step ksvcv1alpha1.InferenceStep, successor *GraphNode, nodeMap *GraphNodeMap) {
	if step.NodeName != "" {
		Graph2Show(step.NodeName, namespace, graph, successor, nil, nodeMap)
		return
	}
	if successor == nil {
		return
	}
	gn := InferenceGraph2GraphNode(namespace, step, nodeMap)
	nodeMap.AddEdges(successor, "", gn)
}

// Graph2Show 推理图转换成GraphNode
func Graph2Show(name, namespace string, graph ksvcv1alpha1.InferenceGraphSpec, successor, graphNode *GraphNode, nodeMap *GraphNodeMap) {
	currentNode := graph.Nodes[name]
	if graphNode == nil {
		graphNode = nodeMap.AddNodes(name, namespace, InferenceGraphNodeKind, GraphNodeStatusTrue, nil)
	}
	if len(currentNode.Steps) == 0 {
		if successor != nil {
			nodeMap.AddEdges(successor, "", graphNode)
		}
		return
	}
	// 顺序执行
	if currentNode.RouterType == ksvcv1alpha1.Sequence {
		gn := InferenceGraph2GraphNode(namespace, currentNode.Steps[0], nodeMap)
		InferenceGraph2GraphEdge(currentNode, 0, graphNode, gn, nodeMap)
		for i := 0; i < len(currentNode.Steps); i++ {
			var succ *GraphNode
			if i == len(currentNode.Steps)-1 {
				succ = successor
			} else {
				succ = InferenceGraph2GraphNode(namespace, currentNode.Steps[i+1], nodeMap)
			}
			GraphStep2Show(namespace, graph, currentNode.Steps[i], succ, nodeMap)
		}
		return
	}
	for i := range currentNode.Steps {
		gn := InferenceGraph2GraphNode(namespace, currentNode.Steps[i], nodeMap)
		InferenceGraph2GraphEdge(currentNode, i, graphNode, gn, nodeMap)
		GraphStep2Show(namespace, graph, currentNode.Steps[i], successor, nodeMap)
	}
	return
}

// GraphDetail 推理图详情
func (kh *Handler) GraphDetail(ctx context.Context, name, namespace string) (*ksvcv1alpha1.InferenceGraph, ksvcconstants.DeploymentModeType, *GraphNodeMap, GraphNodeStatus, error) {
	graph := ksvcv1alpha1.InferenceGraph{}
	ready := GraphNodeStatusFalse
	if err := kh.kc.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &graph); err != nil {
		return nil, "", nil, ready, err
	}
	cm := &corev1.ConfigMap{}
	if err := kh.kc.Get(ctx, client.ObjectKey{Name: ksvcconstants.InferenceServiceConfigMapName, Namespace: ksvcconstants.KServeNamespace}, cm); err != nil {
		return nil, "", nil, ready, err
	}

	dm, err := kh.getDeploymentMode(ctx, graph.Status.DeploymentMode, graph.Annotations, cm)
	if err != nil {
		return nil, "", nil, ready, err
	}
	// 获取推理图状态
	for i := range graph.Status.Conditions {
		if graph.Status.Conditions[i].Type == "Ready" && graph.Status.Conditions[i].Status == "True" {
			ready = GraphNodeStatusTrue
			break
		}
	}
	var tail *GraphNode
	nodes := GraphNodeMap{Edges: make(map[string]*GraphEdge), Nodes: make(map[string]*GraphNode)}
	// 不同的模式处理
	if dm == ksvcconstants.Standard {
		svc := NewService(ctx, kh.kc, graph.Name, graph.Name, graph.Namespace, "")
		tail, _ = svc.ToGraphNode(ctx, &nodes, nil)
	} else {
		ksvc := NewKnativeService(ctx, kh.kc, graph.Name, graph.Namespace)
		tail, _ = ksvc.ToGraphNode(ctx, kh.kc, &nodes, nil)
	}
	head := nodes.AddNodes(ksvcv1alpha1.GraphRootNodeName, namespace, InferenceGraphNodeKind, GraphNodeStatusTrue, nil)
	Graph2Show(ksvcv1alpha1.GraphRootNodeName, graph.Namespace, graph.Spec, nil, head, &nodes)
	nodes.AddEdges(head, "", tail)
	return &graph, dm, &nodes, ready, nil
}

// GetGraph 获取推理图详情
func (kh *Handler) GetGraph(c *gin.Context) {
	namespace := c.Query("namespace")
	name := c.Param("name")
	if namespace == "" || name == "" {
		c.Status(http.StatusBadRequest)
		_ = c.Error(fmt.Errorf("namespace and name must be provided"))
		return
	}

	graph, dm, nodes, ready, err := kh.GraphDetail(c.Request.Context(), namespace, name)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		_ = c.Error(err)
		return
	}
	middleware.SuccessJson(c, map[string]any{"graph": nodes, "dm": dm, "status": ready, "isvcGraph": graph})
}

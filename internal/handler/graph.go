package kserve

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kserve-nexus/internal/middleware"
	ksvcv1alpha1 "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	ksvcconstants "github.com/kserve/kserve/pkg/constants"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (kh *KserveHandler) getGraph(ctx context.Context, namespace string, graph []map[string]string) ([]map[string]string, error) {
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
func (kh *KserveHandler) ListGraph(c *gin.Context) {
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

func (kh *KserveHandler) getNodeEdgeLabel(graph *map[string]ksvcv1alpha1.InferenceRouter, node *ksvcv1alpha1.InferenceRouter, index int) (string, string, graphNodeKind) {
	eLabel := ""
	if node.RouterType == ksvcv1alpha1.Switch || (node.RouterType == ksvcv1alpha1.Sequence && node.Steps[index].Condition != "") {
		eLabel = "Condition: " + node.Steps[index].Condition
	}

	if node.RouterType == ksvcv1alpha1.Splitter {
		eLabel = "Weight: " + strconv.FormatInt(*node.Steps[index].Weight, 10) + "%"
	}
	kind := graphNodeKindNode
	// 官方允许 StepName 为空字符串，且NodeName与ServiceName不能同时存在
	nName := node.Steps[index].StepName
	if node.Steps[index].NodeName != "" {
		kind = graphNodeKindStep
		if nName != "" {
			nName += "-"
		}
		nName += node.Steps[index].NodeName + ": " + string((*graph)[node.Steps[index].NodeName].RouterType)
	}
	if nName == "" && node.Steps[index].ServiceName != "" {
		nName = node.Steps[index].ServiceName
	}
	return eLabel, nName, kind
}

func (kh *KserveHandler) node2Edge(namespace, nodeName string, parent []*simpleObject, graph *map[string]ksvcv1alpha1.InferenceRouter, nodes simpleObjectMap) []*simpleObject {
	node := (*graph)[nodeName]
	addNodes := make([]*simpleObject, 0)
	for i := range node.Steps {
		obj := nodes.AddGraph(namespace, node.Steps[i], node.RouterType, parent...)
		addNodes = append(addNodes, obj)
		if node.Steps[i].NodeName == "" {
			continue
		}
		cn := kh.node2Edge(namespace, node.Steps[i].NodeName, []*simpleObject{obj}, graph, nodes)
		if len(cn) > 0 {
			parent = cn
		} else {
			if node.RouterType == ksvcv1alpha1.Sequence {
				parent = []*simpleObject{obj}
			}
		}
	}
	if node.RouterType != ksvcv1alpha1.Sequence {
		return addNodes
	}
	length := len(addNodes)
	return addNodes[length-1:]
}

// GetGraph 获取推理图详情
// @Summary 获取InferenceGraph详情
// @Description 获取指定命名空间下单个InferenceGraph的详细信息，包含推理路由拓扑图
// @Tags graph
// @Accept json
// @Produce json
// @Param name path string true "InferenceGraph名称"
// @Param namespace query string true "命名空间"
// @Success 200 {object} middleware.Response "成功"
// @Failure 400 {object} middleware.Response "参数错误"
// @Failure 500 {object} middleware.Response "请求异常"
// @Router /kserve/graph/{name} [get]
func (kh *KserveHandler) GetGraph(c *gin.Context) {
	namespace := c.Query("namespace")
	name := c.Param("name")
	if namespace == "" || name == "" {
		c.Status(http.StatusBadRequest)
		_ = c.Error(fmt.Errorf("namespace and name must be provided"))
		return
	}
	graph := ksvcv1alpha1.InferenceGraph{}
	if err := kh.kc.Get(c.Request.Context(), client.ObjectKey{Name: name, Namespace: namespace}, &graph); err != nil {
		c.Status(http.StatusInternalServerError)
		_ = c.Error(err)
		return
	}
	dm, err := kh.getDeploymentMode(c.Request.Context(), graph.Status.DeploymentMode, graph.Annotations, nil)
	if err != nil {
		middleware.ErrorJson(c, err, "")
		return
	}

	ready := isvcGraphNodeStatusFalse
	for j := range graph.Status.Conditions {
		if graph.Status.Conditions[j].Type == "Ready" && graph.Status.Conditions[j].Status == "True" {
			ready = isvcGraphNodeStatusTrue
			break
		}
	}
	var sampleObj simpleObjectMap = make(map[string]*simpleObject)
	var ksvcObj *simpleObject
	if dm != ksvcconstants.Standard {
		ksvcObj = kh.getKsvcShow(c.Request.Context(), name, name, namespace, nil, sampleObj)
	}
	if dm == ksvcconstants.Knative {
		name = "knative-" + name
	} else {
		name = "kserve-router"
	}
	obj := sampleObj.AddNodes(name, namespace, "input", isvcGraphNodeStatusTrue, nil, ksvcObj)
	kh.node2Edge(namespace, ksvcv1alpha1.GraphRootNodeName, []*simpleObject{obj}, &graph.Spec.Nodes, sampleObj)
	nodes, edges := simpleObject2graphNode(sampleObj)
	middleware.SuccessJson(c, map[string]any{"kind": "single", "nodes": nodes, "edges": edges, "dm": dm, "status": ready, "graph": graph})
}

package kserve

import (
	"context"
	"strings"

	ksvcv1beta1 "github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	ksvcconstants "github.com/kserve/kserve/pkg/constants"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kserve-nexus/pkg/log"
	"github.com/kserve-nexus/pkg/utils"
)

type standardIsvc struct {
	ctx  context.Context
	kc   client.Client
	isvc *ksvcv1beta1.InferenceService

	// 涉及到的组件
	Gw  *GraphNode // isvc 对外 gateway
	Gwc *GraphNode // isvc 对外 gatewayclass
	Hr  *HttpRoute // isvc 对外 httproute
	Hrp *HttpRoute // isvc Predictor 对外 httproute
	Hrt *HttpRoute // isvc Transformer 对外 httproute
	Hre *HttpRoute // isvc Explainer 对外 httproute
}

// GetGateway 获取 gateway以及gatewayclass
func (s *standardIsvc) GetGateway(nodes *GraphNodeMap, namespace2name ...string) {
	var namespace, name string
	// 使用默认配置的网关信息
	if len(namespace2name) == 1 {
		namespace2names := strings.Split(namespace2name[0], "/")
		namespace, name = namespace2names[0], namespace2names[1]
	} else {
		namespace, name = namespace2name[0], namespace2name[1]
	}
	var gw = &gwapiv1.Gateway{}
	s.Gw = nodes.AddNodes(name, namespace, utils.GetCrdKey("gtw"), GraphNodeStatusTrue, nil)
	if err := s.kc.Get(s.ctx, client.ObjectKey{Name: name, Namespace: namespace}, gw); err != nil {
		log.Logger.Error(err, "获取 gateway 失败 ", "namespace", namespace, "name", name)
		s.Gw.SetStatus(GraphNodeStatusFalse)
		return
	}
	gwcName := string(gw.Spec.GatewayClassName)
	var gwc = &gwapiv1.GatewayClass{}
	s.Gwc = nodes.AddNodes(gwcName, namespace, utils.GetCrdKey("gc"), GraphNodeStatusTrue, nil)
	if err := s.kc.Get(s.ctx, client.ObjectKey{Name: gwcName}, gwc); err != nil {
		log.Logger.Error(err, "获取 gatewayclass 失败 ", "name", name)
		s.Gwc.SetStatus(GraphNodeStatusFalse)
	}
}

// ToGraphNode 转换成图结构
func (s *standardIsvc) ToGraphNode(ctx context.Context, nodes *GraphNodeMap, belong *GraphNode, parent ...*GraphNode) {
	head, _ := s.Hr.ToGraphNode(ctx, nodes, nil, s.Gw)
	s.Hrp.ToGraphNode(ctx, nodes, nil, s.Gw, head)
	if s.Hre != nil {
		s.Hre.ToGraphNode(ctx, nodes, nil, s.Gw, head)
	}
	if s.Hrt != nil {
		s.Hrt.ToGraphNode(ctx, nodes, nil, s.Gw, head)
	}
}

// Standard2GraphNode Standard 服务的相关组件
func Standard2GraphNode(ctx context.Context, kc client.Client, isvc *ksvcv1beta1.InferenceService, ingressConfig *ksvcv1beta1.IngressConfig) *GraphNodeMap {
	ac := ksvcconstants.AutoscalerClassHPA
	if ac1, ok := isvc.Annotations[ksvcconstants.AutoscalerClass]; ok {
		ac = ksvcconstants.AutoscalerClassType(ac1)
	}

	svc := make(map[string]*Service)
	s := &standardIsvc{
		ctx:  ctx,
		kc:   kc,
		isvc: isvc,

		Hr: NewHttpRoute(ctx, kc, svc, isvc.Name, isvc.Name, isvc.Namespace, ac),
	}
	nodes := make(GraphNodeMap)
	s.GetGateway(&nodes)
	// Predictor
	s.Hrp = NewHttpRoute(ctx, kc, svc, isvc.Name, ksvcconstants.PredictorServiceName(isvc.Name), isvc.Namespace, ac)
	// Transformer
	if isvc.Spec.Transformer != nil {
		s.Hrt = NewHttpRoute(ctx, kc, svc, isvc.Name, ksvcconstants.TransformerServiceName(isvc.Name), isvc.Namespace, ac)
	}
	// Explainer
	if isvc.Spec.Explainer != nil {
		s.Hrt = NewHttpRoute(ctx, kc, svc, isvc.Name, ksvcconstants.ExplainerServiceName(isvc.Name), isvc.Namespace, ac)
	}
	s.ToGraphNode(ctx, &nodes, nil)
	return &nodes
}

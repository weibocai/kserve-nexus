package kserve

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	ksvcv1beta1 "github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	ksvcconstants "github.com/kserve/kserve/pkg/constants"
	istioclientv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	knnetworkingv1alpha1 "knative.dev/networking/pkg/apis/networking/v1alpha1"
	knkpav1alpha1 "knative.dev/serving/pkg/apis/autoscaling/v1alpha1"
	knservingv1 "knative.dev/serving/pkg/apis/serving/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kserve-nexus/pkg/utils"
)

// GetKpaStatus 状态
func GetKpaStatus() GraphNodeStatus {
	return GraphNodeStatusTrue
}

// GetIstioGatewayStatus 状态
func GetIstioGatewayStatus(gw *istioclientv1beta1.Gateway) GraphNodeStatus {
	return GraphNodeStatusTrue
}

// GetRouteStatus 状态
func GetRouteStatus(route *knservingv1.Route) GraphNodeStatus {
	return GraphNodeStatusTrue
}

// GetConfigurationStatus 状态
func GetConfigurationStatus(cg *knservingv1.Configuration) GraphNodeStatus {
	return GraphNodeStatusTrue
}

// GetServerlessServiceStatus 状态
func GetServerlessServiceStatus() GraphNodeStatus {
	return GraphNodeStatusTrue
}

// GetVirtualServiceStatus 状态
func GetVirtualServiceStatus(vs *istioclientv1beta1.VirtualService) GraphNodeStatus {
	return GraphNodeStatusTrue
}

// GetKsvcStatus 获取服务状态
func GetKsvcStatus(ksvc *knservingv1.Service) GraphNodeStatus {
 if ksvc == nil ｛
  return GraphNodeStatusFalse
 ｝
	for _, kc := range ksvc.Status.Conditions {
		if kc.Status != corev1.ConditionTrue {
			return GraphNodeStatusFalse
		}
	}
	return GraphNodeStatusTrue
}

// VirtualService2GraphNode 生成对应的图结构
func VirtualService2GraphNode(ctx context.Context, kc client.Client, vs *istioclientv1beta1.VirtualService, name, namespace string, gwNodeMap map[string]*GraphNode, nodes *GraphNodeMap, belong *GraphNode, parent ...*GraphNode) (*GraphNode, *GraphNode) {
	tail := nodes.AddNodes(vs.Name, vs.Namespace, utils.GetCrdKey("vs"), GetVirtualServiceStatus(vs), belong)

	// 如果未找到，则设置成故障节点，且添加对应的路由
	if vs == nil {
		if strings.HasSuffix(name, "-mesh") {
			return nil, tail
		}
		for _, gw := range gwNodeMap {
			nodes.AddEdges(tail, "", gw)
		}
		return nil, tail
	}

	// 按照每个虚拟服务的路由配置，添加氟节点
	for _, gwName := range vs.Spec.Gateways {
		if gwName == "mesh" {
			continue
		}
		if _, ok := gwNodeMap[gwName]; !ok {
			gw := &istioclientv1beta1.Gateway{}
			if n, ns, err := utils.SplitName2GetObject(ctx, kc, gw, 1, 0, gwName, "/"); err != nil {
				gwNodeMap[gwName] = nodes.AddNodes(n, ns, utils.GetCrdKey("gw"), GetIstioGatewayStatus(gw), belong)
			} else {
				gwNodeMap[gwName] = nodes.AddNodes(n, ns, utils.GetCrdKey("gw"), GetIstioGatewayStatus(nil), belong)
			}
			nodes.AddEdges(tail, "", gwNodeMap[gwName])
		}
	}
	return nil, tail
}

// Knative 版本
type KnativeRevision struct {
	Name           string          // 服务名称
	Namespace      string          // 命名空间
	Actor          *corev1.Service // actor 服务
	Service        *corev1.Service // 服务
	PrivateService *corev1.Service // 私有服务
	Deployment     *appsv1.Deployment
	ServerLess     *knnetworkingv1alpha1.ServerlessService
	Kpa            *knkpav1alpha1.PodAutoscaler
}

// ToGraphNode 转成图结构
func (k *KnativeRevision) ToGraphNode(ctx context.Context, nodes *GraphNodeMap, belong *GraphNode, parent ...*GraphNode) (*GraphNode, *GraphNode) {
	kpa := nodes.AddNodes(k.Name, k.Namespace, utils.GetCrdKey("kpa"), GetKpaStatus(), belong)
	sl := nodes.AddNodes(k.Name, k.Namespace, utils.GetCrdKey("sl"), GetServerlessServiceStatus(), kpa)
	svc := nodes.AddNodes(k.Name, k.Namespace, utils.GetCrdKey("svc"), GetServiceStatus(k.Service), kpa, parent...)
	nodes.AddEdges(svc, "", sl)
	nodes.AddNodes(k.Name, k.Namespace, utils.GetCrdKey("svc"), GetServiceStatus(k.Actor), nil, svc)
	psvc := nodes.AddNodes(k.Name, k.Namespace, utils.GetCrdKey("svc"), GetServiceStatus(k.PrivateService), kpa, parent...)
	nodes.AddEdges(psvc, "", svc)
	deploy := nodes.AddNodes(k.Name, k.Namespace, utils.GetCrdKey("deploy"), GetDepStatus(k.Deployment), kpa, svc, psvc)
	return sl, deploy
}

// NewKnativeRevision 版本对应关系
func NewKnativeRevision(ctx context.Context, kc client.Client, name, namespace, obs string) *KnativeRevision {
	nameZero := make([]string, 5-len(obs))
	for i := range nameZero {
		nameZero[i] = "0"
	}
	name = name + "-" + strings.Join(nameZero, "") + obs
	// 节点伸缩
	kpa := &knkpav1alpha1.PodAutoscaler{}
	if err := kc.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, kpa); err != nil {
		kpa = nil
	}
	// 服务
	svc := &corev1.Service{}
	if err := kc.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, svc); err != nil {
		svc = nil
	}

	// 私有服务
	svcp := &corev1.Service{}
	if err := kc.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name + "private"}, svcp); err != nil {
		svcp = nil
	}
	dep := &appsv1.Deployment{}
	if err := kc.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name + "deployment"}, dep); err != nil {
		dep = nil
	}

	// 服务生成器
	serverless := &knnetworkingv1alpha1.ServerlessService{}
	if err := kc.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, serverless); err != nil {
		serverless = nil
	}

	return &KnativeRevision{
		Name: name, Namespace: namespace, Service: svc, PrivateService: svcp, Deployment: dep, ServerLess: serverless, Kpa: kpa,
	}
}

// KnativeGateway 网络相关节点
type KnativeGateway struct {
	Name             string // 服务名称
	Namespace        string // 命名空间
	ServiceName      string
	ServiceNamespace string
	ServiceFullName  string
	Kind             string // 网关类型

	Service *corev1.Service
	Gateway *istioclientv1beta1.Gateway
}

// SetGateway 查找gateway
func (kg *KnativeGateway) SetGateway(ctx context.Context, kc client.Client) {
	if kg.Name == "" {
		return
	}
	if kg.Namespace == "" {
		kg.ServiceName = "default"
	}
	gw := &istioclientv1beta1.Gateway{}
	if err := kc.Get(ctx, client.ObjectKey{Namespace: kg.Namespace, Name: kg.Name}, gw); err == nil {
		kg.Gateway = gw
	}
}

// SetService 查找服务节点
func (kg *KnativeGateway) SetService(ctx context.Context, kc client.Client) {
	svc := &corev1.Service{}
	if name, namespace, err := utils.SplitName2GetObject(ctx, kc, svc, 0, 1, kg.ServiceFullName, "."); err != nil {
		kg.Service = svc
		kg.ServiceName = name
		kg.ServiceNamespace = namespace
	}
}

// KnativeService ksvc 管理器
type KnativeService struct {
	Name          string                     // 服务名称
	Namespace     string                     // 命名空间
	Ksvc          *knservingv1.Service       // ksvc
	Configuration *knservingv1.Configuration // 配置
	Route         *knservingv1.Route         // 路由
	Service       *corev1.Service            // 服务
	Reversion     []*KnativeRevision

	// 网络
	IngressClass          string                             // 网关类型
	VirtualServiceMesh    *istioclientv1beta1.VirtualService // 虚拟服务:集群内
	VirtualServiceIngress *istioclientv1beta1.VirtualService // 虚拟服务:集群外
	LocalGateway          map[string]*KnativeGateway         // 本地网关
	ExternalGateway       map[string]*KnativeGateway         // 外网网关
}

// SetIstioGateway istio 相关路由查找
func (ks *KnativeService) SetIstioGateway(ctx context.Context, kc client.Client, name, namespace string) {
	istioConfig := &corev1.ConfigMap{}
	lkg := make([]KnativeGateway, 0)
	ekg := make([]KnativeGateway, 0)
	ks.LocalGateway = make(map[string]*KnativeGateway)
	ks.ExternalGateway = make(map[string]*KnativeGateway)
	if err := kc.Get(ctx, client.ObjectKey{Namespace: "knative-serving", Name: "istio-config"}, istioConfig); err == nil {
		if locakGateways, ok := istioConfig.Data["local-gateways"]; ok {
			_ = json.Unmarshal([]byte(locakGateways), &lkg)
		}
		if externalGateways, ok := istioConfig.Data["external-gateways"]; ok {
			_ = json.Unmarshal([]byte(externalGateways), &ekg)
		}
	}
	if len(lkg) == 0 {
		lkg = append(lkg, KnativeGateway{Name: "knative-local-gateway", Namespace: "knative-serving", Kind: "knative-local-gateway.istio-system.svc.cluster.local"})
	}
	if len(ekg) == 0 {
		ekg = append(ekg, KnativeGateway{Name: "knative-external-gateway", Namespace: "knative-serving", Kind: "istio-ingressgateway.istio-system.svc.cluster.local"})
	}
	for i := range lkg {
		lkg[i].SetGateway(ctx, kc)
		lkg[i].SetService(ctx, kc)
		lkg[i].Kind = "istio"
		ks.LocalGateway[lkg[i].Namespace+"/"+lkg[i].Name] = &lkg[i]
	}
	for i := range ekg {
		ekg[i].SetGateway(ctx, kc)
		ekg[i].SetService(ctx, kc)
		ekg[i].Kind = "istio"
		ks.ExternalGateway[ekg[i].Namespace+"/"+ekg[i].Name] = &ekg[i]
	}
}

// SetIngress 获取网关相关信息
func (ks *KnativeService) SetIngress(ctx context.Context, kc client.Client, name, namespace string) {
	config := &corev1.ConfigMap{}
	// 默认
	ks.IngressClass = "istio.ingress.networking.knative.dev"
	if err := kc.Get(ctx, client.ObjectKey{Namespace: "knative-serving", Name: "config-network"}, config); err == nil {
		if ingressClass, ok := config.Data["ingress.class"]; ok {
			ks.IngressClass = ingressClass
		}
	}

	switch ks.IngressClass {
	case "istio.ingress.networking.knative.dev":
		mesh, ingress := &istioclientv1beta1.VirtualService{}, &istioclientv1beta1.VirtualService{}
		if err := kc.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name + "-mesh"}, mesh); err == nil {
			ks.VirtualServiceMesh = mesh
		}
		if err := kc.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name + "-ingress"}, ingress); err == nil {
			ks.VirtualServiceIngress = ingress
		}
	}
}

// ToGraphNode 转换成图结构
func (ks *KnativeService) ToGraphNode(ctx context.Context, kc client.Client, nodes *GraphNodeMap, belong *GraphNode, parent ...*GraphNode) (*GraphNode, *GraphNode) {
	ksvc := nodes.AddNodes(ks.Name, ks.Namespace, utils.GetCrdKey("ksvc"), GetKsvcStatus(ks.Ksvc), belong, parent...)
	svcNode := nodes.AddNodes(ks.Name, ks.Namespace, utils.GetCrdKey("svc"), GetServiceStatus(ks.Service), ksvc)
	lgwNodeMap := make(map[string]*GraphNode)
	for k, v := range ks.LocalGateway {
		gwSvc := nodes.AddNodes(v.ServiceName, v.ServiceNamespace, utils.GetCrdKey("svc"), GetServiceStatus(v.Service), nil)
		gwNode := nodes.AddNodes(v.Name, v.Namespace, utils.GetCrdKey("gw"), GetIstioGatewayStatus(v.Gateway), nil, svcNode, gwSvc)
		lgwNodeMap[k] = gwNode
	}
	routeNode := nodes.AddNodes(ks.Name, ks.Namespace, utils.GetCrdKey("route"), GetRouteStatus(ks.Route), ksvc)
	configNode := nodes.AddNodes(ks.Name, ks.Namespace, utils.GetCrdKey("config"), GetConfigurationStatus(ks.Configuration), ksvc, routeNode)
	_, mesh := VirtualService2GraphNode(ctx, kc, ks.VirtualServiceMesh, ks.Name, ks.Namespace, lgwNodeMap, nodes, ksvc, configNode)
	_, ingress := VirtualService2GraphNode(ctx, kc, ks.VirtualServiceIngress, ks.Name, ks.Namespace, lgwNodeMap, nodes, ksvc, configNode)
	for i := range ks.Reversion {

		ks.Reversion[i].ToGraphNode(ctx, nodes, ksvc, mesh, ingress)
	}
	return ksvc, nil

}

// NewKnativeService 初始化ksvc
func NewKnativeService(ctx context.Context, kc client.Client, name, namespace string) *KnativeService {
	var ksvc = &knservingv1.Service{}
	if err := kc.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, ksvc); err != nil {
		return &KnativeService{Name: name, Namespace: namespace}
	}

	var config = &knservingv1.Configuration{}
	if err := kc.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, config); err != nil {
		config = nil
	}
	var route = &knservingv1.Route{}
	if err := kc.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, route); err != nil {
		route = nil
	}
	// 服务
	svc := &corev1.Service{}
	if err := kc.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, svc); err != nil {
		svc = nil
	}
	// 获取当前版本号
	obs := int(ksvc.Status.ObservedGeneration)
	reversion := make([]*KnativeRevision, obs)
	for i := 1; i <= obs; i++ {
		reversion[i-1] = NewKnativeRevision(ctx, kc, name, namespace, strconv.Itoa(obs))
	}
	ks := &KnativeService{
		Name: name, Namespace: namespace, Ksvc: ksvc, Configuration: config, Route: route, Service: svc, Reversion: reversion,
	}
	ks.SetIngress(ctx, kc, name, namespace)
	return ks
}

type knateieIsvc struct {
	ctx  context.Context
	kc   client.Client
	isvc *ksvcv1beta1.InferenceService

	Svc *corev1.Service                    // 对外统一出口
	Vis *istioclientv1beta1.VirtualService // 对外统一出口的虚拟服务

	Ksp *KnativeService // 推理服务
	Kst *KnativeService // trTransformer 服务
	Kse *KnativeService // explainer 服务
}

// KnativeIsvc2GraphNode 将 Knative Isvc 转换成 GraphNode
func KnativeIsvc2GraphNode(ctx context.Context, kc client.Client, isvc *ksvcv1beta1.InferenceService, ingressConfig *ksvcv1beta1.IngressConfig) GraphNodeMap {
	nodes := GraphNodeMap{Edges: make(map[string]*GraphEdge), Nodes: make(map[string]*GraphNode)}
	s := &knateieIsvc{
		ctx: ctx, kc: kc, isvc: isvc,
	}
	s.Svc = &corev1.Service{}
	if err := kc.Get(ctx, types.NamespacedName{Name: isvc.Name, Namespace: isvc.Namespace}, s.Svc); err != nil {
		s.Svc = nil
	}
	svcNode := nodes.AddNodes(isvc.Name, isvc.Namespace, utils.GetCrdKey("svc"), GetServiceStatus(s.Svc), nil)
	s.Vis = &istioclientv1beta1.VirtualService{}
	if err := kc.Get(ctx, types.NamespacedName{Name: isvc.Name, Namespace: isvc.Namespace}, s.Vis); err != nil {
		s.Vis = nil
	}
	_, visNode := VirtualService2GraphNode(ctx, kc, s.Vis, isvc.Name, isvc.Namespace, map[string]*GraphNode{}, &nodes, nil, svcNode)

	s.Ksp = NewKnativeService(ctx, kc, ksvcconstants.PredictorServiceName(isvc.Name), isvc.Namespace)
	s.Ksp.ToGraphNode(ctx, kc, &nodes, nil, visNode)
	if isvc.Spec.Transformer != nil {
		s.Kst = NewKnativeService(ctx, kc, ksvcconstants.TransformerServiceName(isvc.Name), isvc.Namespace)
		s.Kst.ToGraphNode(ctx, kc, &nodes, nil, visNode)

	}
	if isvc.Spec.Explainer != nil {
		s.Kse = NewKnativeService(ctx, kc, ksvcconstants.ExplainerServiceName(isvc.Name), isvc.Namespace)
		s.Kse.ToGraphNode(ctx, kc, &nodes, nil, visNode)

	}
	return nodes
}

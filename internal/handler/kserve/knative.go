package kserve

import (
	"context"
	"strconv"
	"strings"

	ksvcv1beta1 "github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	istioclientv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	knkpav1alpha1 "knative.dev/networking/pkg/apis/networking/v1alpha1"
	knnetworkingv1alpha1 "knative.dev/networking/pkg/apis/networking/v1alpha1"
	knservingv1 "knative.dev/serving/pkg/apis/serving/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kserve-nexus/pkg/log"
)

type KnativeRevision struct {
	Name           string          // 服务名称
	Namespace      string          // 命名空间
	Service        *corev1.Service // 服务
	PrivateService *corev1.Service // 私有服务
	Deployment     *appsv1.Deployment
	ServerLess     *knnetworkingv1alpha1.ServerlessService
	Kpa            *knkpav1alpha1.ServerlessService
}

// NewKnativeRevision 版本对应关系
func NewKnativeRevision(ctx context.Context, kc client.Client, name, namespace, obs string) *KnativeRevision {
	nameZero := make([]string, 5-len(obs))
	for i := range nameZero {
		nameZero[i] = "0"
	}
	name = name + strings.Join(nameZero, "") + obs
	// 节点伸缩
	kpa := &knkpav1alpha1.ServerlessService{}
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
	if err := kc.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, dep); err != nil {
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

// KnativeServiceNet ksvc 网络相关组件
type KnativeServiceNet struct {
	Gw             *GraphNode
	Gwc            *GraphNode
	IstioSvc       *GraphNode
	VirtualService *istioclientv1beta1.VirtualService
}

// KnativeService ksvc 管理器
type KnativeService struct {
	Name          string // 服务名称
	Namespace     string // 命名空间
	Ksvc          *knservingv1.Service
	Configuration *knservingv1.Configuration
	Route         *knservingv1.Route
	Service       *corev1.Service // 服务
	Reversion     []*KnativeRevision

	// 网络
	IngressClass string // 网关类型
}

// GetKsvcStatus 获取服务状态
func (svc *KnativeService) GetKsvcStatus(ksvc *knservingv1.Service) GraphNodeStatus {
	for _, kc := range ksvc.Status.Conditions {
		if kc.Status != corev1.ConditionTrue {
			return GraphNodeStatusFalse
		}
	}
	return GraphNodeStatusTrue
}

// SetKsvcIngress 获取网关相关信息
func (svc *KnativeService) SetKsvcIngress(ctx context.Context, kc client.Client, name, namespace string) {
	config := &corev1.ConfigMap{}
	// 默认
	svc.IngressClass = "istio.ingress.networking.knative.dev"
	if err := kc.Get(ctx, client.ObjectKey{Namespace: "knative-serving", Name: "config-network"}, config); err != nil {
		log.Logger.Error(err, "config-network.ConfigMap.knative-serving 获取异常")
	} else {
		if ic, ok := config.Data["ingress.class"]; ok {
			svc.IngressClass = ic
		}
	}
	switch svc.IngressClass {
	case "istio.ingress.networking.knative.dev":

	}
}

// NewKnativeService 初始化ksvc
func NewKnativeService(ctx context.Context, kc client.Client, name, namespace string) *KnativeService {
	var ksvc = &knservingv1.Service{}
	if err := kc.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, ksvc); err != nil {
		return &KnativeService{}
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
	ks.SetKsvcIngress(ctx, kc, name, namespace)
	return ks
}

type knateieIsvc struct {
	ctx  context.Context
	kc   client.Client
	isvc *ksvcv1beta1.InferenceService

	// 涉及到的组件
	Ksp *KnativeRevision
}

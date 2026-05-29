package kserve

import (
	"context"
	"strings"

	ksvcconstants "github.com/kserve/kserve/pkg/constants"
	istioclientv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kserve-nexus/internal/handler"
	"github.com/kserve-nexus/pkg/log"
)

func getHTTPRouteStatus(hr *gwapiv1.HTTPRoute) GraphNodeStatus {
	if hr == nil {
		return GraphNodeStatusFalse
	}
	if len(hr.Status.Parents) == 0 {
		return GraphNodeStatusFalse
	}
	for _, parent := range hr.Status.Parents {
		for _, c := range parent.Conditions {
			switch c.Type {
			case string(gwapiv1.RouteConditionAccepted):
				if c.Status != metav1.ConditionTrue {
					return GraphNodeStatusFalse
				}
			case string(gwapiv1.RouteConditionResolvedRefs):
				if c.Status != metav1.ConditionTrue {
					return GraphNodeStatusFalse
				}
			}
		}
	}
	return GraphNodeStatusTrue
}

// 网关相关
func (kh *Handler) showGateway(ctx context.Context, pr []gwapiv1.ParentReference, theNode *GraphNode, nodes GraphNodeMap) {
	for i := range pr {
		nss := "default"
		ns := pr[i].Namespace
		if ns != nil {
			nss = string(*ns)
		}
		kind := pr[i].Kind
		if kind != nil && strings.ToLower(string(*kind)) != "gateway" {
			continue
		}
		name, gwcName := string(pr[i].Name), "未知"
		var gwcObject = nodes.AddNodes(gwcName, "all", handler.GetCrdKey("gc"), GraphNodeStatusFalse, nil)
		var gwObject = nodes.AddNodes(name, nss, handler.GetCrdKey("gtw"), GraphNodeStatusFalse, nil, gwcObject)
		theNode.AddParent(gwObject)
		var gw = &gwapiv1.Gateway{}
		if err := kh.kc.Get(ctx, types.NamespacedName{Namespace: nss, Name: name}, gw); err != nil {
			continue
		}
		gwcName = string(gw.Spec.GatewayClassName)
		gwObject.SetStatus(GraphNodeStatusTrue)
		gwcObject.SetName(gwcName)
		theNode.AddParent(gwObject)
		var gwc = &gwapiv1.GatewayClass{}
		if err := kh.kc.Get(ctx, types.NamespacedName{Namespace: nss, Name: string(gw.Spec.GatewayClassName)}, gwc); err != nil {

			continue
		}
		gwcObject.SetStatus(GraphNodeStatusTrue)
	}
}

// 获取网关
func (kh *Handler) getGateway(ctx context.Context, ingress string) (*gwapiv1.HTTPRoute, error) {
	gs := strings.Split(ingress, "/")
	namespace, name := gs[0], gs[1]
	var gw = &gwapiv1.Gateway{}
	if err := kh.kc.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, gw); err != nil {
		return nil, err
	}
	hr := &gwapiv1.HTTPRoute{}
	if err := kh.kc.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, hr); err != nil {
		return nil, err
	}
	return hr, nil
}

// getHttpRoute 获取路由
func (kh *Handler) getHttpRoute(ctx context.Context, name, namespace string) (*gwapiv1.HTTPRoute, error) {
	hr := &gwapiv1.HTTPRoute{}
	if err := kh.kc.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, hr); err != nil {
		return nil, err
	}
	return hr, nil
}

// istio 虚拟服务
func (kh *Handler) getVirtualServices(ctx context.Context, name, namespace string) (*istioclientv1beta1.VirtualService, error) {
	vir := istioclientv1beta1.VirtualService{}
	if err := kh.kc.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &vir); err != nil {
		return nil, err
	}
	return &vir, nil
}

// 路由展示链路
func (kh *Handler) getHTTPRouteShow(ctx context.Context, isvcName, name, namespace string, ac ksvcconstants.AutoscalerClassType, nodes GraphNodeMap) (*GraphNode, *GraphNode) {
	hr := &gwapiv1.HTTPRoute{}
	hrObj := nodes.AddNodes(name, namespace, handler.GetCrdKey("hr"), GraphNodeStatusFalse, nil)
	if err := kh.kc.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, hr); err != nil {
		hrObj.SetStatus(GraphNodeStatusFalse)
		log.Logger.Error(err, "获取 HTTPRoute 失败", "Namespace", namespace, "Name", name)
		return hrObj, nil
	}
	hrObj.SetStatus(getHTTPRouteStatus(hr))
	kh.showGateway(ctx, hr.Spec.ParentRefs, hrObj, nodes)
	var svcObject *GraphNode
	for i := range hr.Spec.Rules {
		r := &hr.Spec.Rules[i]
		for j := range r.BackendRefs {
			b := &r.BackendRefs[j]
			if b.Kind == nil {
				log.Logger.Info("参数错误", "kind", b.Kind, "namespace", b.Namespace, "name", b.Name)
				continue
			}
			switch *b.Kind {
			case "Service":
				svcObject = kh.getSvcDepShow(ctx, isvcName, string(b.Name), namespace, hrObj, ac, nodes)
			case "InferencePool":
				kh.getInferencePoolsShow(ctx, isvcName, string(b.Name), namespace, hrObj, nodes)
			default:
				log.Logger.Info("暂未集成", "kind", b.Kind, "namespace", b.Namespace, "name", b.Name)
			}
		}
	}
	return hrObj, svcObject
}

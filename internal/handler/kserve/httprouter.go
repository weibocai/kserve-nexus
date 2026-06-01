package kserve

import (
	"context"

	"github.com/bytedance/gopkg/util/logger"
	"github.com/kserve-nexus/pkg/log"
	"github.com/kserve-nexus/pkg/utils"
	ksvcconstants "github.com/kserve/kserve/pkg/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// HttpRoute 路由相关资源
type HttpRoute struct {
	Hr                  *gwapiv1.HTTPRoute                // isvc 对外 httproute
	Svc                 map[string]*Service               // svc 相关信息
	AutoscalerClassType ksvcconstants.AutoscalerClassType // 自动伸缩类型

	Name      string
	Namespace string
	IsvcName  string
}

// ToGraphNode 转换成图结构节点
func (h *HttpRoute) ToGraphNode(ctx context.Context, nodes *GraphNodeMap, belong *GraphNode, parent ...*GraphNode) (*GraphNode, *GraphNode) {
	hr := nodes.AddNodes(h.Name, h.Namespace, utils.GetCrdKey("hr"), GetHTTPRouteStatus(h.Hr), belong, parent...)
	for i := range h.Hr.Spec.Rules {
		r := &h.Hr.Spec.Rules[i]
		for j := range r.BackendRefs {
			b := &r.BackendRefs[j]
			if b.Kind == nil {
				log.Logger.Info("参数错误", "kind", b.Kind, "namespace", b.Namespace, "name", b.Name)
				continue
			}
			switch *b.Kind {
			case "Service":
				svcName := string(b.Name)
				if svc, ok := h.Svc[svcName]; ok {
					nodes.AddNodes(svcName, h.Namespace, utils.GetCrdKey("svc"), GraphNodeStatusFalse, hr)
					continue
				} else {
					svc.ToGraphNode(ctx, nodes, nil, hr)
				}
			case "InferencePool":

			default:
				log.Logger.Info("暂未集成", "kind", b.Kind, "namespace", b.Namespace, "name", b.Name)
			}
		}
	}
	return hr, nil
}

// NewHttpRoute 创建 httproute
func NewHttpRoute(ctx context.Context, kc client.Client, svc map[string]*Service, isvcName, name, namespace string, ac ksvcconstants.AutoscalerClassType) *HttpRoute {
	var hr = &gwapiv1.HTTPRoute{}
	if err := kc.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, hr); err != nil {
		logger.Error(err, "获取 httproute 失败 ", "name", name)
		return nil
	}
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
				svcName := string(b.Name)
				if _, ok := svc[svcName]; ok {
					continue
				}
				svc[svcName] = NewService(ctx, kc, isvcName, string(b.Name), namespace, ac)
			case "InferencePool":

			default:
				log.Logger.Info("暂未集成", "kind", b.Kind, "namespace", b.Namespace, "name", b.Name)
			}
		}
	}
	return &HttpRoute{
		Hr:                  hr,
		Svc:                 svc,
		AutoscalerClassType: ac,
	}
}

// GetHTTPRouteStatus 获取 httproute 状态
func GetHTTPRouteStatus(hr *gwapiv1.HTTPRoute) GraphNodeStatus {
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

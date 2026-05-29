package kserve

import (
	"context"
	"strings"

	"github.com/bytedance/gopkg/util/logger"
	ksvcv1beta1 "github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kserve-nexus/pkg/log"
)

type standardIsvc struct {
	ctx context.Context
	kc  client.Client
	cs  *kubernetes.Clientset

	gw   *gwapiv1.Gateway      // isvc 对外 gateway
	gwc  *gwapiv1.GatewayClass // isvc 对外 gatewayclass
	hr   *gwapiv1.HTTPRoute    // isvc 对外 httproute
	hrp  *gwapiv1.HTTPRoute    // isvc Predictor 对外 httproute
	hrt  *gwapiv1.HTTPRoute    // isvc Transformer 对外 httproute
	hre  *gwapiv1.HTTPRoute    // isvc Explainer 对外 httproute
	isvc *ksvcv1beta1.InferenceService
	svc  map[string]*SvcDepPod
}

// GetGateway 获取 gateway以及gatewayclass
func (s *standardIsvc) GetGateway(namespace2name ...string) {
	var namespace, name string
	if len(namespace2name) == 1 {
		namespace2names := strings.Split(namespace2name[0], "/")
		namespace, name = namespace2names[0], namespace2names[1]
	} else {
		namespace, name = namespace2name[0], namespace2name[1]
	}
	var gw = &gwapiv1.Gateway{}
	if err := s.kc.Get(s.ctx, client.ObjectKey{Name: name, Namespace: namespace}, gw); err != nil {
		log.Logger.Error(err, "获取 gateway 失败 ", "namespace", namespace, "name", name)
		return
	}
	s.gw = gw
	gwcName := string(gw.Spec.GatewayClassName)
	var gwc = &gwapiv1.GatewayClass{}
	if err := s.kc.Get(s.ctx, client.ObjectKey{Name: gwcName}, gwc); err != nil {
		log.Logger.Error(err, "获取 gatewayclass 失败 ", "name", name)
	} else {
		s.gwc = gwc
	}
}

// GetHttpRoute 获取 httproute
func (s *standardIsvc) GetHttpRoute(name string) *gwapiv1.HTTPRoute {
	var hr = &gwapiv1.HTTPRoute{}
	if err := s.kc.Get(s.ctx, client.ObjectKey{Name: name, Namespace: s.isvc.Namespace}, hr); err != nil {
		logger.Error(err, "获取 httproute 失败 ", "name", name)
		return nil
	}
	return hr
}

// GetSvc 获取服务的相关信息
func (s *standardIsvc) GetSvc(name string) *SvcDepPod {
	if obj, ok := s.svc[name]; ok {
		return obj
	}
	s.svc[name] = GetSvcDep(s.ctx, s.kc, name, s.isvc.Namespace)
	return s.svc[name]
}

func (s *standardIsvc) DealHttpRoute(hr *gwapiv1.HTTPRoute) {

}

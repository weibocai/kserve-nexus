package kserve

import (
	"context"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	ksvcconstants "github.com/kserve/kserve/pkg/constants"
	istioclientv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	knnetworkingv1alpha1 "knative.dev/networking/pkg/apis/networking/v1alpha1"
	knservingv1 "knative.dev/serving/pkg/apis/serving/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kserve-nexus/internal/handler"
	"github.com/kserve-nexus/pkg/log"
)

type knativeRevision struct {
	Namespace      string
	RevisionName   string                                  `json:"revisionName"`
	Revision       *knservingv1.Revision                   `json:"revision"`
	Config         *knservingv1.Configuration              `json:"config"`
	ServerLess     *knnetworkingv1alpha1.ServerlessService `json:"serverless"`
	Service        *corev1.Service                         `json:"service"`
	Deployment     *appsv1.Deployment                      `json:"deployment"`
	PrivateService *corev1.Service                         `json:"privateService"`
}

func (k *knativeRevision) RevisionStatus() isvcGraphNodeStatus {
	if k.Revision == nil {
		return isvcGraphNodeStatusFalse
	}
	for index := range k.Revision.Status.Conditions {
		if k.Revision.Status.Conditions[index].Type == "Ready" {
			return isvcGraphNodeStatus(k.Revision.Status.Conditions[index].Status)
		}
	}
	return isvcGraphNodeStatusFalse
}

func (k *knativeRevision) ConfigStatus() isvcGraphNodeStatus {
	if k.Config == nil {
		return isvcGraphNodeStatusFalse
	}
	for index := range k.Config.Status.Conditions {
		if k.Config.Status.Conditions[index].Type == "Ready" {
			return isvcGraphNodeStatus(k.Config.Status.Conditions[index].Status)
		}
	}
	return isvcGraphNodeStatusFalse
}

func (k *knativeRevision) ServerLessStatus() isvcGraphNodeStatus {
	if k.ServerLess == nil {
		return isvcGraphNodeStatusFalse
	}
	for index := range k.ServerLess.Status.Conditions {
		if k.ServerLess.Status.Conditions[index].Type == "Ready" {
			return isvcGraphNodeStatus(k.ServerLess.Status.Conditions[index].Status)
		}
	}
	return isvcGraphNodeStatusFalse
}

func (k *knativeRevision) ToSampleObject(ksvcObj *simpleObject, nodes simpleObjectMap) {
	rObj := nodes.AddNodes(k.RevisionName, k.Namespace, handler.GetCrdKey("revision"), k.RevisionStatus(), ksvcObj)
	nodes.AddNodes(k.RevisionName, k.Namespace, handler.GetCrdKey("kConfig"), k.ConfigStatus(), ksvcObj, rObj)
	slObj := nodes.AddNodes(k.RevisionName, k.Namespace, handler.GetCrdKey("kLessService"), k.ServerLessStatus(), ksvcObj, rObj)
	sStatus := isvcGraphNodeStatusTrue
	if k.Service == nil {
		sStatus = isvcGraphNodeStatusFalse
	}
	sObj := nodes.AddNodes(k.RevisionName, k.Namespace, handler.GetCrdKey("svc"), sStatus, ksvcObj, slObj)
	nodes.AddNodes(k.RevisionName, k.Namespace, handler.GetCrdKey("deployment"), getDepStatus(k.Deployment), ksvcObj, sObj)
	spStatus := isvcGraphNodeStatusTrue
	if k.PrivateService == nil {
		spStatus = isvcGraphNodeStatusFalse
	}
	nodes.AddNodes(k.RevisionName+"-private", k.Namespace, handler.GetCrdKey("svc"), spStatus, ksvcObj, slObj)
}

// KnativeRevision 相关组件组合成一个结构体，便于后续的解析
func newKnativeRevision(ctx context.Context, kc client.Client, ksvc *knservingv1.Service, name, namespace string) []*knativeRevision {
	ksvcLabels := client.MatchingLabelsSelector{Selector: labels.SelectorFromSet(map[string]string{"serving.knative.dev/service": name, "serving.knative.dev/serviceUID": string(ksvc.GetUID())})}
	ksvcOptions := &client.ListOptions{Namespace: namespace, LabelSelector: ksvcLabels}
	revisions := &knservingv1.RevisionList{}
	revisionsMap := make(map[string]*knativeRevision)
	if err := kc.List(ctx, revisions, ksvcOptions); err == nil {
		for index := range revisions.Items {
			revisionsMap[revisions.Items[index].Name] = &knativeRevision{
				RevisionName: revisions.Items[index].Name, Revision: &revisions.Items[index],
			}
		}
	}

	svc := &corev1.ServiceList{}
	if err := kc.List(ctx, svc, ksvcOptions); err == nil {
		for index := range svc.Items {
			rid, ok := svc.Items[index].GetAnnotations()["serving.knative.dev/revisionUID"]
			if !ok {
				continue
			}
			kr, ok1 := revisionsMap[rid]
			revisionName, ok2 := svc.Items[index].GetAnnotations()["serving.knative.dev/revision"]
			if !ok1 && !ok2 {
				continue
			}
			if !ok1 {
				revisionsMap[rid] = &knativeRevision{RevisionName: revisionName}
				kr, ok1 = revisionsMap[rid]
			}
			if svc.Items[index].Name == revisionName {
				kr.Service = &svc.Items[index]
			} else {
				kr.PrivateService = &svc.Items[index]
			}
			dep := &appsv1.Deployment{}
			if err := kc.Get(ctx, types.NamespacedName{Namespace: namespace, Name: revisionName + "-deployment"}, dep); err == nil {
				kr.Deployment = dep
			}
		}
	}

	svcLess := &knnetworkingv1alpha1.ServerlessServiceList{}
	if err := kc.List(ctx, svcLess, ksvcOptions); err == nil {
		for index := range svcLess.Items {
			rid, ok := svcLess.Items[index].GetAnnotations()["serving.knative.dev/revisionUID"]
			if !ok {
				continue
			}
			kr, ok1 := revisionsMap[rid]
			revisionName, ok2 := svcLess.Items[index].GetAnnotations()["serving.knative.dev/revision"]
			if !ok1 && !ok2 {
				continue
			}
			if !ok1 {
				revisionsMap[rid] = &knativeRevision{RevisionName: revisionName}
				kr, ok1 = revisionsMap[rid]
			}
			kr.ServerLess = &svcLess.Items[index]
		}
	}

	config := &knservingv1.ConfigurationList{}
	if err := kc.List(ctx, config, ksvcOptions); err == nil {
		for index := range config.Items {
			rid, ok := config.Items[index].GetAnnotations()["serving.knative.dev/revisionUID"]
			if !ok {
				continue
			}
			kr, ok1 := revisionsMap[rid]
			revisionName, ok2 := config.Items[index].GetAnnotations()["serving.knative.dev/revision"]
			if !ok1 && !ok2 {
				continue
			}
			if !ok1 {
				revisionsMap[rid] = &knativeRevision{RevisionName: revisionName}
				kr, ok1 = revisionsMap[rid]
			}
			kr.Config = &config.Items[index]
		}
	}
	kv := make([]*knativeRevision, len(revisionsMap))
	index := 0
	for k := range revisionsMap {
		kv[index] = revisionsMap[k]
		index++
	}
	return kv
}

func (kh *Handler) getVirtualServicesShow(ctx context.Context, name, namespace string, nodes simpleObjectMap) *simpleObject {
	vr, err := kh.getVirtualServices(ctx, name, namespace)
	virObject := nodes.AddNodes(name, namespace, handler.GetCrdKey("vs"), isvcGraphNodeStatusFalse, nil)
	if err != nil {
		log.Logger.Error(err, "获取virtualservices 失败 ", "namespace", namespace, "name", name)
		virObject.SetStatus(isvcGraphNodeStatusFalse)
		return virObject
	}
	for i := range vr.Spec.Gateways {
		gw := strings.Split(vr.Spec.Gateways[i], "/")
		if len(gw) != 2 {
			continue
		}
		gwns, gwn := gw[0], gw[1]
		var gateway = &istioclientv1beta1.Gateway{}
		var gwIstio = &corev1.ServiceList{}
		gwObject := nodes.AddNodes(gwn, gwns, handler.GetCrdKey("gw"), isvcGraphNodeStatusTrue, nil)
		virObject.AddParent(gwObject)
		if err = kh.kc.Get(ctx, client.ObjectKey{Name: gwn, Namespace: gwns}, gateway); err != nil {
			log.Logger.Error(err, "获取virtualservices gateway 失败 ", "namespace", namespace, "name", name)
			gwObject.SetStatus(isvcGraphNodeStatusFalse)
			continue
		}
		ls := &metav1.LabelSelector{MatchLabels: gateway.Spec.Selector}
		l, _ := metav1.LabelSelectorAsSelector(ls)
		if err = kh.kc.List(ctx, gwIstio, &client.ListOptions{Namespace: "istio-system", LabelSelector: l}); err != nil {
			continue
		}
		kind := reflect.TypeOf((*corev1.Service)(nil)).Elem().Name()
		for j := range gwIstio.Items {
			gwIstioObj := nodes.AddNodes(gwIstio.Items[j].Name, "istio-system", kind, isvcGraphNodeStatusTrue, nil)
			gwObject.AddParent(gwIstioObj)
		}
	}
	return virObject
}

func (kh *Handler) getKsvcStatus(ksvc *knservingv1.Service) isvcGraphNodeStatus {
	for _, kc := range ksvc.Status.Conditions {
		if kc.Status != corev1.ConditionTrue {
			return isvcGraphNodeStatusFalse
		}
	}
	return isvcGraphNodeStatusTrue
}

func (kh *Handler) getKsvc(ctx context.Context, name, isvcName, namespace string) (*knservingv1.Service, *corev1.Service, []*knativeRevision) {
	ksvc := &knservingv1.Service{}
	if err := kh.kc.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, ksvc); err != nil {
		log.Logger.Error(err, "获取ksvc 失败", "namespace", namespace, "name", name, "isvc", isvcName)
		return nil, nil, nil
	}
	var ksvcSvc = &corev1.Service{}
	if err := kh.kc.Get(ctx, types.NamespacedName{Namespace: namespace, Name: isvcName}, ksvcSvc); err != nil {
		log.Logger.Error(err, "获取ksvc svc 失败", "namespace", namespace, "name", name, "isvc", isvcName)
	}
	if n, ok := ksvcSvc.Annotations["serving.knative.dev/service"]; !ok || n != isvcName {
		ksvcSvc = nil
	}

	ksvcRev := newKnativeRevision(ctx, kh.kc, ksvc, name, namespace)
	return ksvc, ksvcSvc, ksvcRev
}

func (kh *Handler) getKsvcShow(ctx context.Context, name, isvcName, namespace string, parent *simpleObject, nodes simpleObjectMap) *simpleObject {
	ksvc, ksvcSvc, ksvcRev := kh.getKsvc(ctx, name, isvcName, namespace)
	virMesh := kh.getVirtualServicesShow(ctx, name+"-mesh", namespace, nodes)
	virIngress := kh.getVirtualServicesShow(ctx, name+"-ingress", namespace, nodes)
	ksvcObj := nodes.AddNodes(name, namespace, handler.GetCrdKey("ksvc"), kh.getKsvcStatus(ksvc), nil, parent, virMesh, virIngress)
	ksvcSvcStatus := isvcGraphNodeStatusTrue
	if ksvcSvc == nil {
		ksvcSvcStatus = isvcGraphNodeStatusFalse
	}
	_ = nodes.AddNodes(name, namespace, handler.GetCrdKey("svc"), ksvcSvcStatus, ksvcObj)
	for index := range ksvcRev {
		ksvcRev[index].ToSampleObject(ksvcObj, nodes)
	}
	return ksvcObj
}

// Knative 相关节点
func (kh *Handler) getServiceKnative(c *gin.Context, isvc *v1beta1.InferenceService, nodes simpleObjectMap, namespace string) {
	virObj := kh.getVirtualServicesShow(c, isvc.Name, namespace, nodes)
	svc := corev1.Service{}
	svcObj := nodes.AddNodes(isvc.Name, namespace, handler.GetCrdKey("svc"), isvcGraphNodeStatusTrue, nil, virObj)
	if err := kh.kc.Get(c.Request.Context(), types.NamespacedName{Namespace: namespace, Name: isvc.Name}, &svc); err != nil {
		log.Logger.Error(err, "获取svc 失败 ", "namespace", namespace, "name", isvc.Name)
		svcObj.SetStatus(isvcGraphNodeStatusFalse)
	}

	// 推理服务
	predictorName := ksvcconstants.PredictorServiceName(isvc.Name)
	kh.getKsvcShow(c.Request.Context(), predictorName, isvc.Name, namespace, virObj, nodes)

	// Transformer
	if isvc.Spec.Transformer != nil {
		transformerName := ksvcconstants.TransformerServiceName(isvc.Name)
		kh.getKsvcShow(c.Request.Context(), transformerName, isvc.Name, namespace, virObj, nodes)

	}
	// Explainer
	if isvc.Spec.Explainer != nil {
		explainerName := ksvcconstants.ExplainerServiceName(isvc.Name)
		kh.getKsvcShow(c.Request.Context(), explainerName, isvc.Name, namespace, virObj, nodes)
	}
}

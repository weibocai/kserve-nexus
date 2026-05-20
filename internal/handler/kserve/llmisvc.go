package kserve

import (
	"context"
	"fmt"
	"time"

	epv1alpha1 "github.com/envoyproxy/ai-gateway/api/v1alpha1"
	"github.com/gin-gonic/gin"
	ksvcv1alpha2 "github.com/kserve/kserve/pkg/apis/serving/v1alpha2"
	ksvcllmisvc "github.com/kserve/kserve/pkg/controller/v1alpha2/llmisvc"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
	igwapi "sigs.k8s.io/gateway-api-inference-extension/api/v1"

	"github.com/kserve-nexus/internal/handler"
	"github.com/kserve-nexus/internal/middleware"
	"github.com/kserve-nexus/pkg/log"
)

func (kh *Handler) getLLMIsvc(ctx context.Context, namespace string) ([]map[string]string, error) {
	var llmisvc ksvcv1alpha2.LLMInferenceServiceList
	if err := kh.kc.List(ctx, &llmisvc, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	response := make([]map[string]string, len(llmisvc.Items))
	for i := range llmisvc.Items {
		svc := llmisvc.Items[i]
		_response := map[string]string{
			"name": svc.Name, "namespace": svc.Namespace, "url": svc.Status.URL.String(), "ready": "False", "reason": "",
		}
		for j := range svc.Status.Conditions {
			if svc.Status.Conditions[j].Type == apis.ConditionReady {
				_response["ready"] = string(svc.Status.Conditions[j].Status)
				_response["reason"] = svc.Status.Conditions[j].Reason
				break
			}
		}
		response[i] = _response
	}
	return response, nil
}

// ListLLMIsvc 获取LLMInferenceService列表
// @Summary 获取LLM推理服务列表
// @Description 获取指定命名空间下的LLMInferenceService列表，若namespace=all则返回所有命名空间下的服务
// @Tags llmisvc
// @Accept JSON
// @Produce JSON
// @Param namespace query string false "命名空间，默认all表示所有命名空间"
// @Success 200 {object} middleware.Response "成功"
// @Failure 500 {object} middleware.Response "请求异常"
// @Router /kserve/llmisvc [get]
func (kh *Handler) ListLLMIsvc(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "all")
	if namespace != "all" {
		services, err := kh.getLLMIsvc(c.Request.Context(), namespace)
		middleware.ResponseJson(c, services, err)
		return
	}
	namespaces, err := kh.listNamespaces(c.Request.Context())
	if err != nil {
		middleware.ErrorJson(c, err, "")
		return
	}
	services := make([]map[string]string, 0)
	for index := range namespaces {
		if _services, err := kh.getLLMIsvc(c.Request.Context(), namespaces[index]); err == nil {
			services = append(services, _services...)
		} else {
			middleware.ErrorJson(c, err, "")
			return
		}
	}
	middleware.ResponseJson(c, services, nil)
}

func (kh *Handler) getInferencePoolsShow(ctx context.Context, isvcName, name, namespace string, parentHr *simpleObject, nodes simpleObjectMap) {
	ip := &igwapi.InferencePool{}
	ipObj := nodes.AddNodes(name, namespace, handler.GetCrdKey("ip"), isvcGraphNodeStatusTrue, nil, parentHr)
	if err := kh.kc.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, ip); err != nil {
		ipObj.SetStatus(isvcGraphNodeStatusFalse)
		log.Logger.Error(err, "获取 InferencePool 失败", "Namespace", namespace, "Name", name)
	}
	if !ksvcllmisvc.IsInferencePoolReady(ip) {
		ipObj.SetStatus(isvcGraphNodeStatusFalse)
	}
	if ip.Spec.EndpointPickerRef.Kind != "Service" {
		log.Logger.Info("获取 InferencePool Service 失败", "kind", ip.Spec.EndpointPickerRef.Kind, "Namespace", namespace, "Name", name)
		nodes.AddNodes(string(ip.Spec.EndpointPickerRef.Name), namespace, handler.GetCrdKey("svc"), isvcGraphNodeStatusFalse, nil, ipObj)
		return
	}

	kh.getSvcDepShow(ctx, isvcName, string(ip.Spec.EndpointPickerRef.Name), namespace, ipObj, "", nodes)
}

func (kh *Handler) getEnvoyProxyShow(ctx context.Context, isvcName, name, namespace string, nodes simpleObjectMap) {
	var aiGateway epv1alpha1.AIGatewayRouteList
	if err := kh.kc.List(ctx, &aiGateway, client.InNamespace(namespace)); err != nil {
		return
	}
	var aig *epv1alpha1.AIGatewayRoute
	for i := range aiGateway.Items {
		ag := aiGateway.Items[i]
		for j := range ag.Spec.Rules {
			for k := range ag.Spec.Rules[j].BackendRefs {
				if ag.Spec.Rules[j].BackendRefs[k].Kind == nil {
					continue
				}
				if string(*ag.Spec.Rules[j].BackendRefs[k].Kind) != "InferencePool" {
					continue
				}
				if ag.Spec.Rules[j].BackendRefs[k].Name != name {
					continue
				}
				aig = &ag
			}
		}
	}
	if aig == nil {
		return
	}
	aigObj := nodes.AddNodes(aig.Name, aig.Namespace, handler.GetCrdKey(""), isvcGraphNodeStatusFalse, nil)
	hrObj, _ := kh.getHTTPRouteShow(ctx, isvcName, name, namespace, "", nodes)
	hrObj.AddParent(aigObj)
}

// GetLLMIsvc 获取单个LLMInferenceService详情
// @Summary 获取单个LLM推理服务详情及拓扑图
// @Description 获取指定命名空间下单个LLMInferenceService的详细信息，包含部署拓扑图。kind=grafana时返回DOT格式，否则返回节点与边JSON
// @Tags llmisvc
// @Accept JSON
// @Produce JSON
// @Param name path string true "LLMInferenceService名称"
// @Param namespace query string true "命名空间"
// @Param kind query string false "返回格式：grafana(DOT格式)或默认(节点边JSON)"
// @Success 200 {object} middleware.Response "成功"
// @Failure 400 {object} middleware.Response "参数错误"
// @Failure 500 {object} middleware.Response "请求异常"
// @Router /kserve/llmisvc/{name} [get]
func (kh *Handler) GetLLMIsvc(c *gin.Context) {
	namespace := c.Query("namespace")
	name := c.Param("name")
	kind := c.DefaultQuery("kind", "grafana")
	fmt.Println(namespace, name, kind)
	if namespace == "" || name == "" {
		middleware.ErrorJson(c, nil, fmt.Sprintf("参数错误：name=%s; namespace=%s", name, namespace))
		return
	}
	response := make(map[string]any)
	var llmisvc ksvcv1alpha2.LLMInferenceService
	if err := kh.kc.Get(c.Request.Context(), types.NamespacedName{Namespace: namespace, Name: name}, &llmisvc); err != nil {
		middleware.ErrorJson(c, err, "")
		return
	}
	response["llmisvc"] = llmisvc
	response["status"] = isvcGraphNodeStatusFalse
	for j := range llmisvc.Status.Conditions {
		if llmisvc.Status.Conditions[j].Type == apis.ConditionReady {
			response["status"] = string(llmisvc.Status.Conditions[j].Status)
			break
		}
	}
	var nodes simpleObjectMap = make(map[string]*simpleObject)
	kh.getHTTPRouteShow(c.Request.Context(), llmisvc.Name, llmisvc.Name+"-kserve-route", namespace, "", nodes)
	if kind == "grafana" {
		dotDiagram, err := simpleObject2DigraphNode(name, nodes)
		if err != nil {
			log.Logger.Error(err, "failed to generate diagram")
		}
		data := map[string]string{"timestamp": time.Now().Format("2006-04-02 15:01:05"), "dot_diagram": dotDiagram}
		middleware.SuccessJson(c, data)
		return
	}
	response["nodes"], response["edges"] = simpleObject2graphNode(nodes)
	middleware.ResponseJson(c, response, nil)
}

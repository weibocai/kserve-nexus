package kserve

import (
	"github.com/gin-gonic/gin"
	"github.com/kserve-nexus/internal/middleware"
	ksvcv1alpha1 "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	ksvcv1alpha2 "github.com/kserve/kserve/pkg/apis/serving/v1alpha2"
	ksvcv1beta1 "github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func isvcConditions(conditions *duckv1.Conditions) (bool, string) {
	for j := range *conditions {
		if (*conditions)[j].Type != apis.ConditionReady {
			return true, (*conditions)[j].Reason
		}
	}
	return true, ""
}

// Dashboard 首页统计信息
func (kh *KserveHandler) Dashboard(c *gin.Context) {
	response := map[string]int{"isvc": 0, "isvcError": 0, "llmisvc": 0, "llmisvcError": 0, "graph": 0, "graphError": 0}
	// isvc 服务数量
	var isvc = &ksvcv1beta1.InferenceServiceList{}
	if err := kh.kc.List(c.Request.Context(), isvc); err != nil {
		middleware.ErrorJson(c, err, "获取 isvc 异常")
		return
	}
	for i := range isvc.Items {
		if ready, _ := isvcConditions(&isvc.Items[i].Status.Conditions); !ready {
			response["isvcError"] += 1
		}
	}

	// llm 服务数量
	var llm = &ksvcv1alpha2.LLMInferenceServiceList{}
	if err := kh.kc.List(c.Request.Context(), llm); err != nil {
		middleware.ErrorJson(c, err, "获取 llmisvc 失败")
		return
	}

	for i := range llm.Items {
		if ready, _ := isvcConditions(&llm.Items[i].Status.Conditions); !ready {
			response["llmisvcError"] += 1
		}
	}

	// 推理图
	var graph = &ksvcv1alpha1.InferenceGraphList{}
	if err := kh.kc.List(c.Request.Context(), graph); err != nil {
		middleware.ErrorJson(c, err, "获取 graph 失败")
		return
	}

	for i := range graph.Items {
		if ready, _ := isvcConditions(&graph.Items[i].Status.Conditions); !ready {
			response["graphError"] += 1
		}
	}
	response["isvc"], response["llmisvc"], response["graph"] = len(isvc.Items), len(llm.Items), len(graph.Items)
	middleware.SuccessJson(c, response)
}

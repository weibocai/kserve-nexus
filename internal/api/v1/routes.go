package v1

import (
	"github.com/gin-gonic/gin"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hc "github.com/kserve-nexus/internal/handler/crd"
	he "github.com/kserve-nexus/internal/handler/event"
	hk "github.com/kserve-nexus/internal/handler/kserve"
)

// SetupRoutes 设置路由
func SetupRoutes(r *gin.Engine, kc client.Client, cs *kubernetes.Clientset) {
	// 定义kserve路由组
	kr := r.Group("/kserve")
	{
		// 创建 UserService 实例
		kh := hk.NewKserveHandler(kc, cs)
		kr.GET("/namespace", kh.ListNamespaces)
		kr.GET("/config", kh.GetConfigMap)
		kr.GET("/isvc", kh.ListIsvc)
		kr.GET("/isvc/:name", kh.GetIsvc)
		kr.GET("/graph", kh.GetGraph)
		kr.GET("/graph/:name", kh.GetGraph)
		kr.GET("/llmisvc", kh.ListLLMIsvc)
		kr.GET("/llmisvc/:name", kh.GetLLMIsvc)
		kr.GET("/dashboard", kh.Dashboard)
	}
	// crd 相关路由
	cr := r.Group("/crd")
	{
		ch := hc.NewCrdHandler(kc, cs)
		cr.GET("/pod", ch.Pod)
	}

	// websocket 相关路由
	ws := r.Group("/websocket")
	wsm := he.NewWebsocketClientManager("default")
	{
		wh := he.NewHandleEvent(wsm)
		ws.GET("/host", wh.GetEventHost)
		ws.GET("/event/:name", wh.Event)
	}
}

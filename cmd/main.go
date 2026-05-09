package main

import (
	_ "github.com/kserve-nexus/docs"
	"github.com/kserve-nexus/internal/app"
	_ "github.com/kserve-nexus/internal/handler"
)

/*
@title KServe Nexus API
@version 1.0
@description KServe推理服务管理后台API，提供InferenceService、LLMInferenceService、InferenceGraph等KServe资源的查询与管理功能
@termsOfService http://example.com/terms/
@contact.name API Support
@contact.url http://example.com/support
@contact.email support@example.com
@license.name Apache 2.0
@license.url http://www.apache.org/licenses/LICENSE-2.0.html
@host localhost:5109
@BasePath /
@schemes http https
*/
func main() {
	app.Start()
}

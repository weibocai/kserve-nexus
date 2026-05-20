package handler

// GetCrdKey 简写转成 kubectl api-resources
func GetCrdKey(name string) string {
	crdName := map[string]string{
		"svc": "Service", "deploy": "Deployment", "pod": "pod", "hpa": "HorizontalPodAutoscaler",
		"hr": "HTTPRoute", "gtw": "Gateway", "gc": "GatewayClass", "vs": "VirtualService",
		"gw": "istio.Gateway", "ksvc": "ksvc", "isvc": "InferenceService", "llmisvc": "LLMInferenceService",
		"revision": "knative.Revision", "kConfig": "knative.Configuration",
		"kLessService": "knative.ServerlessService", "ip": "InferencePool", "ServiceAccount": "sa", "Secret": "secrets",
		"pvc": "PersistentVolumeClaim", "pv": "PersistentVolume", "sc": "StorageClass",
	}
	if key, ok := crdName[name]; ok {
		return key
	}
	return "unknown"
}

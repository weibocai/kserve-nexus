package crd

import (
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Handler crd 处理器
type Handler struct {
	kc client.Client
	cs *kubernetes.Clientset
}

// NewCrdHandler 初始化 crd handler
func NewCrdHandler(kc client.Client, clientSet *kubernetes.Clientset) *Handler {
	return &Handler{kc: kc, cs: clientSet}
}

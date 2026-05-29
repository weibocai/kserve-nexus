kind create cluster --config=docs/samples/kind/kind-config.yaml

kind load docker-image quay.io/jetstack/cert-manager-cainjector:v1.20.2 --name kserve
kind load docker-image quay.io/jetstack/cert-manager-controller:v1.20.2 --name kserve
kind load docker-image quay.io/jetstack/cert-manager-webhook:v1.20.2 --name kserve
kind load docker-image istio/pilot:1.28.6 --name kserve
kind load docker-image istio/proxyv2:1.28.6 --name kserve
kind load docker-image swr.cn-north-4.myhuaweicloud.com/ddn-k8s/gcr.io/knative-releases/knative.dev/serving/cmd/autoscaler:v1.21.1 --name kserve
kind load docker-image swr.cn-north-4.myhuaweicloud.com/ddn-k8s/gcr.io/knative-releases/knative.dev/serving/cmd/activator:v1.21.1 --name kserve
kind load docker-image swr.cn-north-4.myhuaweicloud.com/ddn-k8s/gcr.io/knative-releases/knative.dev/net-istio/cmd/webhook:v1.21.1 --name kserve
kind load docker-image swr.cn-north-4.myhuaweicloud.com/ddn-k8s/gcr.io/knative-releases/knative.dev/serving/cmd/controller:v1.21.1 --name kserve
kind load docker-image swr.cn-north-4.myhuaweicloud.com/ddn-k8s/gcr.io/knative-releases/knative.dev/net-istio/cmd/controller:v1.21.1 --name kserve
kind load docker-image swr.cn-north-4.myhuaweicloud.com/ddn-k8s/gcr.io/knative-releases/knative.dev/serving/cmd/webhook:v1.21.1 --name kserve
kind load docker-image swr.cn-north-4.myhuaweicloud.com/ddn-k8s/gcr.io/knative-releases/knative.dev/serving/cmd/autoscaler-hpa:v1.21.1 --name kserve
kind load docker-image ghcr.io/kedacore/keda-admission-webhooks:2.19.0 --name kserve
kind load docker-image ghcr.io/kedacore/keda-metrics-apiserver:2.19.0 --name kserve
kind load docker-image ghcr.io/kedacore/keda:2.19.0 --name kserve
kind load docker-image swr.cn-north-4.myhuaweicloud.com/ddn-k8s/registry.k8s.io/lws/lws:v0.8.0 --name kserve
kind load docker-image envoyproxy/envoy:v1.32.6 --name kserve
kind load docker-image swr.cn-north-4.myhuaweicloud.com/ddn-k8s/gcr.io/knative-releases/knative.dev/serving/cmd/queue:v1.16.2 --name kserve

module github.com/gardener/gardener-extension-provider-vsphere

go 1.16

require (
	github.com/Masterminds/semver v1.5.0
	github.com/ahmetb/gen-crd-api-reference-docs v0.2.0
	github.com/coreos/go-systemd/v22 v22.3.2
	github.com/gardener/etcd-druid v0.8.0
	github.com/gardener/gardener v1.47.1-0.20220524072143-bd4e6e20cacb
	github.com/gardener/machine-controller-manager v0.44.1
	github.com/go-logr/logr v1.2.3
	github.com/golang/mock v1.6.0
	github.com/google/uuid v1.1.2
	github.com/onsi/ginkgo/v2 v2.1.4
	github.com/onsi/gomega v1.19.0
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.4.0
	github.com/spf13/pflag v1.0.5
	github.com/vmware/go-vmware-nsxt v0.0.0-20200114231430-33a5af043f2e
	github.com/vmware/vsphere-automation-sdk-go/lib v0.3.1
	github.com/vmware/vsphere-automation-sdk-go/runtime v0.3.1
	github.com/vmware/vsphere-automation-sdk-go/services/nsxt v0.4.0
	golang.org/x/tools v0.1.10
	k8s.io/api v0.23.3
	k8s.io/apiextensions-apiserver v0.23.3
	k8s.io/apimachinery v0.23.3
	k8s.io/autoscaler/vertical-pod-autoscaler v0.10.0
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/cloud-provider-vsphere v1.1.0
	k8s.io/code-generator v0.23.3
	k8s.io/component-base v0.23.3
	k8s.io/klog v1.0.0
	k8s.io/kubelet v0.23.3
	k8s.io/utils v0.0.0-20220210201930-3a6ce19ff2f9
	sigs.k8s.io/controller-runtime v0.11.1
	sigs.k8s.io/controller-tools v0.8.0
	sigs.k8s.io/yaml v1.3.0
)

replace (
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v1.11.0 // keep this value in sync with k8s.io/client-go
	k8s.io/api => k8s.io/api v0.23.2
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.23.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.23.2
	k8s.io/apiserver => k8s.io/apiserver v0.23.2
	k8s.io/autoscaler => k8s.io/autoscaler v0.0.0-20201008123815-1d78814026aa // translates to k8s.io/autoscaler/vertical-pod-autoscaler@v0.9.0
	k8s.io/autoscaler/vertical-pod-autoscaler => k8s.io/autoscaler/vertical-pod-autoscaler v0.9.0
	k8s.io/client-go => k8s.io/client-go v0.23.2
	k8s.io/code-generator => k8s.io/code-generator v0.23.2
	k8s.io/component-base => k8s.io/component-base v0.23.2
	k8s.io/helm => k8s.io/helm v2.13.1+incompatible
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.23.2
)

// needed for infra-cli and load balancer cleanup
replace k8s.io/cloud-provider-vsphere => github.com/MartinWeindel/cloud-provider-vsphere v1.0.1-0.20210910074917-6559ac3f3bcf

// workaround for https://github.com/gardener/hvpa-controller/issues/92, remove once it's fixed
replace (
	github.com/gardener/hvpa-controller => github.com/gardener/hvpa-controller v0.4.0
	github.com/gardener/hvpa-controller/api => github.com/gardener/hvpa-controller/api v0.4.0
)

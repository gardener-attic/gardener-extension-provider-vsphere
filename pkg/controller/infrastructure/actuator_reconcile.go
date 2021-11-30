/*
 * Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 *
 */

package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"time"

	"github.com/gardener/gardener-extension-provider-vsphere/pkg/vsphere/withkubernetes"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	errors2 "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/retry"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	apisvsphere "github.com/gardener/gardener-extension-provider-vsphere/pkg/apis/vsphere"
	apishelper "github.com/gardener/gardener-extension-provider-vsphere/pkg/apis/vsphere/helper"
	apisvspherev1alpha1 "github.com/gardener/gardener-extension-provider-vsphere/pkg/apis/vsphere/v1alpha1"
	"github.com/gardener/gardener-extension-provider-vsphere/pkg/vsphere"
	"github.com/gardener/gardener-extension-provider-vsphere/pkg/vsphere/helpers"
	"github.com/gardener/gardener-extension-provider-vsphere/pkg/vsphere/infrastructure"
	"github.com/gardener/gardener-extension-provider-vsphere/pkg/vsphere/infrastructure/ensurer"
)

type preparedReconcile struct {
	cloudProfileConfig *apisvsphere.CloudProfileConfig
	infraConfig        *apisvsphere.InfrastructureConfig

	nsxtConfig *infrastructure.NSXTConfig
	region     *apisvsphere.RegionSpec
	spec       infrastructure.NSXTInfraSpec
	ensurer    infrastructure.NSXTInfrastructureEnsurer

	k8sRegion     *apisvsphere.K8sRegionSpec
	k8sRestConfig *rest.Config
	k8sClient     ctrlClient.Client
	apiClient     *withkubernetes.VsphereAPIClient
}

func (p *preparedReconcile) getDefaultLoadBalancerIPPoolName() (*string, error) {
	var ipPoolName *string
	for i, cls := range p.cloudProfileConfig.Constraints.LoadBalancerConfig.Classes {
		if i == 0 || cls.Name == "default" {
			ipPoolName = cls.IPPoolName
		}
	}
	if ipPoolName == nil {
		return nil, fmt.Errorf("ipPoolName not set for default load balancer class")
	}
	return ipPoolName, nil
}

func (a *actuator) prepareReconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) (*preparedReconcile, error) {
	cloudProfileConfig, err := apishelper.GetCloudProfileConfig(cluster)
	if err != nil {
		return nil, err
	}

	infraConfig, err := apishelper.GetInfrastructureConfig(cluster)
	if err != nil {
		return nil, err
	}

	creds, err := vsphere.GetCredentials(ctx, a.Client(), infra.Spec.SecretRef)
	if err != nil {
		return nil, err
	}

	if creds.IsVsphereKubernetes() {
		if cloudProfileConfig.VsphereWithKubernetes == nil {
			return nil, fmt.Errorf("cloudProfileConfig.vsphereWithKubernetes is missing")
		}
		region := apishelper.FindK8sRegion(infra.Spec.Region, cloudProfileConfig)
		if region == nil {
			return nil, fmt.Errorf("k8s region %q not found in cloud profile", infra.Spec.Region)
		}
		restConfig, err := withkubernetes.CreateVsphereKubernetesRestConfig(creds.VsphereKubeconfig())
		if err != nil {
			return nil, fmt.Errorf("cannot create rest config from kubeconfig: %w", err)
		}
		client, err := withkubernetes.CreateVsphereKubernetesClient(creds.VsphereKubeconfig())
		if err != nil {
			return nil, fmt.Errorf("cannot create client from kubeconfig: %w", err)
		}
		apiClient, err := withkubernetes.GetVsphereAPISession(*region, creds.VsphereAPI())
		if err != nil {
			return nil, fmt.Errorf("cannot create vsphere client for user: %s: %w", creds.VsphereAPI().Username, err)
		}
		prepared := &preparedReconcile{
			cloudProfileConfig: cloudProfileConfig,
			infraConfig:        infraConfig,
			k8sClient:          client,
			k8sRestConfig:      restConfig,
			apiClient:          apiClient,
			k8sRegion:          region,
		}
		return prepared, nil
	}

	region := apishelper.FindRegion(infra.Spec.Region, cloudProfileConfig)
	if region == nil {
		return nil, fmt.Errorf("region %q not found in cloud profile", infra.Spec.Region)
	}
	if len(region.Zones) == 0 {
		return nil, fmt.Errorf("region %q has no zones in cloud profile", infra.Spec.Region)
	}
	dnsServers := cloudProfileConfig.DNSServers
	if len(region.DNSServers) > 0 {
		dnsServers = region.DNSServers
	}

	nsxtConfig := helpers.NewNSXTConfig(creds, region)

	spec := infrastructure.NSXTInfraSpec{
		EdgeClusterName:   region.EdgeCluster,
		TransportZoneName: region.TransportZone,
		Tier0GatewayName:  region.LogicalTier0Router,
		SNATIPPoolName:    region.SNATIPPool,
		GardenID:          a.gardenID,
		GardenName:        cloudProfileConfig.NamePrefix,
		ClusterName:       infra.Namespace,
		WorkersNetwork:    *cluster.Shoot.Spec.Networking.Nodes,
		DNSServers:        dnsServers,
	}

	dhcpOptions := cloudProfileConfig.DHCPOptions
	if region.DHCPOptions != nil {
		dhcpOptions = region.DHCPOptions
	}
	if len(dhcpOptions) > 0 {
		spec.DHCPOptions = map[int][]string{}
		for _, option := range dhcpOptions {
			spec.DHCPOptions[option.Code] = option.Values
		}
	}

	if infraConfig.Networks != nil {
		spec.ExternalTier1GatewayPath = &infraConfig.Networks.Tier1GatewayPath
	}

	shootCtx := &ensurer.ShootContext{ShootNamespace: cluster.ObjectMeta.Name, GardenID: a.gardenID}
	infraEnsurer, err := ensurer.NewNSXTInfrastructureEnsurer(a.logger, nsxtConfig, shootCtx)
	if err != nil {
		return nil, err
	}

	prepared := &preparedReconcile{
		cloudProfileConfig: cloudProfileConfig,
		infraConfig:        infraConfig,
		nsxtConfig:         nsxtConfig,
		region:             region,
		spec:               spec,
		ensurer:            infraEnsurer,
	}
	return prepared, nil
}

func (a *actuator) getInfrastructureState(infra *extensionsv1alpha1.Infrastructure) (*apisvsphere.NSXTInfraState, *bool, error) {
	infraStatus, err := apishelper.GetInfrastructureStatus(infra.Namespace, infra.Status.ProviderStatus)
	if err != nil {
		return nil, nil, err
	}
	if infraStatus == nil {
		return nil, nil, nil
	}
	return infraStatus.NSXTInfraState, infraStatus.CreationStarted, nil
}

func (a *actuator) reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	state, creationStarted, err := a.getInfrastructureState(infra)
	// error handling later

	prepared, err2 := a.prepareReconcile(ctx, infra, cluster)
	if err2 != nil {
		return err2
	}
	if prepared.k8sClient != nil {
		if creationStarted != nil {
			return fmt.Errorf("invalid state: creationStarted for vsphere with kubernetes?")
		}
		return a.reconcileK8s(ctx, prepared, infra, cluster)
	}

	if creationStarted == nil || !*creationStarted {
		// early status update to allow deletion on wrong credentials
		if err == nil {
			err = prepared.ensurer.CheckConnection()
		}
		b := err == nil
		creationStarted = &b
		errUpdate := a.updateProviderStatus(ctx, infra, state, prepared, creationStarted)
		if err != nil {
			return err
		}
		if errUpdate != nil {
			return errUpdate
		}
	}
	if err != nil {
		return err
	}

	if state == nil {
		state, err = prepared.ensurer.NewStateWithVersion(prepared.infraConfig.OverwriteNSXTInfraVersion)
		if err != nil {
			return errors.Wrapf(err, "NewStateWithVersion failed")
		}
	}
	err = prepared.ensurer.EnsureInfrastructure(prepared.spec, state)
	errUpdate := a.updateProviderStatus(ctx, infra, state, prepared, creationStarted)
	if err != nil {
		return err
	}
	return errUpdate
}

// Helper functions

func (a *actuator) updateProviderStatus(
	ctx context.Context,
	infra *extensionsv1alpha1.Infrastructure,
	newState *apisvsphere.NSXTInfraState,
	prepared *preparedReconcile,
	creationStarted *bool,
) error {
	status, err := a.makeProviderInfrastructureStatus(newState, prepared, creationStarted)
	if err == nil {
		err = a.doUpdateProviderStatus(ctx, infra, status)
	}
	if err != nil {
		a.logFailedSaveState(err, newState)
	}
	return err
}

func (a *actuator) makeProviderInfrastructureStatus(
	state *apisvsphere.NSXTInfraState,
	prepared *preparedReconcile,
	creationStarted *bool,
) (*apisvsphere.InfrastructureStatus, error) {
	safe := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}

	status := &apisvsphere.InfrastructureStatus{
		TypeMeta: metav1.TypeMeta{
			APIVersion: api.SchemeGroupVersion.String(),
			Kind:       "InfrastructureStatus",
		},
		NSXTInfraState:  state,
		CreationStarted: creationStarted,
	}

	if prepared != nil {
		cloudProfileConfig := prepared.cloudProfileConfig
		region := prepared.region
		zoneConfigs := map[string]apisvsphere.ZoneConfig{}
		for _, z := range region.Zones {
			datacenter := region.Datacenter
			if z.Datacenter != nil {
				datacenter = z.Datacenter
			}
			if datacenter == nil {
				return nil, fmt.Errorf("datacenter not set in zone %s", z.Name)
			}
			datastore := region.Datastore
			datastoreCluster := region.DatastoreCluster
			if z.Datastore != nil {
				datastore = z.Datastore
				datastoreCluster = nil
			} else if z.DatastoreCluster != nil {
				datastore = nil
				datastoreCluster = z.DatastoreCluster
			}
			zoneConfigs[z.Name] = apisvsphere.ZoneConfig{
				Datacenter:       safe(datacenter),
				ComputeCluster:   safe(z.ComputeCluster),
				ResourcePool:     safe(z.ResourcePool),
				HostSystem:       safe(z.HostSystem),
				Datastore:        safe(datastore),
				DatastoreCluster: safe(datastoreCluster),
				SwitchUUID:       safe(z.SwitchUUID),
			}
		}
		status.VsphereConfig = &apisvsphere.VsphereConfig{
			Folder:      cloudProfileConfig.Folder,
			Region:      region.Name,
			ZoneConfigs: zoneConfigs,
		}
	}

	return status, nil
}

func (a *actuator) doUpdateProviderStatus(
	ctx context.Context,
	infra *extensionsv1alpha1.Infrastructure,
	status *apisvsphere.InfrastructureStatus,
) error {

	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, a.Client(), infra, func() error {
		statusV1alpha1 := &apisvspherev1alpha1.InfrastructureStatus{}
		err := a.Scheme().Convert(status, statusV1alpha1, nil)
		if err != nil {
			return err
		}
		statusV1alpha1.SetGroupVersionKind(apisvspherev1alpha1.SchemeGroupVersion.WithKind("InfrastructureStatus"))
		infra.Status.ProviderStatus = &runtime.RawExtension{Object: statusV1alpha1}
		return nil
	})
}

func (a *actuator) logFailedSaveState(err error, state *apisvsphere.NSXTInfraState) {
	bytes, err2 := json.Marshal(state)
	stateString := ""
	if err2 == nil {
		stateString = string(bytes)
	} else {
		stateString = err2.Error()
	}
	a.logger.Error(err, "persisting infrastructure state failed", "state", stateString)
}

func (a *actuator) reconcileK8s(ctx context.Context, prepared *preparedReconcile, infra *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	vwk := prepared.cloudProfileConfig.VsphereWithKubernetes
	namespace, createNamespace := withkubernetes.CalcSupervisorNamespace(cluster, vwk)

	err := a.checkOrCreateNamespace(ctx, prepared, namespace, createNamespace, vwk)
	if err != nil {
		return err
	}

	err = a.checkOrCreateCCMServiceAccount(ctx, prepared, namespace)
	if err != nil {
		return err
	}

	nodes := cluster.Shoot.Spec.Networking.Nodes
	if nodes == nil {
		return fmt.Errorf("unexpected: networking nodes == nil")
	}
	err = a.checkOrCreateNetwork(ctx, prepared.k8sClient, ctrlClient.ObjectKey{Namespace: namespace, Name: cluster.ObjectMeta.Name}, *nodes)
	if err != nil {
		return err
	}
	ncpRouterID, err := a.lookupNCPRouterID(ctx, prepared.k8sClient, namespace)
	if err != nil {
		return err
	}

	return a.updateProviderStatusK8s(ctx, infra, cluster.ObjectMeta.Name, ncpRouterID)
}

func (a *actuator) checkOrCreateNamespace(ctx context.Context, prepared *preparedReconcile, namespace string, create bool, vwk *apisvsphere.VsphereWithKubernetes) error {
	vsphereCluster, err := prepared.apiClient.GetNamespaceCluster(namespace)

	if create && withkubernetes.IsNotFoundError(err) {
		err = prepared.apiClient.CreateNamespace(namespace, vwk)
		if err != nil {
			return fmt.Errorf("creation of namespace %s failed: %w", namespace, err)
		}
	} else if err != nil {
		return fmt.Errorf("namespace %s cannot looked up: %w", namespace, err)
	} else if vsphereCluster != prepared.k8sRegion.Cluster {
		return fmt.Errorf("namespace %s has wrong cluster: %s != %s (region=%s)", namespace, vsphereCluster, prepared.k8sRegion.Cluster, prepared.k8sRegion.Name)
	}

	return a.ensureNamespaceShootLabel(ctx, prepared, namespace)
}

func (a *actuator) ensureNamespaceShootLabel(ctx context.Context, prepared *preparedReconcile, namespace string) error {
	key := ctrlClient.ObjectKey{Name: namespace}
	ns := &corev1.Namespace{}
	err := prepared.k8sClient.Get(ctx, key, ns)
	if err != nil {
		return fmt.Errorf("cannot get kubernetes namespace object %s: %w", namespace, err)
	}
	if labels := ns.GetLabels(); labels == nil || labels["gardener.cloud/purpose"] != "shoot" {
		// currently this label is needed to disable patched webhooks on supervisor cluster:
		// - "ValidatingWebhookConfiguration/vmware-system-tkg-validating-webhook-configuration"
		//   - webhook named "capi.validating.tanzukubernetescluster.run.tanzu.vmware.com"
		// - "ValidatingWebhookConfiguration/vmware-system-vmop-validating-webhook-configuration"
		//   - webhook named "default.validating.virtualmachine.vmoperator.vmware.com"
		//
		// In both cases a namespaceSelector has been added to exclude this label
		//      "namespaceSelector": {
		//                "matchExpressions": [
		//                    {
		//                        "key": "gardener.cloud/purpose",
		//                        "operator": "NotIn",
		//                        "values": [
		//                            "shoot"
		//                        ]
		//                    }
		//                ]
		//            }
		if labels == nil {
			labels = map[string]string{}
		}
		labels["gardener.cloud/purpose"] = "shoot"
		ns.SetLabels(labels)
		err = prepared.k8sClient.Update(ctx, ns)
		if err != nil {
			return fmt.Errorf("cannot set label for kubernetes namespace object %s: %w", namespace, err)
		}
	}
	return nil
}

func (a *actuator) checkOrCreateCCMServiceAccount(ctx context.Context, prepared *preparedReconcile, namespace string) error {
	chartPath := filepath.Join(vsphere.InternalChartsPath, "supervisor-service-account-ccm")
	return withkubernetes.ApplyServiceAccount(ctx, prepared.k8sClient, prepared.k8sRestConfig, chartPath, "", namespace)
}

func (a *actuator) checkOrCreateNetwork(ctx context.Context, client ctrlClient.Client, name ctrlClient.ObjectKey, nodesCIDR string) error {
	spec := withkubernetes.VirtualNetworkSpec{
		Private:     true,
		EnableDHCP:  true,
		AddressCIDR: nodesCIDR,
	}

	status, err := withkubernetes.GetVirtualNetworkStatus(ctx, client, name)
	if err != nil && errors2.IsNotFound(err) {
		err = withkubernetes.CreateVirtualNetwork(ctx, client, name, spec)
		if err != nil {
			return fmt.Errorf("creating virtual network %s failed: %v", name, err)
		}
	} else if err != nil {
		return fmt.Errorf("getting status of virtual network %s failed: %v", name, err)
	}

	// wait for status == ready
	for i := 0; i < 20; i++ {
		if status != nil && status.Ready {
			break
		}
		time.Sleep(500 * time.Millisecond)
		status, err = withkubernetes.GetVirtualNetworkStatus(ctx, client, name)
		if err != nil {
			return fmt.Errorf("getting status of virtual network %s failed: %v", name, err)
		}
	}
	if !status.Ready {
		return fmt.Errorf("virtual network %s still not ready", name)
	}

	if !reflect.DeepEqual(spec, status.VirtualNetworkSpec) {
		return fmt.Errorf("virtual network %s spec mismatch: %v != %v", name, spec, status.VirtualNetworkSpec)
	}

	return nil
}

func (a *actuator) lookupNCPRouterID(ctx context.Context, client ctrlClient.Client, namespace string) (string, error) {
	ns := &corev1.Namespace{}
	err := client.Get(ctx, ctrlClient.ObjectKey{Name: namespace}, ns)
	if err != nil {
		return "", fmt.Errorf("get namespace object %s failed: %w", namespace, err)
	}

	key := "ncp/router_id"
	if annots := ns.GetAnnotations(); annots != nil {
		if value, ok := annots[key]; ok {
			return value, nil
		}
	}

	return "", fmt.Errorf("cannot find namespace annotation %s", key)
}

func (a *actuator) updateProviderStatusK8s(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, networkName, ncpRouterID string) error {
	status := &apisvsphere.InfrastructureStatus{
		TypeMeta: metav1.TypeMeta{
			APIVersion: api.SchemeGroupVersion.String(),
			Kind:       "InfrastructureStatus",
		},
		VirtualNetwork: &networkName,
		NCPRouterID:    &ncpRouterID,
	}
	return a.doUpdateProviderStatus(ctx, infra, status)
}

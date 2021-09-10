/*
 * Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package loadbalancer

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/loadbalancer"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/loadbalancer/config"
	"k8s.io/cloud-provider-vsphere/pkg/nsxt"
	nsxtconf "k8s.io/cloud-provider-vsphere/pkg/nsxt/config"

	"github.com/gardener/gardener-extension-provider-vsphere/pkg/vsphere/infrastructure"
)

// DestroyState contains all information needed to find and delete lbs
type DestroyState struct {
	ClusterName       string
	Owner             string
	DefaultIPPoolName string
}

// DestroyAll destroys all load balancers created from NSX-T load balancer controller in cloud-provider-vsphere
func DestroyAll(cfg *infrastructure.NSXTConfig, state *DestroyState) error {
	lbCfg := &config.LBConfig{
		LoadBalancer: config.LoadBalancerConfig{
			LoadBalancerClassConfig: config.LoadBalancerClassConfig{
				IPPoolName: state.DefaultIPPoolName,
			},
			Size: "SMALL",
			AdditionalTags: map[string]string{
				loadbalancer.ScopeOwner: state.Owner,
			},
		},
		LoadBalancerClass: map[string]*config.LoadBalancerClassConfig{},
	}

	nsxtConfig := &nsxtconf.Config{
		User:         cfg.User,
		Password:     cfg.Password,
		Host:         cfg.Host,
		InsecureFlag: cfg.InsecureFlag,
		RemoteAuth:   cfg.RemoteAuth,
	}
	connectorManager, err := nsxt.NewConnectorManager(nsxtConfig)
	if err != nil {
		return fmt.Errorf("NSX-T NewConnectorManager failed: %w", err)
	}
	lbProvider, err := loadbalancer.NewLBProvider(lbCfg, connectorManager.GetConnector())
	if err != nil {
		return fmt.Errorf("NewLBProvider failed: %w", err)
	}

	return lbProvider.CleanupServices(state.ClusterName, map[types.NamespacedName]corev1.Service{}, true)
}

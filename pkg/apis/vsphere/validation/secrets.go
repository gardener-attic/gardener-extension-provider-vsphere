// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validation

import (
	"fmt"

	"github.com/gardener/gardener-extension-provider-vsphere/pkg/vsphere"
	corev1 "k8s.io/api/core/v1"
)

// ValidateCloudProviderSecret checks whether the given secret contains a valid vSphere client credentials.
func ValidateCloudProviderSecret(secret *corev1.Secret) error {
	secretKey := fmt.Sprintf("%s/%s", secret.Namespace, secret.Name)

	keys := []string{vsphere.Username, vsphere.Password, vsphere.NSXTUsername, vsphere.NSXTPassword}

	if val, ok := secret.Data[vsphere.Kubeconfig]; ok {
		if len(val) == 0 {
			return fmt.Errorf("field %q in secret %s cannot be empty", vsphere.Kubeconfig, secretKey)
		}

		// vsphere account needed for managing namespaces
		keys = []string{vsphere.Username, vsphere.Password}
	}

	for _, key := range keys {
		val, ok := secret.Data[key]
		if !ok {
			return fmt.Errorf("missing %q field in secret %s", key, secretKey)
		}
		if len(val) == 0 {
			return fmt.Errorf("field %q in secret %s cannot be empty", key, secretKey)
		}
	}

	return nil
}

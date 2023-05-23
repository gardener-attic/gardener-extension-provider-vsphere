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

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"

	api "github.com/gardener/gardener-extension-provider-vsphere/pkg/apis/vsphere"
	. "github.com/gardener/gardener-extension-provider-vsphere/pkg/apis/vsphere/validation"
)

var _ = Describe("InfrastructureConfig validation", func() {
	var (
		nilPath *field.Path

		infrastructureConfig *api.InfrastructureConfig
	)

	BeforeEach(func() {
		infrastructureConfig = &api.InfrastructureConfig{}
	})

	Describe("#ValidateInfrastructureConfig", func() {
		It("should return no errors for a nil configuration", func() {
			Expect(ValidateInfrastructureConfig(nil, nilPath)).To(BeEmpty())
		})

		It("should return no errors for a valid configuration", func() {
			Expect(ValidateInfrastructureConfig(infrastructureConfig, nilPath)).To(BeEmpty())
		})

		It("should check valid value for OverwriteNSXTInfraVersion", func() {
			s := api.Ensurer_Version1_NSXT25
			infrastructureConfig.OverwriteNSXTInfraVersion = &s
			errorList := ValidateInfrastructureConfig(infrastructureConfig, nilPath)
			Expect(errorList).To(BeEmpty())

			s2 := "3"
			infrastructureConfig.OverwriteNSXTInfraVersion = &s2
			errorList = ValidateInfrastructureConfig(infrastructureConfig, nilPath)
			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("overwriteNSXTInfraVersion"),
			}))))
		})

		It("should check if networks is set, but tier1GatewayPath or loadBalancerServicePath is empty", func() {
			infrastructureConfig.Networks = &api.Networks{Tier1GatewayPath: "foo", LoadBalancerServicePath: "bar"}
			errorList := ValidateInfrastructureConfig(infrastructureConfig, nilPath)
			Expect(errorList).To(BeEmpty())

			infrastructureConfig.Networks = &api.Networks{}
			errorList = ValidateInfrastructureConfig(infrastructureConfig, nilPath)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("networks.tier1GatewayPath"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("networks.loadBalancerServicePath"),
				})),
			))
		})
	})

	Describe("#ValidateInfrastructureConfigUpdate", func() {
		It("should return no errors for an unchanged config", func() {
			infrastructureConfig.Networks = &api.Networks{Tier1GatewayPath: "foo", LoadBalancerServicePath: "bar"}
			newInfraConfig := &api.InfrastructureConfig{
				Networks: &api.Networks{Tier1GatewayPath: "foo", LoadBalancerServicePath: "bar"},
			}
			errorList := ValidateInfrastructureConfigUpdate(infrastructureConfig, newInfraConfig, nilPath)
			Expect(errorList).To(BeEmpty())
		})

		It("should return an error for changed network settings", func() {
			infrastructureConfig.Networks = &api.Networks{Tier1GatewayPath: "foo", LoadBalancerServicePath: "bar"}
			newInfraConfig := &api.InfrastructureConfig{
				Networks: &api.Networks{Tier1GatewayPath: "foo", LoadBalancerServicePath: "changed"},
			}
			errorList := ValidateInfrastructureConfigUpdate(infrastructureConfig, newInfraConfig, nilPath)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("networks"),
				})),
			))
		})
	})

	Describe("#ValidateInfrastructureConfigAgainstCloudProfile", func() {
	})
})

# Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

EXTENSION_PREFIX            := gardener-extension
NAME                        := provider-vsphere
VALIDATOR_NAME              := validator-vsphere
GCVE_TM_RUN_NAME            := gcve-tm-run
REGISTRY                    := eu.gcr.io/gardener-project/gardener
IMAGE_PREFIX                := $(REGISTRY)/extensions
REPO_ROOT                   := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
HACK_DIR                    := $(REPO_ROOT)/hack
VERSION                     := $(shell cat "$(REPO_ROOT)/VERSION")
EFFECTIVE_VERSION           := $(VERSION)-$(shell git rev-parse HEAD)
LD_FLAGS                    := "-w -X github.com/gardener/$(EXTENSION_PREFIX)-$(NAME)/pkg/version.Version=$(IMAGE_TAG)"
LEADER_ELECTION             := false
IGNORE_OPERATION_ANNOTATION := true
KIND_ENV                    := "skaffold"

GARDENER_LOCAL_KUBECONFIG   = $(GGARCHIVE)/example/gardener-local/kind/local/kubeconfig
GGTARBALL                   = /tmp/ggtarball.tgz
#GGTARBALLURL                = $(shell #curl -s https://api.github.com/repos/gardener/gardener/releases/latest | jq '.tarball_url')
GGTARBALLURL                = https://codeload.github.com/gardener/gardener/legacy.tar.gz/refs/heads/master
export GGARCHIVE            = /tmp/ggarchive

SHELL=/usr/bin/env bash -o pipefail

VERSION                     ?= $(shell cat ${REPO_ROOT}/VERSION)
IMAGE_TAG                   := ${VERSION}
WEBHOOK_CONFIG_PORT	        := 8443
WEBHOOK_CONFIG_MODE	        := service
WEBHOOK_CONFIG_URL	        := host.docker.internal:$(WEBHOOK_CONFIG_PORT)
EXTENSION_NAMESPACE	        :=

WEBHOOK_PARAM := --webhook-config-url=$(WEBHOOK_CONFIG_URL)
ifeq ($(WEBHOOK_CONFIG_MODE), service)
  WEBHOOK_PARAM := --webhook-config-namespace=$(EXTENSION_NAMESPACE)
endif

ifneq ($(strip $(shell git status --porcelain 2>/dev/null)),)
	EFFECTIVE_VERSION := $(EFFECTIVE_VERSION)-dirty
endif

NSXT_HOST              := $(shell cat "$(REPO_ROOT)/.kube-secrets/vsphere/nsxt_host")
NSXT_USERNAME          := $(shell cat "$(REPO_ROOT)/.kube-secrets/vsphere/nsxt_username")
NSXT_PASSWORD          := $(shell cat "$(REPO_ROOT)/.kube-secrets/vsphere/nsxt_password")
NSXT_TRANSPORT_ZONE    := $(shell cat "$(REPO_ROOT)/.kube-secrets/vsphere/nsxt_transport_zone")
NSXT_T0_GATEWAY        := $(shell cat "$(REPO_ROOT)/.kube-secrets/vsphere/nsxt_t0_gateway")
NSXT_EDGE_CLUSTER      := $(shell cat "$(REPO_ROOT)/.kube-secrets/vsphere/nsxt_edge_cluster")
NSXT_SNAT_IP_POOL      := $(shell cat "$(REPO_ROOT)/.kube-secrets/vsphere/nsxt_snat_ip_pool")

#########################################
# Tools                                 #
#########################################

TOOLS_DIR := hack/tools
include vendor/github.com/gardener/gardener/hack/tools.mk

#########################################
# Rules for local development scenarios #
#########################################

.PHONY: start
start:
	@LEADER_ELECTION_NAMESPACE=garden GO111MODULE=on go run \
		-mod=vendor \
		-ldflags $(LD_FLAGS) \
		./cmd/$(NAME) \
		--config-file=./example/00-componentconfig.yaml \
		--ignore-operation-annotation=$(IGNORE_OPERATION_ANNOTATION) \
		--leader-election=$(LEADER_ELECTION) \
		--webhook-config-server-host=0.0.0.0 \
		--webhook-config-server-port=$(WEBHOOK_CONFIG_PORT) \
		--webhook-config-mode=$(WEBHOOK_CONFIG_MODE) \
		--gardener-version="v1.39.0" \
		$(WEBHOOK_PARAM)

.PHONY: start-validator
start-validator:
	@LEADER_ELECTION_NAMESPACE=garden GO111MODULE=on go run \
		-mod=vendor \
		-ldflags $(LD_FLAGS) \
		./cmd/$(VALIDATOR_NAME) \
		--leader-election=$(LEADER_ELECTION) \
		--webhook-config-server-host=0.0.0.0 \
		--webhook-config-server-port=9443 \
		--webhook-config-cert-dir=./example/validator-vsphere-certs

#################################################################
# Rules related to binary build, Docker image build and release #
#################################################################

.PHONY: install
install:
	@EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) LD_FLAGS="$(shell $(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/get-build-ld-flags.sh "k8s.io/component-base" "$(REPO_ROOT)/VERSION" "$(EXTENSION_PREFIX)-$(NAME)")" \
	   $(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/install.sh ./...

.PHONY: docker-login
docker-login:
	@gcloud auth activate-service-account --key-file .kube-secrets/gcr/gcr-readwrite.json

.PHONY: docker-images
docker-images:
	@echo "Building docker images with version and tag $(EFFECTIVE_VERSION)"
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) -t $(IMAGE_PREFIX)/$(NAME):$(VERSION)              -t $(IMAGE_PREFIX)/$(NAME):latest           -f Dockerfile -m 6g --target $(EXTENSION_PREFIX)-$(NAME)             .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) -t $(IMAGE_PREFIX)/$(VALIDATOR_NAME):$(VERSION)    -t $(IMAGE_PREFIX)/$(VALIDATOR_NAME):latest -f Dockerfile -m 6g --target $(EXTENSION_PREFIX)-$(VALIDATOR_NAME)   .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) -t $(IMAGE_PREFIX)/$(GCVE_TM_RUN_NAME):$(VERSION)  -t $(GCVE_TM_RUN_NAME):latest               -f Dockerfile -m 6g --target $(EXTENSION_PREFIX)-$(GCVE_TM_RUN_NAME) .

#####################################################################
# Rules for verification, formatting, linting, testing and cleaning #
#####################################################################

.PHONY: revendor
revendor:
	@GO111MODULE=on go mod tidy
	@GO111MODULE=on go mod vendor
	@chmod +x $(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/*
	@chmod +x $(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/.ci/*
	@$(REPO_ROOT)/hack/update-github-templates.sh
	@ln -sf ../vendor/github.com/gardener/gardener/hack/cherry-pick-pull.sh $(HACK_DIR)/cherry-pick-pull.sh

.PHONY: clean
clean:
	@$(shell find ./example -type f -name "controller-registration.yaml" -exec rm '{}' \;)
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/clean.sh ./cmd/... ./pkg/... ./test/...

.PHONY: check-generate
check-generate:
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/check-generate.sh $(REPO_ROOT)

.PHONY: check
check: $(GOIMPORTS) $(GOLANGCI_LINT)
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/check.sh --golangci-lint-config=./.golangci.yaml ./cmd/... ./pkg/... ./test/...
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/check-charts.sh ./charts

.PHONY: check-docforge
check-docforge: $(DOCFORGE)
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/check-docforge.sh $(REPO_ROOT) $(REPO_ROOT)/.docforge/manifest.yaml ".docforge/;docs/" "gardener-extension-provider-vsphere" false

.PHONY: generate
generate: $(CONTROLLER_GEN) $(GEN_CRD_API_REFERENCE_DOCS) $(HELM) $(MOCKGEN)
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/generate.sh ./charts/... ./cmd/... ./example/... ./pkg/... ./test/...

.PHONY: format
format: $(GOIMPORTS)
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/format.sh ./cmd ./pkg ./test

.PHONY: test
test:
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/test.sh ./cmd/... ./pkg/...

.PHONY: test-cov
test-cov:
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/test-cover.sh ./cmd/... ./pkg/...

.PHONY: test-clean
test-clean:
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/test-cover-clean.sh

.PHONY: verify
verify: check format test

.PHONY: verify-extended
verify-extended: check-generate check format test-cov test-clean

#####################################################################
# Rules for local environment                                       #
#####################################################################
kind-up kind-down: export KUBECONFIG = $(GARDENER_LOCAL_KUBECONFIG)

# FIXME: This code is eventually necessary, but until https://github.com/gardener/gardener/pull/7247 is released, the deployment will fail.
$(GGTARBALL):
	echo $(GGTARBALLURL) | xargs curl -Lo /tmp/ggtarball.tgz
$(GGARCHIVE): $(GGTARBALL)
	mkdir $(GGARCHIVE) || true
	tar xfz /tmp/ggtarball.tgz -C $(GGARCHIVE) --strip-components=1 # --wildcards */hack/* */example/* */VERSION

kind-up: $(GGARCHIVE) $(KIND) $(KUBECTL) $(HELM)
	@$(GGARCHIVE)/hack/kind-up.sh --chart $(GGARCHIVE)/example/gardener-local/kind/cluster --cluster-name gardener-local --environment $(KIND_ENV) --path-kubeconfig $(GGARCHIVE)/example/provider-local/seed-kind/base/kubeconfig --path-cluster-values $(GGARCHIVE)/example/gardener-local/kind/local/values.yaml
kind-down: $(GGARCHIVE) $(KIND)
	@$(GGARCHIVE)/hack/kind-down.sh --cluster-name gardener-local --path-kubeconfig $(GGARCHIVE)/example/provider-local/seed-kind/base/kubeconfig

# speed-up skaffold deployments by building all images concurrently
export SKAFFOLD_BUILD_CONCURRENCY = 0
# use static label for skaffold to prevent rolling all gardener components on every `skaffold` invocation
gardener-up: export SKAFFOLD_LABEL = skaffold.dev/run-id=gardener-local
# set ldflags for skaffold
gardener-up:  export LD_FLAGS = $(shell $(GGARCHIVE)/hack/get-build-ld-flags.sh)

gardener-up: $(GGARCHIVE) $(SKAFFOLD) $(HELM) $(KUBECTL)
	SKAFFOLD_DEFAULT_REPO=localhost:5001 SKAFFOLD_PUSH=true $(SKAFFOLD) run --kube-context=kind-gardener-local


.PHONY: integration-test-infra
integration-test-infra:
	@go test -timeout=0 -mod=vendor ./test/integration/infrastructure \
		--v -ginkgo.v -ginkgo.progress \
		--kubeconfig=${KUBECONFIG} \
		--nsxt-host="${NSXT_HOST}" \
		--nsxt-username="${NSXT_USERNAME}" \
		--nsxt-password="${NSXT_PASSWORD}" \
		--nsxt-transport-zone="${NSXT_TRANSPORT_ZONE}" \
		--nsxt-t0-gateway="${NSXT_T0_GATEWAY}" \
		--nsxt-edge-cluster="${NSXT_EDGE_CLUSTER}" \
		--nsxt-snat-ip-pool="${NSXT_SNAT_IP_POOL}"

.PHONY: gcve-integration-test-infra
gcve-integration-test-infra:
	@go test -timeout=0 -mod=vendor ./test/integration/infrastructure \
		--v -ginkgo.v -ginkgo.progress \
		--kubeconfig=${KUBECONFIG} \
		--nsxt-host="${NSXT_HOST}" \
		--nsxt-username="${NSXT_USERNAME}" \
		--nsxt-password="${NSXT_PASSWORD}" \
		--nsxt-transport-zone="${NSXT_TRANSPORT_ZONE}" \
		--nsxt-t0-gateway="${NSXT_T0_GATEWAY}" \
		--nsxt-edge-cluster="${NSXT_EDGE_CLUSTER}" \
		--nsxt-snat-ip-pool="${NSXT_SNAT_IP_POOL}" \
		--openvpn-config="${OVPN_CONFIG}"

.PHONY: initial-test-infra
initial-test-infra:
	@dlv test  --api-version=2 --headless -l 127.0.0.1:2345 ./test/initial/infrastructure --build-flags="-mod=vendor" -- \
		--kubeconfig=${KUBECONFIG} \
		--nsxt-host="${NSXT_HOST}" \
		--nsxt-username="${NSXT_USERNAME}" \
		--nsxt-password="${NSXT_PASSWORD}" \
		--nsxt-transport-zone="${NSXT_TRANSPORT_ZONE}" \
		--nsxt-t0-gateway="${NSXT_T0_GATEWAY}" \
		--nsxt-edge-cluster="${NSXT_EDGE_CLUSTER}" \
		--nsxt-snat-ip-pool="${NSXT_SNAT_IP_POOL}" \
		--openvpn-config="${OVPN_CONFIG}"

#################################################################
# build infra-cli                                               #
#################################################################

.PHONY: install-infra-cli
install-infra-cli:
	@go install -mod=vendor ./cmd/infra-cli

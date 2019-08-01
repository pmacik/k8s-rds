# It's necessary to set this because some environments don't link sh -> bash.
SHELL := /bin/bash

#-----------------------------------------------------------------------------
# VERBOSE target
#-----------------------------------------------------------------------------

# When you run make VERBOSE=1 (the default), executed commands will be printed
# before executed. If you run make VERBOSE=2 verbose flags are turned on and
# quiet flags are turned off for various commands. Use V_FLAG in places where
# you can toggle on/off verbosity using -v. Use Q_FLAG in places where you can
# toggle on/off quiet mode using -q. Use S_FLAG where you want to toggle on/off
# silence mode using -s...
VERBOSE ?= 1
Q = @
Q_FLAG = -q
QUIET_FLAG = --quiet
V_FLAG =
S_FLAG = -s
X_FLAG =
ifeq ($(VERBOSE),1)
	Q =
endif
ifeq ($(VERBOSE),2)
	Q =
	Q_FLAG =
	QUIET_FLAG =
	S_FLAG =
	V_FLAG = -v
	X_FLAG = -x
endif

IMAGE_NAME ?= quay.io/$(QUAY_USERNAME)/k8s-rds
TAG ?= latest
IMAGE = $(IMAGE_NAME):$(TAG)

NAMESPACE ?= default
DB_NAME ?= mydb

.DEFAULT_GOAL := help

## -- Utility targets --

## Print help message for all Makefile targets
## Run `make` or `make help` to see the help
.PHONY: help
help: ## Credit: https://gist.github.com/prwhite/8168133#gistcomment-2749866

	@printf "Usage:\n  make <target>";

	@awk '{ \
			if ($$0 ~ /^.PHONY: [a-zA-Z\-\_0-9]+$$/) { \
				helpCommand = substr($$0, index($$0, ":") + 2); \
				if (helpMessage) { \
					printf "\033[36m%-20s\033[0m %s\n", \
						helpCommand, helpMessage; \
					helpMessage = ""; \
				} \
			} else if ($$0 ~ /^[a-zA-Z\-\_0-9.]+:/) { \
				helpCommand = substr($$0, 0, index($$0, ":")); \
				if (helpMessage) { \
					printf "\033[36m%-20s\033[0m %s\n", \
						helpCommand, helpMessage; \
					helpMessage = ""; \
				} \
			} else if ($$0 ~ /^##/) { \
				if (helpMessage) { \
					helpMessage = helpMessage"\n                     "substr($$0, 3); \
				} else { \
					helpMessage = substr($$0, 3); \
				} \
			} else { \
				if (helpMessage) { \
					print "\n                     "helpMessage"\n" \
				} \
				helpMessage = ""; \
			} \
		}' \
		$(MAKEFILE_LIST)

.PHONY: dep
## Runs 'dep ensure -v'
dep:
	$(Q)dep ensure $(V_FLAG)

## -- Build targets --

.PHONY: build
## Compile the operator for Linux/AMD64
build:
	$(Q)GO111MODULE=off go build $(V_FLAG)

.PHONY: build-image
## Build the operator image
build-image: build
	$(Q)podman build -t $(IMAGE) .

.PHONY: push-image
## Push the operator image to quay.io
push-image:
	@podman login -u "$(QUAY_USERNAME)" -p "$(QUAY_PASSWORD)" quay.io
	$(Q)podman push $(IMAGE)

.PHONY: clean
## Clean up 
clean:
	@rm -rvf k8s-rds

## -- Install/Delete targets --

.PHONY: install-operator
## Create secret, role, account for operator
install-operator:
	$(Q)oc apply -f deploy/operator-cluster-role.yaml
	$(Q)oc apply -f deploy/operator-service-account.yaml
	$(Q)oc apply -f deploy/operator-cluster-role-binding.yaml
	$(Q)oc apply -f deploy/aws.secret.yaml

.PHONY: uninstall-operator
## Delete secret, role, account for operator
uninstall-operator:
	$(Q)-oc delete -f deploy/aws.secret.yaml
	$(Q)-oc delete -f deploy/operator-cluster-role-binding.yaml
	$(Q)-oc delete -f deploy/operator-service-account.yaml
	$(Q)-oc delete -f deploy/operator-cluster-role.yaml

.PHONY: deploy-operator
## Create deployment for operator
deploy-operator:
	$(Q)oc apply -f deploy/deployment-rbac.yaml

.PHONY: undeploy-operator
## Delete deployment for operator
undeploy-operator:
	$(Q)-oc delete -f deploy/deployment-rbac.yaml

.PHONY: deploy-db
## Create database secret and deployment
deploy-db:
	$(Q)oc apply -f deploy/db.secret.yaml
	$(Q)oc apply -f deploy/db.yaml

.PHONY: undeploy-db
## Delete database secret, deployment and service
undeploy-db:
	$(Q)-oc delete -f deploy/db.yaml
	$(Q)-oc delete -f deploy/db.secret.yaml
	$(Q)-oc delete svc $(DB_NAME) -n $(NAMESPACE)

.PHONY: undeploy-all
## Undeploy operator and related assets
undeploy-iall: undeploy-db undeploy-operator

.PHONY: run-locally
## Run the operator locally
run-locally:
	$(Q)./k8s-rds

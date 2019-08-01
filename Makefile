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
	dep ensure -v

.PHONY: build
## Compile the operator for Linux/AMD64
build:
	GO111MODULE=off go build

.PHONY: build-image
## Build the operator image
build-image: build
	podman build -t $(IMAGE) .

.PHONY: push-image
## Push the operator image to quay.io
push-image:
	@podman login -u "$(QUAY_USERNAME)" -p "$(QUAY_PASSWORD)" quay.io
	podman push $(IMAGE)

.PHONY: clean
## Clean up 
clean:
	@rm -rvf k8s-rds

.PHONY: install-operator
## Create secret, role, account for operator
install-operator:
	oc apply -f deploy/operator-cluster-role.yaml
	oc apply -f deploy/operator-service-account.yaml
	oc apply -f deploy/operator-cluster-role-binding.yaml
	oc apply -f deploy/aws.secret.yaml

.PHONY: uninstall-operator
## Delete secret, role, account for operator
uninstall-operator:
	-oc delete -f deploy/aws.secret.yaml
	-oc delete -f deploy/operator-cluster-role-binding.yaml
	-oc delete -f deploy/operator-service-account.yaml
	-oc delete -f deploy/operator-cluster-role.yaml

.PHONY: deploy-operator
## Create deployment for operator
deploy-operator:
	oc apply -f deploy/deployment-rbac.yaml

.PHONY: undeploy-operator
## Delete deployment for operator
undeploy-operator:
	-oc delete -f deploy/deployment-rbac.yaml

.PHONY: deploy-db
## Create database secret and deployment
deploy-db:
	oc apply -f deploy/db.secret.yaml
	oc apply -f deploy/db.yaml

.PHONY: undeploy-db
## Delete database secret, deployment and service
undeploy-db:
	-oc delete -f deploy/db.yaml
	-oc delete -f deploy/db.secret.yaml
	-oc delete svc $(DB_NAME) -n $(NAMESPACE)

.PHONY: undeploy-all
## Undeploy operator and related assets
undeploy-iall: undeploy-db undeploy-operator

.PHONY: run-locally
## Run the operator locally
run-locally:
	./k8s-rds

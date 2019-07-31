
IMAGE_NAME ?= quay.io/$(QUAY_USERNAME)/k8s-rds
TAG ?= latest
IMAGE = $(IMAGE_NAME):$(TAG)

NAMESPACE ?= default
DB_NAME ?= mydb

.PHONY: dep
dep:
	dep ensure -v

.PHONY: build
build:
	GO111MODULE=off go build

.PHONY: build-image
build-image: build
	podman build -t $(IMAGE) .

.PHONY: push-image
push-image:
	@podman login -u "$(QUAY_USERNAME)" -p "$(QUAY_PASSWORD)" quay.io
	podman push $(IMAGE)

.PHONY: clean
clean:
	@rm -rvf k8s-rds

.PHONY: install-operator
install-operator:
	oc apply -f deploy/operator-cluster-role.yaml
	oc apply -f deploy/operator-service-account.yaml
	oc apply -f deploy/operator-cluster-role-binding.yaml
	oc apply -f deploy/aws.secret.yaml

.PHONY: uninstall-operator
uninstall-operator:
	-oc delete -f deploy/aws.secret.yaml
	-oc delete -f deploy/operator-cluster-role-binding.yaml
	-oc delete -f deploy/operator-service-account.yaml
	-oc delete -f deploy/operator-cluster-role.yaml

.PHONY: deploy-operator
deploy-operator:
	oc apply -f deploy/deployment-rbac.yaml

.PHONY: undeploy-operator
undeploy-operator:
	-oc delete -f deploy/deployment-rbac.yaml

.PHONY: deploy-db
deploy-db:
	oc apply -f deploy/db.secret.yaml
	oc apply -f deploy/db.yaml

.PHONY: undeploy-db
undeploy-db:
	-oc delete -f deploy/db.yaml
	-oc delete -f deploy/db.secret.yaml

.PHONY: undeploy-all
undeploy-all: undeploy-db undeploy-operator

.PHONY: run-locally
run-locally:
	./k8s-rds --provider local

IMAGE_NAME ?= quay.io/$(QUAY_USERNAME)/k8s-rds
TAG ?= latest
IMAGE = $(IMAGE_NAME):$(TAG)


.PHONY: build
build:
	go build

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
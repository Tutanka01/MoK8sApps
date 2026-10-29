REGISTRY  ?= tutanka01
TAG       ?= 0.1.0
PLATFORMS ?= linux/amd64,linux/arm64
APPS      := pouet kaboom miam

.PHONY: build push deploy undeploy $(addprefix build-,$(APPS)) $(addprefix push-,$(APPS))

## build: build every image for the local architecture
build: $(addprefix build-,$(APPS))

$(addprefix build-,$(APPS)): build-%:
	docker build --build-arg VERSION=$(TAG) -t $(REGISTRY)/$*:$(TAG) $*

## push: build multi-arch images and push them to $(REGISTRY)
push: $(addprefix push-,$(APPS))

$(addprefix push-,$(APPS)): push-%:
	docker buildx build --platform $(PLATFORMS) --build-arg VERSION=$(TAG) -t $(REGISTRY)/$*:$(TAG) --push $*

## deploy: apply every demo app to the current kubectl context
deploy:
	kubectl apply -k .

undeploy:
	kubectl delete -k .

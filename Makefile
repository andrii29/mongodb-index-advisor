.PHONY: build run

IMAGE_NAME = mongodb-index-advisor
IMAGE_VERSION = latest

IMAGE_DOCKERHUB = andriik/$(IMAGE_NAME)

prepare:
	go mod tidy
run:
	go run . -h


docker-build:
	docker build -t $(IMAGE_NAME):$(IMAGE_VERSION) .
docker-run:
	docker run --rm --name $(IMAGE_NAME) $(IMAGE_NAME):$(IMAGE_VERSION) -h

# dockerhub
docker-build-dockerhub:
	docker build -t $(IMAGE_DOCKERHUB):$(IMAGE_VERSION) .

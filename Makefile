IMAGE_PREFIX ?= owend
GIT_REVISION := $(shell git rev-parse --short HEAD)

docker:
	docker build . -t $(IMAGE_PREFIX)/loki-s3-rate-limit-experiment:$(GIT_REVISION)

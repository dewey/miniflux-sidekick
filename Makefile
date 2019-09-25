IMAGE_NAME := dewey/miniflux-sidekick
VERSION_DOCKER := $(shell git describe --abbrev=0 --tags  | sed 's/^v\(.*\)/\1/')

all: install

install:
	go install -v

test:
	go test ./... -v

image-push-staging:
	docker build -t docker.pkg.github.com/dewey/miniflux-sidekick/$(IMAGE_NAME):staging .
	docker push docker.pkg.github.com/dewey/miniflux-sidekick/$(IMAGE_NAME):staging

image-push:
	docker build -t docker.pkg.github.com/dewey/miniflux-sidekick/$(IMAGE_NAME):latest .
	docker tag docker.pkg.github.com/dewey/miniflux-sidekick/$(IMAGE_NAME):latest docker.pkg.github.com/dewey/miniflux-sidekick/$(IMAGE_NAME):$(VERSION_DOCKER)
	docker push docker.pkg.github.com/dewey/miniflux-sidekick/$(IMAGE_NAME):latest
	docker push docker.pkg.github.com/dewey/miniflux-sidekick/$(IMAGE_NAME):$(VERSION_DOCKER)

release:
	git tag -a $(VERSION) -m "Release $(VERSION)" || true
	git push origin $(VERSION)

.PHONY: install test



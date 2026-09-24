VERSION=$(shell git describe --tags --exact-match 2>/dev/null || git rev-parse HEAD)
DEVELOPMENT_VERSION=development

test:
	ginkgo -r

generate:
	go generate ./...

build:
	docker build --build-arg VERSION=$(VERSION) -t ghcr.io/airhelp-osp/autoscaler:$(VERSION) .

release: build
	docker push ghcr.io/airhelp-osp/autoscaler:$(VERSION)

	docker tag ghcr.io/airhelp-osp/autoscaler:$(VERSION) ghcr.io/airhelp-osp/autoscaler:latest
	docker push ghcr.io/airhelp-osp/autoscaler:latest

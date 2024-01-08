VERSION=$(shell cat version.txt)
DEVELOPMENT_VERSION=development

test:
	ginkgo -r

generate:
	go generate ./...

build:
	docker build -t ghcr.io/airhelp-osp/autoscaler:$(VERSION) .

release: build
	docker push ghcr.io/airhelp-osp/autoscaler:$(VERSION)

	docker tag ghcr.io/airhelp-osp/autoscaler:$(VERSION) ghcr.io/airhelp-osp/autoscaler:latest
	docker push ghcr.io/airhelp-osp/autoscaler:latest

VERSION=$(shell cat version.txt)
DEVELOPMENT_VERSION=development

test:
	ginkgo -r

generate:
	go generate ./...

build:
	docker build -t airhelp/autoscaler:$(VERSION) .

release: build
	docker push airhelp/autoscaler:$(VERSION)

	docker tag airhelp/autoscaler:$(VERSION) airhelp/autoscaler:latest
	docker push airhelp/autoscaler:latest

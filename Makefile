DT := $(shell date +%Y.%m.%d.%H.%M.%S)
HASH := $(shell git rev-parse HEAD)
USER := $(shell whoami)
KUBECONFIG := ~/.kube/config
name := netmux

version:
	echo $(DT) > ./foundation/buildinfo/build-date
	cp .semver ./foundation/buildinfo/build-semver
	git rev-parse HEAD > ./foundation/buildinfo/build-hash

lint:
	find . -name '*.go'  | xargs -I{} gofumpt -w {}
	golangci-lint run ./...

test:
	go test ./...

test-race:
	go test -race ./...


sample-server: version
	CGO_ENABLED=0 GOOS=linux go build -ldflags="-extldflags=-static" -o ./zarf/docker/helpers/sample-service/service ./app/helpers/sample-service
	docker build -t digitalcircle/sample-service:$(HASH) -t digitalcircle/sample-service:latest -f ./zarf/docker/helpers/sample-service/Dockerfile .
#	KUBECONFIG=$(KUBECONFIG) kubectl --namespace netmux scale deployment sample --replicas=0
#	KUBECONFIG=$(KUBECONFIG) kubectl --namespace netmux scale deployment sample --replicas=1


docker-img-local-arm64: version

	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o ./zarf/docker/netmux/bin/linux/arm64/$(name) ./app/nx-server

	docker rmi -f duxthemux/$(name):latest
	docker build -f ./zarf/docker/netmux/Dockerfile -t duxthemux/$(name):latest --platform=linux/arm64  .

docker-img-local: version

	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ./zarf/docker/netmux/bin/linux/amd64/$(name) ./app/nx-server
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o ./zarf/docker/netmux/bin/linux/arm64/$(name) ./app/nx-server

	docker rmi -f duxthemux/$(name):latest
	docker buildx build -f ./zarf/docker/netmux/Dockerfile -t duxthemux/$(name):latest --platform=linux/arm64,linux/amd64  . --load

docker-img: version

	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ./zarf/docker/netmux/bin/linux/amd64/$(name) ./app/nx-server
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o ./zarf/docker/netmux/bin/linux/arm64/$(name) ./app/nx-server

	upx --best --lzma ./zarf/docker/netmux/bin/linux/amd64/$(name)
	upx --best --lzma ./zarf/docker/netmux/bin/linux/arm64/$(name)

	docker rmi -f duxthemux/$(name):latest
	docker buildx build -f ./zarf/docker/netmux/Dockerfile -t duxthemux/$(name):latest --platform=linux/arm64,linux/amd64  . --push

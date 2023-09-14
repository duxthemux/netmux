DT := $(shell date +%Y.%m.%d.%H%M%S)
HASH := $(shell git rev-parse HEAD)
USER := $(shell whoami)
KUBECONFIG := ~/.kube/config


version:
	echo $(DT) > ./foundation/buildinfo/build-date
	cp .semver ./foundation/buildinfo/build-semver
	git rev-parse HEAD > ./foundation/buildinfo/build-hash

lint:
	find . -name '*.go'  | xargs -I{} gofumpt -w {}
	golangci-lint run ./...

test:
	go test ./...


sample-server: version
	CGO_ENABLED=0 GOOS=linux go build -ldflags="-extldflags=-static" -o ./zarf/docker/helpers/sample-service/service ./app/helpers/sample-service
	docker build -t digitalcircle/sample-service:$(HASH) -t digitalcircle/sample-service:latest -f ./zarf/docker/helpers/sample-service/Dockerfile .
#	KUBECONFIG=$(KUBECONFIG) kubectl --namespace netmux scale deployment sample --replicas=0
#	KUBECONFIG=$(KUBECONFIG) kubectl --namespace netmux scale deployment sample --replicas=1

netmux-server-arm64: version
	CGO_ENABLED=0 GOOS=linux go build -ldflags="-extldflags=-static" -o ./zarf/docker/nx-server/service ./app/nx-server
	docker build -t digitalcircle/netmux-arm64:$(HASH) -t digitalcircle/netmux-arm64:latest -f ./zarf/docker/nx-server/Dockerfile .
#	KUBECONFIG=$(KUBECONFIG) kubectl --namespace netmux scale deployment netmux --replicas=0
#	KUBECONFIG=$(KUBECONFIG) kubectl --namespace netmux scale deployment netmux --replicas=1
	docker push digitalcircle/netmux-arm64:latest

netmux-server-amd64: version
	CGO_ENABLED=0 GOOS=linux go build -ldflags="-extldflags=-static" -o ./zarf/docker/nx-server/service ./app/nx-server
	docker build -t digitalcircle/netmux-amd64:$(HASH) -t digitalcircle/netmux-amd64:latest -f ./zarf/docker/nx-server/Dockerfile .
#	KUBECONFIG=$(KUBECONFIG) kubectl --namespace netmux scale deployment netmux --replicas=0
#	KUBECONFIG=$(KUBECONFIG) kubectl --namespace netmux scale deployment netmux --replicas=1
	docker push digitalcircle/netmux-amd64:latest

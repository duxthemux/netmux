DT := $(shell date +%Y.%m.%d.%H.%M.%S)
HASH := $(shell git rev-parse HEAD)
USER := $(shell whoami)
SEMVER := $(shell cat .semver)
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

docker-img-local-amd64: version
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ./zarf/docker/netmux/bin/linux/amd64/$(name) ./app/nx-server

	- docker rmi -f duxthemux/$(name):latest
	docker build -f ./zarf/docker/netmux/Dockerfile -t duxthemux/$(name):latest --platform=linux/arm64  . --load


docker-img-local-arm64: version
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o ./zarf/docker/netmux/bin/linux/arm64/$(name) ./app/nx-server

	- docker rmi -f duxthemux/$(name):latest
	docker build -f ./zarf/docker/netmux/Dockerfile -t duxthemux/$(name):latest --platform=linux/arm64  . --load


docker-img: version
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ./zarf/docker/netmux/bin/linux/amd64/$(name) ./app/nx-server
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o ./zarf/docker/netmux/bin/linux/arm64/$(name) ./app/nx-server

	upx --best --lzma ./zarf/docker/netmux/bin/linux/amd64/$(name)
	upx --best --lzma ./zarf/docker/netmux/bin/linux/arm64/$(name)

	- docker rmi -f duxthemux/$(name):latest
	docker buildx build -f ./zarf/docker/netmux/Dockerfile -t duxthemux/$(name):latest --platform=linux/arm64,linux/amd64  . --push

my-bins:
	go build -ldflags="-s -w" -o zarf/dist/nx ./app/nx-cli
	go build -ldflags="-s -w" -o zarf/dist/nx-daemon ./app/nx-daemon

dist-bins:
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o zarf/dist/darwin_arm64/nx ./app/nx-cli
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o zarf/dist/darwin_arm64/nx-daemon ./app/nx-daemon

	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o zarf/dist/darwin_amd64/nx ./app/nx-cli
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o zarf/dist/darwin_amd64/nx-daemon ./app/nx-daemon

	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o zarf/dist/linux_arm64/nx ./app/nx-cli
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o zarf/dist/linux_arm64/nx-daemon ./app/nx-daemon

	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o zarf/dist/linux_amd64/nx ./app/nx-cli
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o zarf/dist/linux_amd64/nx-daemon ./app/nx-daemon

	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o zarf/dist/windows_amd64/nx.exe ./app/nx-cli
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o zarf/dist/windows_amd64/nx-daemon.exe ./app/nx-daemon

	tar czvfp zarf/dist/netmuxcli-$(SEMVER)-darwin_arm64.tgz -C zarf/dist/darwin_arm64 .
	tar czvfp zarf/dist/netmuxcli-$(SEMVER)-darwin_amd64.tgz -C zarf/dist/darwin_amd64 .
	tar czvfp zarf/dist/netmuxcli-$(SEMVER)-linux_arm64.tgz -C zarf/dist/linux_arm64 .
	tar czvfp zarf/dist/netmuxcli-$(SEMVER)-linux_amd64.tgz -C zarf/dist/linux_amd64 .
	cd zarf/dist/windows_amd64 && zip -r ../netmuxcli-$(SEMVER)-windows_amd64.zip . && cd -
# -------------
docker-init-buildx:
	docker buildx create --use

sample-server:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ./zarf/docker/helpers/sample-service/bin/linux/amd64/service ./zarf/sample-apps/sample-service
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o ./zarf/docker/helpers/sample-service/bin/linux/arm64/service ./zarf/sample-apps/sample-service

	upx --best --lzma ./zarf/docker/helpers/sample-service/bin/linux/amd64/service
	upx --best --lzma ./zarf/docker/helpers/sample-service/bin/linux/arm64/service


	- docker rmi -f duxthemux/sample-service:latest
	docker buildx build -f ./zarf/docker/helpers/sample-service/Dockerfile -t duxthemux/sample-service:latest --platform=linux/arm64,linux/amd64  . --push
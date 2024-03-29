files := $(shell find . -path ./vendor -prune -path ./pb -prune -o -name '*.go' -print)
pkgs := $(shell go list ./... | grep -v /vendor/ )

git_rev := $(shell git rev-parse --short HEAD)
git_tag := $(shell git tag --points-at=$(git_rev))
release_date := $(shell date +%d-%m-%y)
latest_git_tag := $(shell git for-each-ref --format="%(tag)" --sort=-taggerdate refs/tags | head -1)
latest_git_rev := $(shell git rev-list --abbrev-commit -n 1 $(latest_git_tag))
version := $(if $(git_tag),$(git_tag),dev-$(git_rev))
build_time := $(shell date -u)
ldflags := -X "github.com/sky-uk/osprey/v2/cmd.version=$(version)" -X "github.com/sky-uk/osprey/v2/cmd.buildTime=$(build_time)"

cwd= $(shell pwd)
build_dir := $(cwd)/build/bin
dist_dir := $(cwd)/dist

# Define cross compiling targets
os := $(shell uname)
ifeq ("$(os)", "Linux")
	target_os = linux
else ifeq ("$(os)", "Darwin")
	target_os = darwin
endif

.PHONY: all build check check-format check-os clean docker format install lint proto release-docker setup test vet

all : check install test
check : check-os check-format vet lint test
travis : clean setup check build test docker

check-os:
ifndef target_os
	$(error Unsupported platform: ${os})
endif

setup:
	@echo "== setup"
	go install golang.org/x/lint/golint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go mod download

format :
	@echo "== format"
	@goimports -w $(files)
	@sync

clean :
	@echo "== clean"
	rm -rf build
	rm -rf dist

build :
	@echo "== build"
	GOOS=${target_os} GOARCH=amd64 go build -ldflags '-s $(ldflags)' -o ${build_dir}/${target_os}_amd64/osprey -v

install :
	@echo "== install"
	@echo "Installing binary for ${target_os}"
	GOOS=${target_os} GOARCH=amd64 go install -ldflags '$(ldflags)' -v

unformatted = $(shell goimports -l $(files))

check-format :
	@echo "== check formatting"
	@if [ "`goimports -l $(files)`" != "" ]; then \
		echo "code needs formatting. Run make format"; \
		exit 1; \
	fi;

vet :
	@echo "== vet"
	@go vet $(pkgs)

lint :
	@echo "== lint"
	@for pkg in $(pkgs); do \
		golint -set_exit_status $$pkg || exit 1 ; \
	done;

test :
	@echo "== run tests"
	go test -v -race $(pkgs)

proto :
	@echo "== compiling proto files"
	@docker run -v `pwd`/common/pb:/pb -w / grpc/go:1.0 protoc -I /pb /pb/osprey.proto --go_out=plugins=grpc:pb

image := skycirrus/osprey

docker : build
	@echo "== docker"
	docker build -t $(image):latest .

release-docker : docker
ifeq ($(strip $(git_tag)),)
	@echo "no tag on $(git_rev), skipping docker release"
else
	@echo "== release docker"
	@echo "releasing $(image):$(git_tag)"
	@docker login -u $(DOCKER_USERNAME) -p $(DOCKER_PASSWORD)
	docker tag $(image):latest $(image):$(git_tag)
	docker push $(image):$(git_tag)
	@if [ "$(git_rev)" = "$(latest_git_rev)" ]; then \
		echo "updating latest image"; \
		echo docker push $(image):latest ; \
	fi;
endif

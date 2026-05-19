.PHONY: build run clean test lint fmt docker-build docker-run gen-rsa

APP_NAME=lic
VERSION?=dev
GOFLAGS?=

build:
	go build -ldflags "-X main.Version=$(VERSION)" -o $(APP_NAME) .

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X main.Version=$(VERSION)" -o $(APP_NAME) .

run: build
	./$(APP_NAME)

clean:
	rm -f $(APP_NAME)
	rm -f cmd/example_client/example_client
	go clean

test:
	go test ./... -v

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

# 生成 RSA 密钥对（用于离线激活签名）
gen-rsa:
	@echo "生成 RSA 密钥对..."
	openssl genrsa -out rsa_private.pem 2048
	openssl rsa -in rsa_private.pem -pubout -out rsa_public.pem
	@echo "密钥已生成: rsa_private.pem, rsa_public.pem"

# Docker 构建
docker-build:
	docker build -t $(APP_NAME):$(VERSION) .

# Docker 运行
docker-run:
	docker run --rm -p 8080:8080 \
		-e DB_TYPE=$$DB_TYPE \
		-e DB_DSN=$$DB_DSN \
		-e JWT_SECRET=$$JWT_SECRET \
		-e ADMIN_PASS_HASH=$$ADMIN_PASS_HASH \
		-v $$(pwd)/rsa_private.pem:/app/rsa_private.pem \
		-v $$(pwd)/rsa_public.pem:/app/rsa_public.pem \
		$(APP_NAME):$(VERSION)

# 交叉编译
build-all: build-linux

# 安装到 GOPATH/bin
install:
	go install .

# 下载依赖
deps:
	go mod download
	go mod tidy

# ---------- 构建阶段 ----------
FROM golang:1.25-alpine AS builder

WORKDIR /app

# 复制依赖文件优先（利用 Docker 缓存）
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .

# 编译静态二进制（无 CGO，适合 Alpine）
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o lic .

# ---------- 运行阶段 ----------
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# 从构建阶段复制二进制
COPY --from=builder /app/lic .

# 复制模板、静态资源、RSA 密钥（如有）
COPY templates/  ./templates/
COPY static/     ./static/
COPY rsa_private.pem ./rsa_private.pem
COPY rsa_public.pem  ./rsa_public.pem

# 设置时区为中国大陆时间
ENV TZ=Asia/Shanghai

# 暴露端口（与 main.go 中 gin 监听端口一致）
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["./lic"]

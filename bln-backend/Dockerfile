# 阶段1：编译
FROM golang:1.25.6 AS builder
WORKDIR /app
ENV GOPROXY=https://goproxy.cn,direct

# 复制项目文件
COPY go.mod go.sum ./
# 下载依赖
RUN go mod download
# 复制项目文件
COPY . .
# 编译项目
RUN CGO_ENABLED=0 go build -o bloom-nft .

# 阶段2：运行（仅二进制 + 小基础镜像）
FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
# 复制编译好的二进制文件
COPY --from=builder /app/bloom-nft .
# 复制配置文件
COPY --from=builder /app/config ./config
# 运行
CMD ["./bloom-nft"]
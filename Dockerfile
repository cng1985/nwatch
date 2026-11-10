# 前端构建：产物进入 web/dist，由 Go 嵌入。
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

# 后端编译：嵌入前端产物，注入版本信息。
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/nwatch ./cmd/server

# 运行镜像：默认 dev profile，生产环境通过 APP_PROFILE=pro 与环境变量注入凭据。
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /opt/nwatch
COPY --from=build /out/nwatch /opt/nwatch/nwatch
COPY configs/ /opt/nwatch/configs/
VOLUME /var/lib/nwatch
ENV APP_PROFILE=dev APP_DATA_DIR=/var/lib/nwatch APP_CONFIG_DIR=/opt/nwatch/configs
EXPOSE 8080
ENTRYPOINT ["/opt/nwatch/nwatch"]

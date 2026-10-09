FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/internal/web/dist ./internal/web/dist
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /out/nmonitor ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /opt/nmonitor
COPY --from=build /out/nmonitor /opt/nmonitor/nmonitor
COPY config.yaml /opt/nmonitor/config.yaml
VOLUME /opt/nmonitor/data
EXPOSE 8080
ENTRYPOINT ["/opt/nmonitor/nmonitor"]

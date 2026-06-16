FROM node:24.16.0-alpine AS build-web

WORKDIR /app

COPY web/package.json ./
COPY web/package-lock.json ./
RUN npm i

COPY web .
RUN npm run build

FROM golang:1.26-alpine AS build-server

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY sdk sdk
COPY internal internal
COPY cmd cmd
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w"  -o ./server ./cmd/server/

FROM alpine:3.24.1

RUN apk add --no-cache tzdata
ENV TZ=Asia/Shanghai
RUN cp /usr/share/zoneinfo/${TZ} /etc/localtime && echo "${TZ}" > /etc/timezone

RUN mkdir /app
COPY --from=build-server /app/server /app/server
COPY --from=build-web /app/dist/index.html /app/management.html
ENV MANAGEMENT_STATIC_PATH=/app

COPY docker/entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]

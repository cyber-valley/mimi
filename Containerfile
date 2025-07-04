FROM docker.io/golang:1.24 AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=1
ENV CGO_LDFLAGS="-L/app/libs"
ENV GOOS=linux
RUN go build -o app -ldflags "-linkmode external -extldflags=-static" ./cmd/app/main.go

FROM docker.io/alpine:edge

WORKDIR /app

COPY --from=build /app/app .
COPY prompts prompts
COPY sql/migrations sql/migrations

# Telegram scraper session
COPY session/prod.json /app/td-session.json

RUN apk upgrade
RUN apk --no-cache add gcompat ca-certificates tzdata git

ENTRYPOINT ["/app/app"]

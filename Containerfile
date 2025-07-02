FROM docker.io/golang:1.24 AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=1
ENV CGO_LDFLAGS="-L/app/libs"
ENV GOOS=linux
RUN go build -o app -ldflags "-linkmode external -extldflags=-static" ./cmd/bot/main.go

FROM docker.io/alpine:edge

WORKDIR /app

COPY --from=build /app/app .
COPY prompts prompts
COPY cozo.db .
COPY sql/migrations sql/migrations

RUN apk upgrade
RUN apk --no-cache add gcompat ca-certificates tzdata nodejs npm
RUN npm install --global logseq-query

ENTRYPOINT ["/app/app"]

##
## Build
##
FROM golang:1.18-alpine AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY cmd ./cmd/
COPY internal ./internal

RUN go build -o /welcome-bot ./cmd/welcome-bot

##
## Deploy
##
FROM alpine

WORKDIR /

RUN adduser -D welcome-bot

USER welcome-bot

COPY --from=build /welcome-bot /welcome-bot

ENTRYPOINT ["/welcome-bot"]

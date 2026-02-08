FROM golang:1.25-alpine AS build

WORKDIR /src
COPY main.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/txtweb main.go

FROM alpine:3.20

WORKDIR /app
COPY --from=build /out/txtweb /usr/local/bin/txtweb

EXPOSE 80

ENTRYPOINT ["/usr/local/bin/txtweb"]

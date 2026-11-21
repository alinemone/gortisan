# Build from the project root: docker build -f deploy/Dockerfile .
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/layout1 .

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata && adduser -D -H app
COPY --from=build /out/layout1 /usr/local/bin/layout1
COPY deploy/entrypoint.sh /usr/local/bin/entrypoint.sh
USER app
EXPOSE 8000
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
# The command to run: serve, or another one of `layout1 --help`.
CMD ["serve"]

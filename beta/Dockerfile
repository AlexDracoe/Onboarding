# Dev: air rebuilds and restarts the binary whenever compose watch syncs a change in
FROM golang:1.27 AS dev
WORKDIR /app
RUN go install github.com/air-verse/air@latest
COPY go.* ./
RUN go mod download
COPY . .
EXPOSE 8000
CMD ["air", "--build.cmd", "go build -o /tmp/droned ./cmd/droned", "--build.bin", "/tmp/droned"]


# Build a static binary
FROM golang:1.27 AS build
WORKDIR /src
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /droned ./cmd/droned

# Run it (default target: the last stage)
FROM alpine:3.22
RUN adduser -D app && mkdir /data && chown app /data
USER app
COPY --from=build /droned /droned
EXPOSE 8000
CMD ["/droned"]

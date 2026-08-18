FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/ingest ./cmd/ingest \
 && CGO_ENABLED=0 go build -o /out/dashboard ./cmd/dashboard \
 && CGO_ENABLED=0 go build -o /out/worker ./cmd/worker

FROM alpine:3.20
RUN adduser -D -H app
COPY --from=build /out/ /usr/local/bin/
USER app
ENTRYPOINT ["/usr/local/bin/ingest"]
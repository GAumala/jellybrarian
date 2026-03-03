FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /jellybrarian .

# ---

FROM alpine:3.21

RUN adduser -D -u 1000 jellybrarian

USER jellybrarian
WORKDIR /app

COPY --from=builder /jellybrarian /app/jellybrarian

EXPOSE 8090

ENTRYPOINT ["/app/jellybrarian"]

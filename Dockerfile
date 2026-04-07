FROM golang:1.25-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/opportunity-radar ./cmd/app

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates && adduser -D -g '' appuser

COPY --from=builder /out/opportunity-radar /app/opportunity-radar
COPY migrations /app/migrations

USER appuser

EXPOSE 8080

CMD ["./opportunity-radar"]

FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /etl ./cmd/etl

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /etl .

RUN mkdir -p data/raw data/processed logs

EXPOSE 8080

ENTRYPOINT ["/app/etl"]

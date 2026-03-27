FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o set-intersection ./cmd/set-intersection

FROM scratch
COPY --from=builder /app/set-intersection /set-intersection
WORKDIR /
ENTRYPOINT [ "/set-intersection" ]

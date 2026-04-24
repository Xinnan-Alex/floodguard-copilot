FROM golang:alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /server .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /server /server

# Cloud Run injects the PORT environment variable
ENV PORT=8080
EXPOSE 8080

CMD ["/server"]

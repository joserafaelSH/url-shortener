FROM golang:1.26-alpine AS stage1

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o api ./cmd/api

################################################

FROM scratch

COPY --from=stage1 /app/api /

ENTRYPOINT ["/api"]
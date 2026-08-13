FROM golang:1.23-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/consflow ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/consflow-migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/consflow-storage-migrate ./cmd/storage-migrate

FROM alpine:3.20
WORKDIR /app
RUN addgroup -S -g 10001 appgroup \
    && adduser -S -D -H -u 10001 -G appgroup appuser
COPY --from=build /bin/consflow /app/consflow
COPY --from=build /bin/consflow-migrate /app/consflow-migrate
COPY --from=build /bin/consflow-storage-migrate /app/consflow-storage-migrate
COPY --from=build /src/migrations /app/migrations
USER 10001:10001
EXPOSE 8080
CMD ["/app/consflow"]

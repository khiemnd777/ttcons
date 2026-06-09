FROM golang:1.23-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/consflow ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/consflow-migrate ./cmd/migrate

FROM alpine:3.20
WORKDIR /app
RUN adduser -D -g '' appuser
COPY --from=build /bin/consflow /app/consflow
COPY --from=build /bin/consflow-migrate /app/consflow-migrate
COPY --from=build /src/migrations /app/migrations
USER appuser
EXPOSE 8080
CMD ["/app/consflow"]

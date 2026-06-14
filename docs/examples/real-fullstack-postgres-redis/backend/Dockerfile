FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/backend .

FROM alpine:3.22
RUN adduser -D -H app
COPY --from=build /out/backend /backend
USER app
EXPOSE 8080
ENTRYPOINT ["/backend"]

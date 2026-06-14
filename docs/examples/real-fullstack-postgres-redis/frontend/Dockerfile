FROM golang:1.25-alpine AS build
WORKDIR /src
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/frontend main.go

FROM scratch
COPY --from=build /out/frontend /frontend
EXPOSE 80
ENTRYPOINT ["/frontend"]

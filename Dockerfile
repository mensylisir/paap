# Build stage: use local go binary via gvm
# We build the binary on host and copy it in, since go1.25 may not have an official docker image yet

FROM alpine:3.21

WORKDIR /app
COPY bin/paap-server .

EXPOSE 9090

ENV PORT=9090
ENV DATABASE_URL="postgres://paap:paap@paap-postgres.paap.svc.cluster.local:5432/paap?sslmode=disable"
ENV JWT_SECRET="paap-dev-secret-change-in-prod"

RUN chmod +x paap-server

ENTRYPOINT ["./paap-server"]

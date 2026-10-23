# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -mod=vendor -trimpath -ldflags="-s -w" -o /shelly-agent .

FROM scratch
# scratch has no system CA bundle. If verified HTTPS needs system roots, add:
# COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /shelly-agent /shelly-agent
USER 65532:65532
ENTRYPOINT ["/shelly-agent"]

# syntax=docker/dockerfile:1
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -mod=vendor -trimpath -o /shelly-monitoring-ui .

FROM scratch
COPY --from=build /shelly-monitoring-ui /shelly-monitoring-ui
USER 65532:65532
EXPOSE 8090
ENTRYPOINT ["/shelly-monitoring-ui"]

# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -mod=vendor -o /shelly-simulator .

FROM scratch
COPY --from=build /shelly-simulator /shelly-simulator
EXPOSE 8080 8443
ENTRYPOINT ["/shelly-simulator"]

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
COPY static/ static/
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /kaboom .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /kaboom /kaboom
EXPOSE 8080
ENTRYPOINT ["/kaboom"]

FROM golang:1.27.2-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Static binary so it runs on an image with nothing else in it
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /guard .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /guard /guard
USER nonroot
EXPOSE 8080
ENTRYPOINT ["/guard"]

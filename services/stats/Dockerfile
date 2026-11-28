FROM golang:1.27.4-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Static binary so it runs on an image with nothing else in it
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /stats .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /stats /stats
USER nonroot
EXPOSE 8080
ENTRYPOINT ["/stats"]

FROM golang:1.27.4-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/resilienceplane ./cmd/resilienceplane

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/resilienceplane /resilienceplane
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/resilienceplane"]

FROM golang:1.27.4-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/target-reconciliation-service ./cmd/target-reconciliation-service

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/target-reconciliation-service /target-reconciliation-service
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/target-reconciliation-service"]

FROM --platform=${BUILDPLATFORM} golang:1.25 AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/proxy-reconciler ./cmd/controller

FROM scratch

COPY --from=builder /out/proxy-reconciler /usr/local/bin/proxy-reconciler

CMD ["/usr/local/bin/proxy-reconciler"]

# Build the toolchain stage on the BUILDER's native arch (--platform=$BUILDPLATFORM) and
# cross-compile to the target arch via GOOS/GOARCH. Running the Go 1.26 toolchain under QEMU
# emulation (e.g. amd64-on-arm64) crashes the runtime, so we never emulate the compiler — only
# the produced binary targets the requested platform. Build from the repo-root context so the
# embedded chart under charts/ is available.
FROM --platform=$BUILDPLATFORM golang:1.26 AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /out/manager ./operator/cmd

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=build /out/manager /manager
USER 65532:65532
ENTRYPOINT ["/manager"]

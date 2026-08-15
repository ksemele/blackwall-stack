# blackwall-scan — external attack surface scanner
# Multi-stage: builder compiles ProjectDiscovery Go tools, runtime is slim.

# ---- Stage 1: build pd-tools ----
FROM golang:1.25-bookworm AS builder

# libpcap-dev needed to build naabu (SYN scan support)
RUN apt-get update && apt-get install -y --no-install-recommends \
    libpcap-dev git \
    && rm -rf /var/lib/apt/lists/*

ENV GOFLAGS=-buildvcs=false
ENV CGO_ENABLED=1
ENV GOTOOLCHAIN=auto

# NOTE: pinned to @latest for MVP — pin exact versions later (config-as-code).
RUN go install github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest && \
    go install github.com/projectdiscovery/dnsx/cmd/dnsx@latest && \
    go install github.com/projectdiscovery/asnmap/cmd/asnmap@latest && \
    go install github.com/projectdiscovery/naabu/v2/cmd/naabu@latest && \
    go install github.com/projectdiscovery/httpx/cmd/httpx@latest && \
    go install github.com/projectdiscovery/tlsx/cmd/tlsx@latest && \
    go install github.com/projectdiscovery/nuclei/v3/cmd/nuclei@latest

# ---- Stage 2: runtime ----
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    nmap masscan libpcap0.8 ca-certificates curl jq dnsutils bash \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /go/bin/subfinder /usr/local/bin/
COPY --from=builder /go/bin/dnsx      /usr/local/bin/
COPY --from=builder /go/bin/asnmap    /usr/local/bin/
COPY --from=builder /go/bin/naabu     /usr/local/bin/
COPY --from=builder /go/bin/httpx     /usr/local/bin/
COPY --from=builder /go/bin/tlsx      /usr/local/bin/
COPY --from=builder /go/bin/nuclei    /usr/local/bin/

# Pre-download nuclei templates at build time so first run is fast & offline-safe.
# Kept before COPY scan.sh so editing the script doesn't re-trigger the download.
RUN nuclei -update-templates -silent || true

COPY scan.sh /usr/local/bin/scan.sh
RUN chmod +x /usr/local/bin/scan.sh

VOLUME ["/targets", "/output"]
WORKDIR /output

ENTRYPOINT ["/usr/local/bin/scan.sh"]

FROM golang:1.26.11
RUN apt-get update && apt-get install -y --no-install-recommends make git curl ca-certificates && rm -rf /var/lib/apt/lists/*
RUN curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b /usr/local/bin v2.12.2

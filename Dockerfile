# VMPacker Build Environment
FROM debian:trixie-slim

# Install build dependencies
RUN apt-get update && apt-get install -y \
    gcc \
    gcc-aarch64-linux-gnu \
    binutils-aarch64-linux-gnu \
    make \
    python3 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /work

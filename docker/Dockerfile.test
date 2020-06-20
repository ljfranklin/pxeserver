FROM golang:buster

RUN apt-get update && \
  apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    cpio \
    dnsmasq \
    iproute2 \
    iptables \
    qemu-system \
    qemu-utils \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

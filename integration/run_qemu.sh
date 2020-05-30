#!/bin/bash

set -eu

: "${DISK_PATH:?}"

ip link add name br99 type bridge
ip addr add 172.20.0.1/16 dev br99
ip link set br99 up
dnsmasq --interface=br99 --bind-interfaces \
  --dhcp-range=172.20.0.2,172.20.255.254 --dhcp-host=52:54:00:12:34:56,172.20.0.10 \
  --keep-in-foreground &
dnsmasq_pid="$!"

[[ ! -d /etc/qemu ]] && mkdir /etc/qemu
touch /etc/qemu/bridge.conf
if ! grep '^allow br99$' /etc/qemu/bridge.conf > /dev/null; then
  echo "allow br99" >> /etc/qemu/bridge.conf
fi

cleanup() {
  ip link delete br99
  kill "${dnsmasq_pid}"
}
trap cleanup EXIT

qemu-system-x86_64 -smp "$(nproc)" -m 4096 \
  -net nic,model=virtio -net bridge,br=br99 \
  -no-reboot \
  -vnc 0.0.0.0:10 \
  -hda "${DISK_PATH}" &
qemu_pid="$!"
trap '{ kill "${qemu_pid}"; }' SIGINT

wait "${qemu_pid}"

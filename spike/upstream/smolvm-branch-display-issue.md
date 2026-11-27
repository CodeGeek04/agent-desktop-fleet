Title: A `machine branch` child never sends host display frames: its `SMOLVM_VNC` port completes the RFB handshake but no framebuffer update arrives (v1.37.7, macOS and Linux)

**Version:** smolvm 1.37.7

**Hosts:**
- macOS 27.2.1 on Apple M5 (Hypervisor.framework)
- Ubuntu 24.04, kernel 7.0.0-1018-aws, on an AWS c8i.4xlarge with nested virtualization (KVM, no host GPU)

**What happens**

A branchable source started with `SMOLVM_DISPLAY` and `SMOLVM_VNC` serves frames normally. A child
created with `machine branch` and its own `SMOLVM_VNC` port listens on that port and completes the RFB
handshake (`RFB 003.008`, desktop name `smolvm`), but never sends a FramebufferUpdate, so a VNC client
waits forever. This happened for 10 of 10 branches on Linux and 1 of 1 on macOS.

Inside the child, `/dev/dri/card0` is present and the compositor keeps running. None of these produce a
frame: moving the pointer over VNC, `swaymsg reload`, `swaymsg output * power off` then `power on`,
or killing and restarting sway.

**Expected**

The child serves its own display on its `SMOLVM_VNC` port, like the source, so each branch of a desktop
can be viewed and driven independently.

**Repro**

```sh
smolvm machine create --net --name desk --image debian:trixie-slim --cpus 2 --mem 2048 --overlay 8
smolvm machine start --name desk
smolvm machine exec --name desk -- sh -c 'apt-get update -qq && \
  DEBIAN_FRONTEND=noninteractive apt-get install -y -qq --no-install-recommends sway foot'
smolvm machine stop --name desk

SMOLVM_DISPLAY=1280x800 SMOLVM_VNC=5920 smolvm machine start --name desk --branchable
smolvm machine exec --name desk -d -- sh -c 'mkdir -p /run/xdg && chmod 700 /run/xdg && \
  XDG_RUNTIME_DIR=/run/xdg WLR_RENDERER=pixman LIBSEAT_BACKEND=noop WLR_LIBINPUT_NO_DEVICES=1 \
  sway -c /dev/null >/tmp/sway.log 2>&1'
sleep 5
vncdo -s 127.0.0.1::5920 capture source.png        # works

SMOLVM_DISPLAY=1280x800 SMOLVM_VNC=5931 smolvm machine branch --from desk --name child
sleep 3
vncdo -s 127.0.0.1::5931 capture child.png         # hangs: handshake only, no frame
```

(`vncdo` is from the `vncdotool` Python package. Note it can exit 0 without writing a file when the
connection drops, so check that `child.png` exists.)

**Related observation (may deserve its own issue)**

On the Linux host above, which has no GPU, `machine start` takes 5.8 s with `SMOLVM_DISPLAY` set and
0.54 s without it, on every start, even without `--gpu`. That matches the 5 second `/dev/dri` probe
described in docs/browser-display.md, which the docs describe for `--gpu` machines.

Filed upstream as https://github.com/smol-machines/smolvm/issues/2475 (2026-11-27).

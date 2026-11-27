# Open-source and self-hostable building blocks for a lightweight "fleet of agent computers" (state as of 2026-11-27)

Method note: GitHub stars, licenses, last-push dates and latest releases below were pulled from the GitHub REST API on 2026-11-27 (via `gh api repos/<owner>/<repo>`), so they are a point-in-time snapshot. README and doc claims were read from the repos' default branches on the same day. Vendor performance claims (boot times, etc.) are self-reported unless noted.

## Q1. Virtualization layers: host requirements, boot time, memory overhead, graphics, snapshot/fork, can it run a desktop?

### Takeaway
No microVM layer ships a virtual display device that is "desktop ready" out of the box except crosvm and libkrun (virtio-gpu); Firecracker, Cloud Hypervisor and QEMU `microvm` have no GPU/display device, so a desktop on them has to be a software framebuffer (Xvfb or a headless Wayland compositor) streamed over the network, which is exactly how E2B Desktop, Arrakis and Kernel do it. For a Linux fleet, Firecracker (snapshot + userfaultfd + CoW, now with PCIe, virtio-mem and pmem) and Cloud Hypervisor (snapshot with on-demand userfaultfd restore, Windows guests, VFIO) are the mature server choices; libkrun-based runtimes (microsandbox, smolvm, BoxLite) are the cross-platform (Linux/macOS/Windows) choice with fork/branch and GPU; macOS guests are capped at 2 concurrent VMs per Mac by Apple's license.

### Cited Findings

**Firecracker** (Apache-2.0, 38.3k stars, v1.18.0 released 2026-11-09, actively pushed 2026-11-27)
- Spec: for a 1 vCPU / 128 MiB guest, Firecracker VMM threads have memory overhead `<= 5 MiB` (workload and config dependent), and it takes `<= 125 ms` from the InstanceStart API call to the guest's `/sbin/init`; the API socket is available within `8 CPU ms`. [Firecracker SPECIFICATION.md](https://github.com/firecracker-microvm/firecracker/blob/main/SPECIFICATION.md)
- Design doc (via search summary): up to 150 microVM creations per second per host; concurrent microVMs limited only by hardware resources. [Firecracker design.md](https://raw.githubusercontent.com/firecracker-microvm/firecracker/main/docs/design.md)
- Host: Linux + KVM only (implied by the design; E2B notes "The runtime needs Linux with KVM"). [e2b-dev/runtime README](https://github.com/e2b-dev/runtime)
- 2025-2026 changelog additions relevant to desktops/density: PCIe transport and PCIe device hotplug (`pcie_hotplug_ports`), virtio-mem memory hotplug, pmem devices with `pmem_overrides` on snapshot load, balloon free page reporting/hinting, `drive_overrides` on `PUT /snapshot/load`, and a fix for creation/restore taking 20-50 ms longer on Linux 6.18+ hosts. No GPU/virtio-gpu entry appears in the changelog. [Firecracker CHANGELOG.md](https://github.com/firecracker-microvm/firecracker/blob/main/CHANGELOG.md)
- Snapshots: full and diff snapshots; diff snapshots are "still in developer preview"; docs include dedicated sections on network connectivity for clones, "snapshot security and uniqueness" and a VMGenID limitation (i.e., cloning a snapshot many times needs care for entropy/identity). [Firecracker snapshot-support.md](https://github.com/firecracker-microvm/firecracker/blob/main/docs/snapshotting/snapshot-support.md)
- Security advisory: versions v0.23.0 through v1.16.1, when restoring from a snapshot with the File memory backend plus a balloon or virtio-mem device, a failed memory discard can expose Firecracker process memory to the guest; workarounds include heuristic overcommit, keeping the OOM killer enabled for the Firecracker cgroup, or using the Uffd backend. Directly relevant to anyone combining snapshots with ballooning for density. [GHSA-48ww-rqpp-2mgm](https://github.com/firecracker-microvm/firecracker/security/advisories/GHSA-48ww-rqpp-2mgm)
- Graphics: secondary sources state Firecracker has no VGA, QXL, or virtio-gpu and no VNC/SPICE. [Justin Lin, Firecracker review](https://justinlinkk.github.io/blog/fire_cracker_review) (secondary; note that older claims "no PCI" are now outdated given PCIe support in the changelog above)

**Cloud Hypervisor** (6.6k stars, v55.0 released 2026-11-19; GitHub API reports no single SPDX license)
- Runs on KVM or Microsoft Hypervisor (MSHV); "low memory footprint"; CPU, memory, PCI hotplug; supports 64-bit Linux and Windows guests such as Windows Server 2025 and Windows 11; direct kernel boot or firmware boot. [Cloud Hypervisor README](https://github.com/cloud-hypervisor/cloud-hypervisor)
- Snapshot/restore: `memory_restore_mode=ondemand` uses userfaultfd to fault snapshot pages lazily; VFIO and vDPA devices are not supported with snapshot/restore; `prefault=on` not supported in that mode. [cloud-hypervisor docs/snapshot_restore.md](https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/snapshot_restore.md)
- The docs directory (2026-11-27) contains balloon, hotplug, live_migration, vfio, vfio-user, vhost-user, windows, landlock, ivshmem, etc., but no GPU/virtio-gpu document. [cloud-hypervisor docs/](https://github.com/cloud-hypervisor/cloud-hypervisor/tree/main/docs)
- Arrakis uses Cloud Hypervisor and states it "only works with `/dev/kvm`... we only support Linux machines." [abshkbh/arrakis README](https://github.com/abshkbh/arrakis)

**QEMU `microvm` machine type**
- "Inspired by Firecracker"; minimalist, no PCI nor ACPI, optimized for boot time and footprint; supports ISA bus, one optional ISA serial port and up to eight virtio-mmio devices; PCI-only devices cannot be used. [QEMU microvm docs](https://www.qemu.org/docs/master/system/i386/microvm.html) (fetched from the qemu-project GitLab source `docs/system/i386/microvm.rst`)
- A Proxmox integration of microvm lists: no VGA, QXL or virtio-gpu, no VNC or SPICE, serial console only, no PCI passthrough (GPUs), no live migration. [rcarmo/pve-microvm limitations](https://raw.githubusercontent.com/rcarmo/pve-microvm/main/docs/limitations.md)
- Full QEMU (q35) remains the general option for GPU, desktop virtualization and passthrough (secondary). [Northflank, Firecracker vs QEMU](https://northflank.com/blog/firecracker-vs-qemu)
- Kernel's hypeman notes "On Linux amd64, use QEMU's minimal microvm backend. It cannot use PCI devices or hotplug memory." [kernel/hypeman README](https://github.com/kernel/hypeman)

**libkrun** (Apache-2.0, 3.0k stars, v1.19.8 released 2026-11-06; repo moved to `libkrun/libkrun`)
- Library giving programs KVM-based (Linux) and Hypervisor.framework-based (macOS) process isolation; devices include virtio-gpu (venus and native-context); krunkit runs GPU-enabled (venus) lightweight VMs on macOS; muvm uses native-context GPU acceleration; SEV/SEV-SNP and TDX variants exist. [libkrun README](https://github.com/libkrun/libkrun)

**microsandbox** (built on libkrun; Apache-2.0, 9.2k stars, v0.8.3 released 2026-11-26; repo moved from zerocore-ai to `superradcompany/microsandbox`)
- Claims average boot under 100 ms; fork live sandboxes (`msb fork`), snapshot to file and restore (`msb snap create`); embeddable SDK, no daemon; Linux (KVM), macOS (Apple Silicon), Windows (WHP). A community showcase "msb-omarchy" runs an Omarchy desktop with graphics inside a microVM on Apple Silicon. [superradcompany/microsandbox README](https://github.com/superradcompany/microsandbox)

**smolvm** (libkrun VMM + libkrunfw kernel; Apache-2.0, 7.2k stars, v1.37.7 released 2026-11-26)
- Boots "in under a second" (table: "<200ms to agent ready"); `.smolmachine` packed executables boot "in under 200 ms"; memory elastic via virtio balloon, idle vCPUs sleep; defaults 4 vCPUs and half host RAM up to 8 GiB; branchable live CoW children, checkpoints capture RAM/CPU/disk, arm64 Mac checkpoints resume on arm64 Linux; hosts: macOS (Hypervisor.framework), Linux (KVM), Windows (WHP, no GPU acceleration on Windows); docs for GPU (Vulkan via virtio-gpu/Venus, CUDA API remoting), nested virtualization, Kubernetes RuntimeClass, and "Browser display: a machine's desktop in the browser, with keyboard and mouse". Vendor comparison table claims QEMU ~15-30 s, Firecracker <125 ms, Kata ~500 ms boot. [smol-machines/smolvm README](https://github.com/smol-machines/smolvm)

**BoxLite** (Apache-2.0, 2.7k stars, v0.15.0 released 2026-11-24)
- Daemonless embeddable micro-VM per box (KVM / Hypervisor.framework) plus seccomp/sandbox-exec; per-box QCOW2 CoW disk, clone, export/import; macOS Apple Silicon, Linux x86_64/ARM64, Windows via WSL2 with KVM; cloud deployment guides for AWS/GCP; used by AstrBot as a "per-session computer-use sandbox". [boxlite-ai/boxlite README](https://github.com/boxlite-ai/boxlite)

**crosvm** (BSD-3-Clause, 1.4k stars on the GitHub mirror, pushed 2026-11-26)
- Device list includes `gpu` (graphics adapter), `wayland` (guest uses host Wayland socket), `input`, `snd`, `balloon`, `pmem`; Wayland forwarding requires virtio-gpu cross-domain mode (Linux 5.16+) and the `sommelier` proxy compositor in the guest. [crosvm book devices/index.md](https://github.com/google/crosvm/blob/main/docs/book/src/devices/index.md); [crosvm book wayland.md](https://github.com/google/crosvm/blob/main/docs/book/src/devices/wayland.md)

**gVisor** (Apache-2.0, 20.1k stars, release-20261123.1)
- Not a VM; a user-space application kernel. GPU via `nvproxy` (`--nvproxy` flag) which proxies to the host NVIDIA driver; targets CUDA, Vulkan and NVENC/NVDEC; selected NVIDIA driver versions and GPUs only. [gVisor GPU docs](https://gvisor.dev/docs/user_guide/gpu/)
- Measured overhead in one nested-host benchmark: gVisor pod ~30.9 MiB idle host memory vs 5.3 MiB runc, and 0.95 s to Ready vs 0.84 s runc. [computingforgeeks, gVisor vs Kata vs runc](https://computingforgeeks.com/gvisor-vs-kata-containers-vs-runc/)
- Cua's new SDK uses gVisor for Linux sandboxes: `cua sb create ubuntu --name dev  # a gVisor container`. [trycua/cua README](https://github.com/trycua/cua)

**Kata Containers** (Apache-2.0, 9.2k stars, 4.4.0 released 2026-11-17)
- One nested-virtualization benchmark (Kata 4.3.1): idle Kata pod held ~179.9 MiB host memory (QEMU PSS 179-185 MiB + ~2 MiB virtiofsd + ~7 MiB shim) and took 4.76 s to Ready; author warns nesting inflates Kata numbers. [computingforgeeks](https://computingforgeeks.com/gvisor-vs-kata-containers-vs-runc/)
- Academic benchmark (4 vCPU/4 GB): Kata startup 1913 ms vs Docker 571.6 ms. [Edera paper, arXiv 2501.04580](https://arxiv.org/pdf/2501.04580)
- Vendor estimate: 150-300 ms boot depending on VMM. [Northflank, What are Kata Containers](https://northflank.com/blog/what-are-kata-containers) (conflicts with the measured numbers above; config and nesting dependent)
- firecracker-containerd's deprecation notice calls Kata "likely the closest open-source alternative, supports multiple hypervisors including Firecracker". [firecracker-containerd README](https://github.com/firecracker-microvm/firecracker-containerd)

**Unikraft** (GitHub license field NOASSERTION, 3.9k stars, RELEASE-0.21.0 on 2026-05-20)
- Kernel's open-source images run headful Chromium either in Docker or as a Unikraft unikernel; unikernel standby snapshots state and gives "extremely fast cold restarts (<20ms)"; the image "requires at least 8gb of memory"; openbox/tint2 take a few seconds to start initially. [kernel/kernel-images README](https://github.com/kernel/kernel-images)
- Unikraft blog: 64 concurrent headless Chromium instances at ~1.5 GB RAM each, restore from snapshot in 50 ms or less, "thousands" of instances per server (no exact figure); headless, no display server. Page metadata last_updated 2025-03-17. [Unikraft blog, millisecond headless browsers](https://unikraft.com/blog/millisecond-headless-browsers)

**Hyperlight** (Microsoft, Apache-2.0, 4.8k stars, v0.19.0 released 2026-11-04)
- "No kernel or OS in the VM"; guests are `no_std` Rust or C ELF binaries; micro VMs spin up in milliseconds, guest calls in microseconds; snapshot()/restore() of VM memory; explicitly not for "general-purpose virtualization" or "full-blown Linux guest workloads that need syscalls, networking, or filesystem access". Cannot run a desktop. [hyperlight-dev/hyperlight README](https://github.com/hyperlight-dev/hyperlight)

**Apple Virtualization.framework / Containerization / `container`** (`apple/container` Apache-2.0, 52.1k stars, release 1.7.1 on 2026-11-06; `apple/containerization` 9.0k stars)
- `container` runs a lightweight VM per container using the Containerization Swift package, Virtualization.framework and vmnet; designed for macOS 26 or later. [apple/container README](https://github.com/apple/container); [technical-overview.md](https://github.com/apple/container/blob/main/docs/technical-overview.md)
- Memory: Virtualization.framework implements only partial ballooning; pages freed inside the Linux guest are not relinquished to the host, so many memory-intensive containers may need restarts. Containers "require less memory than full VMs, with boot times that are comparable to containers running in a shared VM" (no number given). [technical-overview.md](https://github.com/apple/container/blob/main/docs/technical-overview.md)
- New "container machine" feature (persistent Linux environment with read-write home directory mount) explicitly "trades away much of the isolation" and is only for trusted images. [container-machine.md](https://github.com/apple/container/blob/main/docs/container-machine.md)
- macOS guests: Apple's license (Section 2B(iii)) allows two concurrent macOS VMs per Mac and Virtualization.framework refuses a third (error code 6); enforced via a kernel variable. [Eclectic Light, How Apple limits VMs](https://eclecticlight.co/2022/08/04/virtualisation-on-apple-silicon-macs-8-how-apple-limits-vms/); [Apple Developer Forums, 2-VM SLA limit](https://developer.apple.com/forums/thread/830383); a macOS 26.5 user reports the slot is not always released after shutdown [Apple Developer Forums thread 845455](https://developer.apple.com/forums/thread/845455?answerId=906368022)
- Lume (Cua) builds on Virtualization.framework for macOS and Linux VMs; an experimental "Metal capability shim" raises reported GPU family support inside macOS VMs while GPU commands still use Apple's paravirtualized graphics path. [trycua/cua libs/lume README](https://github.com/trycua/cua/tree/main/libs/lume); [metal-capability-shim README](https://github.com/trycua/cua/tree/main/libs/lume/metal-capability-shim)
- kernel/hypeman supports Virtualization.framework ("vz") on macOS 11+ alongside Cloud Hypervisor, Firecracker and QEMU on Linux. [kernel/hypeman README](https://github.com/kernel/hypeman)

**Hyper-V / Windows Sandbox / WSL2**
- Windows Sandbox: dynamic base image reuses the host's installed Windows (only mutable files are copied; ~30 MB compressed package, ~500 MB on disk once installed), "direct map" shares immutable OS binary pages with the host, memory is reclaimable; boots from a preserved snapshot of memory/CPU/device state. [Microsoft Learn, Windows Sandbox architecture](https://learn.microsoft.com/en-us/windows/security/application-security/application-isolation/windows-sandbox/windows-sandbox-architecture); older 2018 figures (25 MB / ~100 MB) in [Microsoft Tech Community, Windows Sandbox](https://techcommunity.microsoft.com/blog/windowsosplatform/windows-sandbox/301849) conflict with current docs
- A third-party test (via search summary) reported a sandbox configured with 4 GB consuming only 237 MB host memory with no apps open. [AnandTech feature focus](https://at-web1.www.anandtech.com/show/14388/windows-10-may-2019-update-feature-focus-light-themes-and-sandboxes/5) (2019, possibly outdated)
- Windows 11 24H2 adds a `wsb` CLI with `start`, `list`, `exec`, `connect`, `share`, `ip`, `stop`. [Microsoft Learn, Windows Sandbox CLI](https://learn.microsoft.com/en-us/windows/security/application-security/application-isolation/windows-sandbox/windows-sandbox-cli)
- WSL2/WSLg: all user and system distros for a Windows user run in the same WSL VM against a single Linux kernel; GUI comes from a per-distro "system distro" (Azure Linux 3.0) running the WSLg X server, Wayland server and PulseAudio. Not an isolation boundary between agents. [microsoft/wslg README](https://github.com/microsoft/wslg)
- App Sandbox (new, MIT, 1.1k stars, created 2026-03-16): VM app for Windows and macOS focused on computer use; Windows 11 or Ubuntu 26.04 guests, GPU paravirtualization (GPU-PV), snapshots, nested virtualization, local API, MCP server, Python SDK. [jamesstringer90/appsandbox](https://github.com/jamesstringer90/appsandbox)

### Inferences
- Summary matrix (my synthesis of the cited facts above):

| Layer | Host | Boot (claimed) | Per-VM overhead | Graphics | Snapshot / fork | Desktop viable? |
|---|---|---|---|---|---|---|
| Firecracker | Linux KVM | <=125 ms to init; ms-scale restore | <=5 MiB VMM | none | full + diff (preview), uffd lazy restore (E2B), fork via snapshot | Yes, Xvfb/VNC inside guest (E2B Desktop) |
| Cloud Hypervisor | Linux KVM / MSHV | not measured here | "low" | VFIO passthrough only | yes, uffd on-demand restore, live migration | Yes (Arrakis); also Windows guests |
| QEMU microvm | Linux KVM | not measured here | small | none (no PCI) | QEMU savevm | Only software framebuffer |
| QEMU q35 | Linux/macOS/Win | seconds | 100s of MiB (Kata data) | virtio-gpu, VGA, SPICE/VNC built in | savevm | Yes, standard (OSWorld, dockur/windows) |
| libkrun (microsandbox, smolvm, BoxLite) | Linux KVM, macOS HVF, Windows WHP | <100-200 ms | balloon-elastic | virtio-gpu venus / native context | fork, branch, portable checkpoints | Yes (showcases, smolvm browser display) |
| crosvm | Linux KVM | not measured here | small | virtio-gpu, Wayland cross-domain | limited (not researched) | Yes (ChromeOS Crostini model) |
| gVisor | Linux (no KVM needed) | ~1 s pod | ~31 MiB | nvproxy (NVIDIA only) | checkpoint/restore (not researched here) | Yes, software Xvfb in a container |
| Kata | Linux KVM | 0.15-4.8 s | ~180 MiB (QEMU) | VFIO | depends on VMM | Yes, but heavy |
| Unikraft | KVM / Unikraft Cloud | <20 ms restore | n/a | none | snapshot standby | Headful Chromium only (Kernel) |
| Hyperlight | KVM / MSHV / Hyper-V | ms | tiny | none | memory snapshot | No |
| Apple VZ / container | macOS (26+ for container) | sub-second (unquantified) | partial balloon | paravirt Metal for macOS guests | VM save/restore (not researched) | Yes; macOS guests capped at 2 |
| Windows Sandbox / Hyper-V | Windows Pro/Ent | seconds (snapshot boot) | low (direct map) | GPU-PV | none user-exposed | Yes (Windows desktop) |

- Because Xvfb, headless Wayland compositors (Selkies' Smithay compositor, Sway headless, weston headless) and VNC/WebRTC servers do not need a virtual display device, the absence of virtio-gpu in Firecracker/Cloud Hypervisor is not a blocker for computer-use desktops; it only blocks GPU-accelerated rendering and hardware video encode inside the guest, pushing encode cost onto guest vCPUs.
- Firecracker plus the E2B snapshot model is the most production-proven path for Linux; libkrun-based runtimes are the most portable path (same API on macOS laptops and Linux servers) and are the only microVM family here with virtio-gpu.
- The 2-concurrent-VM macOS license cap makes "fleets of macOS agent computers" a per-Mac-hardware problem, not a density problem.

### Gaps
- No primary-source boot-time numbers fetched for Cloud Hypervisor, crosvm, QEMU microvm, or Apple `container` (Apple gives no number).
- gVisor checkpoint/restore and crosvm snapshot capabilities not verified in this pass.
- No authoritative Windows Sandbox boot-time figure; Windows Sandbox's single-instance limitation status in 24H2 not verified.
- Unikraft license (GitHub reports NOASSERTION) and Cloud Hypervisor license (GitHub reports none) were not verified from LICENSE files in this pass.

## Q2. Existing open-source projects targeting agent sandboxes or computer use: layer covered, license, activity, limitations

### Takeaway
The 2025 crop of agent-desktop projects has consolidated sharply by November 2026: Daytona went closed-source (June 2026), Bytebot is archived, Cua shut down its cloud fleets (Oct 2026) and deprecated its computer-server/agent/Lumier packages, firecracker-containerd was deprecated (Nov 2026), Ignite is dead, and Arrakis is stale since June 2025. The healthy open pieces are E2B runtime (Firecracker control plane, Apache-2.0) plus E2B Desktop template, OpenSandbox (Firecracker "Fast Sandbox" on K8s, MCP), kubernetes-sigs/agent-sandbox (K8s CRDs and warm pools), libkrun-based runtimes (microsandbox, smolvm, BoxLite), Kernel's browser images and hypeman, and streaming projects (neko, Selkies, KasmVNC). None of them alone is a full "desktop fleet" product.

### Cited Findings

**VM lifecycle / control planes**
- **E2B runtime** (formerly `e2b-dev/infra`, now `e2b-dev/runtime`; Apache-2.0, 1.9k stars, release 2026.30 on 2026-09-10, pushed 2026-11-25): full E2B backend (control-plane API, per-node Firecracker orchestrator, in-VM agent, edge router, template builder). "A sandbox is a resumed snapshot": templates are pre-booted VMs in object storage, memory served lazily via userfaultfd, rootfs is a CoW overlay; pause diffs memory/disk to object storage, idle sandboxes auto-pause and wake on traffic; fork a live sandbox into up to 100 sandboxes per request. Self-host via "E2B Embed" (single Linux KVM host, docker compose), Terraform for GCP/AWS/Azure, and a Kubernetes manifest. [e2b-dev/runtime README](https://github.com/e2b-dev/runtime)
  - Architecture doc: placement scores nodes on CPU load and hugepage-pool load, with tunable overcommit ratio and `placement-memory-max-overcommit`; sandboxes can hotplug memory in 128 MiB slots up to `maxMemoryMB`; nodes use NBD, cgroups, hugepages and local template caches; references a web dashboard's build listings and killed-sandbox records. [e2b-dev/runtime docs/ARCHITECTURE.md](https://github.com/e2b-dev/runtime/blob/main/docs/ARCHITECTURE.md)
  - Conflict: a search-result summary stated E2B's MicroVM runtime is proprietary and an academic paper classes E2B as SaaS with no local deployment; the current repo README contradicts this (Apache-2.0, single-machine Embed). [E2B site](https://e2b.dev/) vs [e2b-dev/runtime README](https://github.com/e2b-dev/runtime). Treat the repo as authoritative for Nov 2026.
- **E2B Desktop** (Apache-2.0, 1.6k stars, pushed 2026-11-23): template + examples; SDK sources moved into the E2B monorepo; "There can be only one stream at a time". [e2b-dev/desktop README](https://github.com/e2b-dev/desktop). Related: open-computer-use (2.4k stars) and Surf (881 stars) agent demos. [e2b-dev/open-computer-use](https://github.com/e2b-dev/open-computer-use); [e2b-dev/surf](https://github.com/e2b-dev/surf)
- **OpenSandbox** (moved from alibaba to `opensandbox-group/OpenSandbox`; Apache-2.0, 18.0k stars, release-1.2.1 on 2026-11-13): unified sandbox API local (Docker) to Kubernetes; "Fast Sandbox runtime" with Firecracker creation 64 ms P50 serial / 125.4 ms P99 at 10 concurrent, pause/resume preserving memory and disk; SDKs in Python, Java/Kotlin, TS, C#, Go; CLI `osb`; MCP server (`opensandbox-mcp`) exposing creation, command exec, file ops; examples for Playwright, Chrome and Desktop. [OpenSandbox README](https://github.com/opensandbox-group/OpenSandbox)
  - A downstream PR (openwork, merged 2026-10-29) runs desktop computers on Fast Sandbox microVMs and notes a microVM pause "writes a full checkpoint of disk and memory, about three minutes for a desktop", raising pause timeouts from 30 s to 5 min. [different-ai/openwork PR #6726](https://github.com/different-ai/openwork/pull/6726)
- **kubernetes-sigs/agent-sandbox** (Apache-2.0, 4.7k stars, v1.0.12 on 2026-11-20): SIG Apps CRDs `Sandbox` (single stateful pod, stable identity, persistent storage, hibernation), `SandboxTemplate`, `SandboxClaim`, `SandboxWarmPool`; delegates isolation to gVisor or Kata via RuntimeClass; includes a Sandbox Router HTTP proxy; requires K8s 1.25+; roadmap mentions memory sharing across sandboxes. Orchestrator only: no desktop, no streaming. [kubernetes-sigs/agent-sandbox README](https://github.com/kubernetes-sigs/agent-sandbox)
- **openkruise/agents** (Apache-2.0, 322 stars): operator for agent sandbox lifecycle. [openkruise/agents](https://github.com/openkruise/agents)
- **Arrakis** (AGPL-3.0, 897 stars, last push 2025-06-02): self-hosted Cloud Hypervisor microVM sandboxes with Ubuntu, a VNC server and Chrome at boot, REST API, Python SDK, MCP server, automatic port forwarding to the VNC GUI, snapshot-and-restore for backtracking; Linux KVM only. Closest single-repo match to "microVM + desktop + MCP + snapshot", but inactive for ~18 months. [abshkbh/arrakis](https://github.com/abshkbh/arrakis)
- **Flintlock** (MPL-2.0, 1.5k stars, v0.16.0 on 2026-10-03): containerd-backed microVM lifecycle service; successor recommended by Ignite. [liquidmetal-dev/flintlock](https://github.com/liquidmetal-dev/flintlock)
- **Weave Ignite** (archived; last push 2023-12-07): "This repository is no longer maintained... take a look at Flintlock." [weaveworks/ignite](https://github.com/weaveworks/ignite)
- **firecracker-containerd** (archived): "As of November 10 2026, this project is no longer maintained"; alternatives listed: AWS Nitro Sandboxes (closed-source), Kata, apple/container, containerd/nerdbox (libkrun), Lima. firecracker-go-sdk is also archived. [firecracker-containerd README](https://github.com/firecracker-microvm/firecracker-containerd); [firecracker-go-sdk](https://github.com/firecracker-microvm/firecracker-go-sdk)
- **kernel/hypeman** (MIT, 383 stars, pushed 2026-11-25): multi-hypervisor VM runtime for OCI images (Cloud Hypervisor, Firecracker, QEMU, Virtualization.framework), standby/restore "in milliseconds", GPU passthrough (vGPU, VFIO), systemd/launchd service. [kernel/hypeman](https://github.com/kernel/hypeman)
- **KubeVirt** (Apache-2.0, 7.2k stars, v1.10.0 on 2026-11-18): full VMs on Kubernetes (QEMU/libvirt). [kubevirt/kubevirt](https://github.com/kubevirt/kubevirt)
- **Daytona**: repo archived; "As of June 2026, Daytona's core development has moved to a private codebase"; last public tag v0.190.0 remains forkable; docs had Computer use and VNC access pages; 71.3k stars. [daytonaio/daytona README](https://github.com/daytonaio/daytona)
- Other general platforms that can host agent desktops: Incus (system containers + VMs, Apache-2.0, 6.7k) [lxc/incus](https://github.com/lxc/incus); Coder ("Secure environments for developers and their agents", AGPL-3.0, 17.5k) [coder/coder](https://github.com/coder/coder); Ubicloud (AGPL-3.0, 12.4k) [ubicloud/ubicloud](https://github.com/ubicloud/ubicloud).

**Embeddable microVM runtimes (2025-2026 newcomers)**
- microsandbox, smolvm and BoxLite (details in Q1). [microsandbox](https://github.com/superradcompany/microsandbox); [smolvm](https://github.com/smol-machines/smolvm); [BoxLite](https://github.com/boxlite-ai/boxlite)
- An arXiv paper (SideKernel, macOS microVM sandbox for coding agents) lists SmolVM, Microsandbox, Shuru and VibeBox as local open-source microVM tools for terminal coding agents (via search summary). [arXiv 2610.02456](https://arxiv.org/pdf/2610.02456)

**Computer-use specific stacks**
- **Cua** (`trycua/cua` monorepo, MIT, 33.1k stars, very active): now positioned as Cua Spaces (macOS 26+ desktop app; Spaces are local macOS VMs or Linux/Omarchy images, multiplayer cursors, "teleport" signed-in apps), Cua Driver (desktop automation for macOS/Windows/Linux via CLI, MCP, SDKs), Lume (Virtualization.framework VMs), CUA-S1 models, Cua Bench; SDK in Python/TS/Rust; images with `cua-spacesd` (port 3211) provide processes, files, screenshots, input and low-latency video/audio streaming. [trycua/cua README](https://github.com/trycua/cua)
  - Cua Cloud including Fleets (pools, claims, warm replicas, Terraform provider) ended 2026-10-18; deprecated: `cuabot`, `cua-computer`, `cua-computer-server`, `cua-agent`, `cua-core`, `cua-bench-ui`, and the `cua-xfce`, `xfce-cua`, Kasm and `cua-qemu-*` images; sandboxes now run locally or in the user's own AWS/GCP/Modal. [Cua docs, Cua Cloud ended](https://cua.ai/docs/cua-sdk/guides/cua-cloud-ended)
  - Lumier (macOS/Linux VMs in a Docker container) is deprecated, image builds switched off. [libs/lumier README](https://github.com/trycua/cua/tree/main/libs/lumier)
- **ByteDance agent-infra/sandbox (AIO Sandbox)** (Apache-2.0, 6.4k stars, v1.11.0 on 2026-06-23, last push 2026-09-14): single Docker container combining browser (VNC + CDP), shell, files, MCP hub, VSCode server, Jupyter; run with `--security-opt seccomp=unconfined`; compose example limits memory to 2Gi. Container isolation only. [agent-infra/sandbox README](https://github.com/agent-infra/sandbox)
- **Bytebot** (Apache-2.0, 11.1k stars): archived (last push 2025-09-12). Desktop container runs Xvfb 1280x960x24, x11vnc, websockify, bytebotd; compose uses `privileged: true` and `shm_size: 2g`. [bytebot-ai/bytebot](https://github.com/bytebot-ai/bytebot); [supervisord.conf](https://github.com/bytebot-ai/bytebot/blob/main/packages/bytebotd/root/etc/supervisor/conf.d/supervisord.conf)
- **Anthropic computer-use-demo** (now in `anthropics/claude-quickstarts`, MIT, 21.4k stars): "deliberately minimal, containerized reference"; agent loop runs inside the controlled container, one session at a time. [computer-use-demo README](https://github.com/anthropics/claude-quickstarts/tree/main/computer-use-demo)
- **OpenAI CUA sample app** (MIT, 1.9k stars): now two agents, Playwright (browser) and PyAutoGUI (desktop via screenshots); no VM/fleet infrastructure. [openai/openai-cua-sample-app](https://github.com/openai/openai-cua-sample-app)
- **Kernel kernel-images** (Apache-2.0, 1.2k stars): sandboxed Chrome with CDP on 9222, live view via noVNC or WebRTC (neko), single-recording-at-a-time recording API on port 10001, Docker or Unikraft. Browser only, not a full desktop. [kernel/kernel-images](https://github.com/kernel/kernel-images)
- **Steel Browser** (Apache-2.0, 8.0k stars): browser sandbox API. [steel-dev/steel-browser](https://github.com/steel-dev/steel-browser)
- **OSWorld** (Apache-2.0, 3.3k stars): providers for VMware, VirtualBox, Docker (QEMU/KVM inside the `happysixd/osworld-docker` image, defaults `RAM_SIZE=4G`, `CPU_CORES=4`, `DISK_SIZE=32G`), AWS, Azure; OSWorld-Verified (2025-07-28) parallelizes on AWS to finish evaluation within about 1 hour. [xlang-ai/OSWorld README](https://github.com/xlang-ai/OSWorld); [docker provider.py](https://github.com/xlang-ai/OSWorld/blob/main/desktop_env/providers/docker/provider.py)
- **Agent-S** (Apache-2.0, 13.1k stars) is an agent framework, not infrastructure. [simular-ai/Agent-S](https://github.com/simular-ai/Agent-S). **Windows Agent Arena** (MIT, 920 stars). [microsoft/WindowsAgentArena](https://github.com/microsoft/WindowsAgentArena)
- **dockur/windows** (MIT, 54.3k stars): Windows inside a Docker container (QEMU/KVM). [dockur/windows](https://github.com/dockur/windows)
- **OpenHands** (repo now `OpenHands/OpenHands`, MIT, 94.4k stars). [OpenHands/OpenHands](https://github.com/OpenHands/OpenHands)
- **tart** now lives at `openai/tart` (8.0k stars, license field NOASSERTION): macOS and Linux VMs on Apple Silicon for CI/automation. [openai/tart](https://github.com/openai/tart)
- **App Sandbox** (MIT, new in 2026): VM app with local API + MCP + Python SDK for computer-use models on Windows/macOS hosts. [jamesstringer90/appsandbox](https://github.com/jamesstringer90/appsandbox)
- DeepSeek DSec (paper, Sep 2026): production sandbox platform with FnCall, container, microVM and full-VM backends; ~160 nodes per unit, ~3M sandboxes/day, 380k+ concurrent, 5,000+ creations/s; not stated as open source; no GUI mentioned. [arXiv 2609.22978](https://arxiv.org/abs/2609.22978)

### Inferences
- Layer coverage (synthesis): VM runtime = Firecracker, Cloud Hypervisor, libkrun family, hypeman, Flintlock; fleet control plane = E2B runtime, OpenSandbox, agent-sandbox (K8s), openkruise/agents; desktop image = E2B Desktop template, Anthropic demo, Bytebot (archived), linuxserver webtop, accetto, Cua images; API/SDK = E2B, OpenSandbox, microsandbox, smolvm, BoxLite, Cua SDK; MCP = OpenSandbox MCP, Arrakis MCP, AIO Sandbox MCP hub, Cua Driver MCP, App Sandbox MCP; live view = noVNC/VNC everywhere, WebRTC in neko/Kernel/Selkies/Cua spacesd; recording = Kernel (1 at a time), ffmpeg in E2B template (DIY); fleet dashboard with live multi-view = none found.
- Commercial gravity: Daytona (closed in June 2026) and Cua (cloud shut in Oct 2026, pivot to local "Spaces") show that open-core computer-use fleet hosting has been hard to sustain; the open pieces that survive are either infra-vendor runtimes (E2B, Kernel) or generic sandbox orchestration (OpenSandbox, agent-sandbox).
- E2B runtime is the most complete open Firecracker control plane (snapshot templates, pause/resume, fork, placement with hugepage-aware overcommit) and E2B Desktop proves the desktop-on-Firecracker pattern; but it is Linux/KVM-only, desktop streaming is one stream per sandbox, and it is not designed around a multi-desktop live dashboard.

### Gaps
- Kasm Workspaces (the orchestration product, not KasmVNC) license and self-host terms were not verified in this pass.
- OpenHands runtime internals (Docker vs remote runtime, VNC support) were not researched.
- E2B dashboard repo and whether it shows live desktop views were not verified.
- Engineering blogs from Fly.io, Modal, Cloudflare, Morph describing architectures were not fetched (time budget); Cloudflare sandbox-sdk exists (1.2k stars) but was not analyzed. [cloudflare/sandbox-sdk](https://github.com/cloudflare/sandbox-sdk)
- Why `cirruslabs/tart` is now under the `openai` org was not verified.

## Q3. Desktop images: smallest practical Linux desktop for computer use and its footprint

### Takeaway
The de facto computer-use desktop is Xvfb + a light WM (Xfce, mutter+tint2, openbox) + xdotool + x11vnc + noVNC/websockify + a browser, at 1024x768 to 1280x960; Wayland alternatives now exist (Selkies' Smithay compositor, Sway in linuxserver webtop 4.0). Published sizing is generous (E2B Desktop template 8 vCPU / 8 GB; Kernel headful Chromium unikernel needs 8 GB; OSWorld VM 4 GB / 4 cores; AIO sandbox 2 GiB limit), and no primary benchmark of a minimal Xvfb+WM+Chromium idle footprint was found.

### Cited Findings
- E2B Desktop template apt-installs `xvfb`, `xfce4`, `xfce4-goodies`, `xdotool`, `ffmpeg`, `x11vnc`, plus an E2B fork of noVNC and websockify; production build uses `cpu_count=8, memory_mb=8192`. [e2b-dev/desktop template/template.py](https://github.com/e2b-dev/desktop/blob/main/template/template.py); [template/build_prod.py](https://github.com/e2b-dev/desktop/blob/main/template/build_prod.py)
- Anthropic computer-use-demo: `ubuntu:22.04`, `xvfb`, `xdotool`, `mutter`, `x11vnc`, `tint2`, `firefox-esr`, noVNC v1.5.0 + websockify v0.12.0; default `WIDTH=1024`, `HEIGHT=768`. [computer-use-demo Dockerfile](https://github.com/anthropics/claude-quickstarts/blob/main/computer-use-demo/Dockerfile)
- Bytebot desktop: Xvfb `1280x960x24`, x11vnc, websockify, `shm_size: 2g`. [Bytebot supervisord.conf](https://github.com/bytebot-ai/bytebot/blob/main/packages/bytebotd/root/etc/supervisor/conf.d/supervisord.conf); [docker-compose.yml](https://github.com/bytebot-ai/bytebot/blob/main/docker/docker-compose.yml)
- Kernel headful Chromium: openbox + tint2, "requires at least 8gb of memory" for the unikernel deployment. [kernel/kernel-images](https://github.com/kernel/kernel-images)
- AIO Sandbox compose example memory limit `2Gi`. [agent-infra/sandbox README](https://github.com/agent-infra/sandbox)
- OSWorld Docker VM defaults: 4G RAM, 4 cores, 32G disk. [OSWorld docker provider.py](https://github.com/xlang-ai/OSWorld/blob/main/desktop_env/providers/docker/provider.py)
- linuxserver webtop 4.0 (2026-01-07): Wayland mode via a custom Smithay-based compositor replacing Xvfb; i3-tagged images run Sway, KDE runs plasma_wayland; wlroots desktops can attach to `wayland-1`; without GPU falls back to Pixman/LLVMpipe; legacy X11 stack needed a 16K virtual framebuffer, clamping to ~4K via `MAX_RES` improved performance. [linuxserver.io blog, Webtop 4.0](https://www.linuxserver.io/blog/webtop-4-0-wayland-is-here-engage-the-reality-engine); images: [linuxserver/docker-webtop](https://github.com/linuxserver/docker-webtop) (GPL-3.0, 4.5k stars)
- Other base images: accetto headless Ubuntu/Xfce with VNC/noVNC (MIT, 325 stars). [accetto/ubuntu-vnc-xfce-g3](https://github.com/accetto/ubuntu-vnc-xfce-g3)
- Cua's catalog images are `ghcr.io/trycua/linux`, `windows`, `macos`; the old `cua-xfce`/`xfce-cua`/Kasm images are deprecated. [Cua docs, Cua Cloud ended](https://cua.ai/docs/cua-sdk/guides/cua-cloud-ended)
- Chromium memory (secondary estimates, via search summary): roughly 50-150 MB cold start; 100-300 MB baseline plus 50-150 MB per page; up to 100-500 MB per instance; advice to size for peak, and to use `--disable-dev-shm-usage` because container `/dev/shm` defaults to 64 MB. [webscraping.ai FAQ](https://webscraping.ai/faq/headless-chromium/what-are-the-resource-requirements-for-running-headless-chromium-at-scale); [dev.to, headless Chromium at scale](https://dev.to/rendershot/headless-chromium-at-scale-four-fixes-for-a-fleet-that-kept-eating-ram-1mdp). Attribution of each figure to a specific page was not verified.
- Unikraft's trimmed headless Chromium image used ~1.5 GB per instance. [Unikraft blog](https://unikraft.com/blog/millisecond-headless-browsers)
- Restoring many sandboxes from one snapshot maps memory MAP_PRIVATE, so read-only pages are shared; one sample guest showed ~1.45 GiB Shared_Clean vs ~72 MiB Private_Dirty (vendor blog, Aug 2026). [PandaStack, KSM for microVMs](https://www.pandastack.ai/blog/firecracker-ksm-memory-deduplication-explained/)

### Inferences
- A practical minimal agent desktop is Xvfb (or a headless Wayland compositor) + openbox or mutter + tint2 + xdotool + x11vnc (or a WebRTC capturer) + Chromium/Firefox. Rough planning (my extrapolation, not measured): a few tens of MB for Xvfb+WM+VNC, with the browser dominating at 300 MB to 1.5 GB; vendors provision 2-8 GB per desktop, which is mostly headroom.
- The snapshot-shared-memory data point (1.45 GiB clean shared vs 72 MiB private dirty) suggests that when many desktops are restored from one warm template, the marginal RSS per idle desktop can be in the low hundreds of MB, until browsing dirties pages. This is the key lever for density, more than KSM.
- Resolution matters: 1024x768 or 1280x800 framebuffers keep both screenshot tokens and encoder CPU low; the linuxserver note about clamping the X11 framebuffer supports keeping virtual displays small.

### Gaps
- No primary measurement found of idle RAM/CPU for Xvfb + Xfce/openbox + Chromium inside a microVM, nor compressed image sizes of the E2B/Anthropic/Bytebot desktop images.
- No benchmark of Wayland-headless (Sway/weston/Smithay) vs Xvfb footprint.

## Q4. Screen streaming and input: VNC/noVNC, WebRTC (Selkies, neko), RDP/xrdp, SPICE; per-stream CPU; cheap 50-500 live thumbnails

### Takeaway
noVNC over x11vnc/websockify is the default in nearly every agent desktop (E2B, Anthropic, Bytebot, Arrakis, AIO, Kernel); WebRTC (neko, Selkies, Kernel's neko mode, Cua spacesd) is better for a human taking over. For many thumbnails, the cheapest open mechanisms are pull-based JPEG snapshots (neko screencast and screenshot API, KasmVNC web API, VNC framebuffer requests at low rates) rather than continuous video streams, and neko only runs its encoder while a client is connected. No published per-stream CPU benchmark for neko, Selkies or noVNC was found.

### Cited Findings
- neko (Apache-2.0, 22.7k stars, v3.2.0 on 2026-10-15): WebRTC desktop/browser streaming via GStreamer; codecs `vp8`, `vp9`, `av1`, `h264`, `h265`; multiple video pipelines (clients choose quality); "The Gstreamer pipeline is started when the first client requests the video stream and is stopped after the last client disconnects"; a WebRTC fallback "Screencast" captures the display as JPEG images over HTTP; RTMP broadcast; multi-user simultaneous control; neko-rooms for room API. [neko capture docs](https://github.com/m1k1o/neko/blob/master/webpage/docs/configuration/capture.md); [neko README](https://github.com/m1k1o/neko); screenshot API doc exists at [screen-shot-image.api.mdx](https://github.com/m1k1o/neko/blob/master/webpage/docs/api/screen-shot-image.api.mdx)
- Selkies (MPL-2.0, 2.7k stars, 2.2.1 on 2026-11-04): "at least 60 frames per second on Full HD", X11 and Wayland, GPU/CPU accelerated; `pip install selkies` bundles pixelflux and pcmflux encoders and web client. [selkies-project/selkies README](https://github.com/selkies-project/selkies)
- linuxserver webtop 4.0 (Selkies-based, 2026-01-07): zero-copy GPU encode (VAAPI on Intel/AMD, NVENC on NVIDIA); on an ~$80 Intel N97 mini PC with 16 GB RAM the author ran eight concurrent Firefox sessions playing 1080p YouTube, "eight separate 60 fps" sessions; no CPU utilization numbers published. [linuxserver.io blog](https://www.linuxserver.io/blog/webtop-4-0-wayland-is-here-engage-the-reality-engine)
- KasmVNC (GPL-2.0, 5.4k stars, v1.5.2 on 2026-10-24): web-native VNC with WebP/JPEG encoders, `max_frame_rate: 60` config, web API for remote control and information, multi-user with permissions, "network and CPU bottleneck statistics". [kasmtech/KasmVNC README](https://github.com/kasmtech/KasmVNC); [KasmVNC API wiki](https://github.com/kasmtech/KasmVNC/wiki/API)
- noVNC (14.3k stars) and TigerVNC (GPL-2.0, 7.8k) and xrdp (Apache-2.0, 6.8k) remain active (pushes in Oct-Nov 2026). [novnc/noVNC](https://github.com/novnc/noVNC); [TigerVNC](https://github.com/TigerVNC/tigervnc); [neutrinolabs/xrdp](https://github.com/neutrinolabs/xrdp)
- Kernel images offer noVNC or WebRTC (neko-based, "much faster than VNC", supports resize and copy/paste); WebRTC on Unikraft Cloud requires a TURN server because UDP exposure is unsupported. [kernel/kernel-images](https://github.com/kernel/kernel-images)
- E2B Desktop streams via noVNC, one stream at a time per sandbox, with auth keys. [e2b-dev/desktop README](https://github.com/e2b-dev/desktop)
- Cua images with `cua-spacesd` provide "low-latency video and audio streaming"; Spaces support multiplayer (human + agents each with own cursor). [trycua/cua README](https://github.com/trycua/cua)
- WSLg uses its own X/Wayland/PulseAudio servers in a system distro (RDP-based integration with Windows is the known design, not re-verified here). [microsoft/wslg](https://github.com/microsoft/wslg)
- General WebRTC codec data: H.264 typically lowest CPU when hardware accelerated; VP8 has little hardware acceleration; AV1 roughly 2x VP8 CPU at QVGA in one evaluation. [rtcbits, WebRTC codec performance](https://www.rtcbits.com/2021/02/webrtc-video-codecs-performance.html); [BlogGeek, which codec](https://bloggeek.me/webrtc-video-codec/)

### Inferences
- Thumbnail wall design for 50-500 desktops (synthesis): do not hold 500 WebRTC/VNC video sessions open. Instead (a) poll low-res JPEG frames at 0.5-2 fps per desktop from a screenshot endpoint (neko screencast/screenshot API, KasmVNC API, or `xwd`/`import`/ffmpeg x11grab inside the guest) and only (b) upgrade the focused tile to a full noVNC/WebRTC session. Because neko's GStreamer pipeline stops when no client is connected, idle desktops cost no encoder CPU. With VNC, unscaled framebuffer updates only send dirty rectangles, so mostly-static agent desktops are cheap to keep connected at low fps.
- Without GPUs (Firecracker/Cloud Hypervisor have none), all encoding is software on guest vCPUs; the N97 result suggests roughly 2 x 1080p60 sessions per low-end core with hardware encode, so software encode at full rate for hundreds of desktops would be CPU-prohibitive, reinforcing the snapshot-poll approach.
- Agent screenshots and human thumbnails can share one capture path (the agent loop already takes screenshots every step), which a fleet dashboard could reuse at zero extra guest cost.

### Gaps
- No primary per-stream CPU figures for x11vnc+noVNC, neko (x264/vp8) or Selkies (pixelflux) on CPU-only hosts.
- No found open-source project implementing a many-desktop thumbnail wall; SPICE and xrdp were not benchmarked.

## Q5. Density benchmarks: desktop microVMs per host (ballooning, KSM, overcommit)

### Takeaway
I found no published open benchmark of "N desktop microVMs per host". Closest data points: Firecracker's spec (<=5 MiB VMM overhead, 150 creations/s/host), OpenSandbox Fast Sandbox (64 ms P50 create), DSec (380k concurrent sandboxes on ~160 nodes, headless), Unikraft (64 headless Chromium at ~1.5 GB each), linuxserver webtop (8 x 1080p60 Firefox sessions on an N97 mini PC with GPU encode), and vendor notes that snapshot MAP_PRIVATE sharing beats KSM. Desktop density is therefore bounded by browser RAM and encode CPU, not VMM overhead.

### Cited Findings
- Firecracker VMM overhead <=5 MiB per microVM; creation at up to 150/s per host; concurrency limited by hardware. [Firecracker SPECIFICATION.md](https://github.com/firecracker-microvm/firecracker/blob/main/SPECIFICATION.md); [design.md](https://raw.githubusercontent.com/firecracker-microvm/firecracker/main/docs/design.md)
- An AWS Compute Blog post on scaling RL rollouts with Firecracker on EC2 metal reports (per search summary) that tail latency stays flat as density increases until about one microVM per vCPU; I could not fetch the article body to verify. [AWS Compute Blog](https://aws.amazon.com/blogs/compute/scaling-rl-rollouts-with-firecracker-on-amazon-ec2-metal/)
- E2B placement supports tunable memory overcommit and hugepage-pool scoring; memory hotplug in 128 MiB slots. [e2b-dev/runtime ARCHITECTURE.md](https://github.com/e2b-dev/runtime/blob/main/docs/ARCHITECTURE.md)
- OpenSandbox Fast Sandbox: "fast, high-density sandboxes on Kubernetes", 64 ms P50 / 125.4 ms P99 creation; desktop pause checkpoint ~3 minutes (full disk+memory). [OpenSandbox README](https://github.com/opensandbox-group/OpenSandbox); [openwork PR #6726](https://github.com/different-ai/openwork/pull/6726)
- DSec (DeepSeek): ~160 nodes, 380,000+ concurrent sandboxes, 5,000+ creations/s, "high-density overcommit" and memory sharing/reclamation; headless. [arXiv 2609.22978](https://arxiv.org/abs/2609.22978)
- Unikraft: 64 concurrent headless Chromium, ~1.5 GB each, "thousands" per server claimed. [Unikraft blog](https://unikraft.com/blog/millisecond-headless-browsers)
- linuxserver webtop: 8 concurrent 1080p60 Firefox sessions on Intel N97 (16 GB). [linuxserver.io blog](https://www.linuxserver.io/blog/webtop-4-0-wayland-is-here-engage-the-reality-engine)
- KSM vs snapshot sharing (vendor blog, 2026-08-23): MAP_PRIVATE snapshot restore gives structural page sharing with zero ongoing CPU and no cross-tenant merging; KSM default scan ~5,000 pages/s (~20 MiB/s), creates a CoW timing side channel (co-residency detection, fingerprinting), and some hypervisor vendors restricted cross-guest sharing; reported restore p50 179 ms / p99 ~203 ms, cold template boot ~3 s, forks 400-750 ms same host and 1.2-3.5 s cross-host. [PandaStack, KSM trap](https://www.pandastack.ai/blog/firecracker-ksm-memory-deduplication-explained/). Another vendor (AerolVM) avoids KSM by assuming one tenant per host. [AerolVM, Firecracker hydration](https://microvm.aerol.ai/firecracker-hydration/)
- Firecracker host OOM kills the whole VMM process; cap each Firecracker process with a cgroup memory limit (vendor guidance). [PandaStack, Firecracker OOM](https://www.pandastack.ai/blog/firecracker-oom-killer-guest-memory/)
- Ballooning + snapshot restore had a memory-disclosure advisory through v1.16.1. [GHSA-48ww-rqpp-2mgm](https://github.com/firecracker-microvm/firecracker/security/advisories/GHSA-48ww-rqpp-2mgm)
- Apple VZ guests do not return freed memory to the host (partial ballooning), limiting overcommit on Mac hosts. [apple/container technical-overview.md](https://github.com/apple/container/blob/main/docs/technical-overview.md)
- smolvm: virtio balloon elasticity and sleeping idle vCPUs mean "over-provisioning costs almost nothing" (vendor claim). [smolvm README](https://github.com/smol-machines/smolvm)
- Windows Sandbox: a 4 GB-configured sandbox measured at 237 MB host usage idle (third party, 2019). [AnandTech](https://at-web1.www.anandtech.com/show/14388/windows-10-may-2019-update-feature-focus-light-themes-and-sandboxes/5)

### Inferences
- Back-of-envelope (mine, not measured): on a 512 GB / 128-vCPU host, with desktops restored from a shared warm snapshot and ~0.5-1.5 GB private memory each once a browser is active, memory supports roughly 300-1,000 concurrent desktops; CPU at ~1 vCPU per actively-working desktop (screenshot + browser render + occasional encode) caps active desktops nearer 100-250, with many more possible if most are paused (E2B/OpenSandbox-style pause to storage) or idle. This is consistent with the "flat until ~1 microVM per vCPU" observation if accurate.
- DSec's average (~2,400 concurrent sandboxes per node) shows what headless density looks like at the extreme; GUI desktops should be expected at 1-2 orders of magnitude lower.
- Desktop snapshots are large and slow to pause (~3 minutes in OpenSandbox's case), so warm pools plus diff snapshots and lazy (uffd) restore matter more for desktops than for headless code sandboxes.

### Gaps
- No primary benchmark of desktop (Xvfb/Wayland + browser) microVM density on Firecracker, Cloud Hypervisor or libkrun found.
- AWS RL rollout article content unverified.
- No measured KSM savings for desktop guests.

## Q6. What is missing: is there one open project doing lightweight VM lifecycle + desktop + MCP + fleet dashboard with live view and recording?

### Takeaway
No. As of November 2026 no single open-source, self-hostable project combines (1) lightweight microVM lifecycle with snapshot/fork, (2) a maintained desktop image, (3) an MCP/SDK control surface, and (4) a fleet dashboard with many live views and session recording. The closest partial matches are E2B runtime + E2B Desktop (1, 2, SDK; single stream, no fleet live wall), OpenSandbox (1, MCP, desktop example; desktop pause is slow), Arrakis (1, 2, MCP, snapshot; stale since mid-2025, AGPL, no dashboard), Kernel images + hypeman (browser-only live view and single recording), and Cua (macOS-first local Spaces; its cloud fleet product ended in Oct 2026).

### Cited Findings
- E2B runtime covers control plane, Firecracker orchestration, snapshots, pause/resume, fork, single-host Embed and Terraform/K8s; Linux KVM only. [e2b-dev/runtime](https://github.com/e2b-dev/runtime). E2B Desktop: one stream at a time. [e2b-dev/desktop](https://github.com/e2b-dev/desktop)
- OpenSandbox: Docker to K8s lifecycle, Firecracker fast path, MCP for create/exec/files, desktop examples. [OpenSandbox](https://github.com/opensandbox-group/OpenSandbox)
- Arrakis: Cloud Hypervisor, VNC + Chrome, REST/Python/MCP, snapshot-restore; last push 2025-06-02. [abshkbh/arrakis](https://github.com/abshkbh/arrakis)
- agent-sandbox (K8s): warm pools and claims, but delegates isolation and has no desktop/streaming. [kubernetes-sigs/agent-sandbox](https://github.com/kubernetes-sigs/agent-sandbox)
- Kernel kernel-images: live view (noVNC/WebRTC) + recording API limited to one recording at a time; Chrome only. [kernel/kernel-images](https://github.com/kernel/kernel-images)
- Cua: Fleets/pools/warm replicas ended 2026-10-18 with "no replacement"; computer-server and agent packages deprecated. [Cua docs](https://cua.ai/docs/cua-sdk/guides/cua-cloud-ended)
- Daytona (which had computer use + VNC) moved to a private codebase in June 2026. [daytonaio/daytona](https://github.com/daytonaio/daytona)
- Bytebot (desktop agent + UI) archived. [bytebot-ai/bytebot](https://github.com/bytebot-ai/bytebot)
- Anthropic's reference demo is explicitly single-session with weak separation. [computer-use-demo README](https://github.com/anthropics/claude-quickstarts/tree/main/computer-use-demo)
- libkrun-family runtimes (microsandbox, smolvm, BoxLite) provide fork/branch and cross-OS hosts but are libraries/CLIs without fleet dashboards; smolvm documents a single-machine browser display. [microsandbox](https://github.com/superradcompany/microsandbox); [smolvm](https://github.com/smol-machines/smolvm); [BoxLite](https://github.com/boxlite-ai/boxlite)

### Inferences
- Concrete missing pieces for an open "fleet of agent computers" (synthesis):
  1. A desktop-aware snapshot pipeline: warm desktop templates (Xvfb/Wayland + browser already running) restored via uffd + CoW with diff snapshots, so create is sub-second and pause is not minutes. E2B has the mechanics; nobody ships it tuned for desktops in the open.
  2. A fleet live-view service: thumbnail wall for 50-500 desktops via low-fps JPEG polling, click-to-upgrade to WebRTC/noVNC, multi-viewer takeover (neko-style). Building blocks exist (neko screencast/screenshot API, KasmVNC API, noVNC) but no project assembles them for many VMs.
  3. Fleet-wide recording and replay: per-session video (ffmpeg x11grab or encoder tee) plus action/screenshot trajectory logs tied to agent steps; Kernel does one recording per browser; Cua Bench exports trajectories but for evaluation, not ops.
  4. A single MCP + SDK surface over lifecycle, input (xdotool/cua-driver), screenshots, files and snapshots/fork. Pieces exist (OpenSandbox MCP is text/file-centric; Cua Driver MCP is desktop-input-centric; Arrakis MCP is stale).
  5. Cross-OS guests in one fleet: Linux on Firecracker/CH/libkrun, Windows via Cloud Hypervisor or QEMU (dockur/windows, Windows Agent Arena), macOS only on Apple hardware with a 2-VM-per-Mac cap.
- The ecosystem trend (Daytona closing, Cua Cloud ending, firecracker-containerd deprecation pointing to closed AWS Nitro Sandboxes) leaves an opening for an Apache/MIT, self-hostable, desktop-first fleet layer that composes E2B-style Firecracker snapshots or a libkrun runtime, a slim Xvfb/Wayland image, neko/KasmVNC streaming, and an MCP server.

### Gaps
- Could not verify whether the E2B dashboard or OpenSandbox ship any multi-sandbox live view UI.
- Did not check every 2026 newcomer (e.g., Shuru, VibeBox, nerdbox) in depth; the list of "newcomers" is not exhaustive.

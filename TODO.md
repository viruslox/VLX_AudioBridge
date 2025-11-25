# VLX_AudioBridge Roadmap & Tasks

**Current Status:** 🟡 **Implementation / Integration Phase**
The project architecture has been fully redesigned. Core modules have been drafted and are ready for compilation. Legacy "Ermete" code has been deprecated.

## 🗺️ Roadmap Overview

| Phase | Description | Status |
| :--- | :--- | :--- |
| **Phase 1** | **Re-Architecture & Cleanup** | ✅ Completed |
| **Phase 2** | **Core Module Development** | ✅ Completed (Drafted) |
| **Phase 3** | **Integration & Build** | 🟡 In Progress |
| **Phase 4** | **Testing & Tuning** | 🔴 Pending |
| **Phase 5** | **Deployment (Systemd)** | 🔴 Pending |

---

## Detailed Task List

### 1. Architecture & Setup
- [x] Define new directory structure (`internal/`, `cmd/`, `configs/`).
- [x] Initialize new Go module (`github.com/viruslox/VLX_AudioBridge`).
- [x] Design `AudioBridge.yaml` configuration schema.
- [x] Deprecate and remove legacy `Ermete` files and scripts.

### 2. Core Modules Implementation
- [x] **Config:** Implement YAML parser (`internal/config`).
- [x] **System:** Implement Pipewire check & Virtual Sink creation (`internal/system`).
- [x] **Stream (Egress):**
    - [x] Create FFmpeg process wrapper for SRT streaming.
    - [x] Implement PCM Audio Mixer (Summation logic + Soft Clipping).
    - [x] Implement Opus Decoding & Packet Handling.
- [x] **Overlay (Ingress):**
    - [x] Implement Headless Chromium manager.
    - [x] Implement Audio Capture from Pipewire Monitor via PortAudio.
- [x] **Bot Logic:**
    - [x] Implement `vlx.join`, `vlx.leave`, `vlx.shutdown` commands.
    - [x] Coordinate Stream Manager and Overlay Capture.
    - [x] Implement SSRC mapping for user exclusion.

### 3. Integration & Build (Immediate Next Steps)
- [ ] **Dependency Check:** Ensure `libopus-dev`, `portaudio19-dev`, `ffmpeg`, `chromium-browser` are installed on target server.
- [ ] **Code Integration:** Assemble all drafted `.go` files into the project structure.
- [ ] **Compilation:** Successfully run `go build -o vlx_bridge main.go` without errors.
- [ ] **Linting:** Run `go vet ./...` to check for potential issues.

### 4. Testing & QA
- [ ] **Pipewire Validation:** Verify `VLX_VirtualSink` creation via `pactl list sinks`.
- [ ] **Overlay Test:** Verify Chromium instances launch and audio is routed to Discord.
- [ ] **SRT Stream Test:**
    - [ ] Verify connection to MediaMTX (or destination server).
    - [ ] Check A/V sync and latency.
    - [ ] **Mixing Quality:** Validate audio quality when multiple users speak (check for distortion).
- [ ] **Exclusion Logic:** Verify that configured `excluded_users` are NOT audible in the SRT stream.

### 5. Deployment
- [ ] **Service Configuration:** Install `scripts/ermete.service` to `~/.config/systemd/user/`.
- [ ] **Autostart:** Enable systemd user service (`systemctl --user enable vlx_bridge`).
- [ ] **Documentation:** Finalize `README.md` with usage instructions.

## Known Issues / To Watch
- **Audio Mixing:** Hard clipping protection is implemented, but high-volume concurrent speakers might still cause artifacts. Consider implementing a dynamic limiter if testing reveals issues.
- **Headless Chrome:** Ensure target server has necessary fonts/libraries for Chromium to render overlays correctly without a display.

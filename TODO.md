# VLX_AudioBridge Roadmap & Tasks

**Current Status:** **Testing & Tuning Phase**
Core streaming and mixing logic is stable. SRT latency issues resolved. Soundboard support deprecated for stability.

## Roadmap Overview

| Phase | Description | Status |
| :--- | :--- | :--- |
| **Phase 1** | **Re-Architecture & Cleanup** | ✅ Completed |
| **Phase 2** | **Core Module Development** | ✅ Completed |
| **Phase 3** | **Integration & Build** | ✅ Completed |
| **Phase 4** | **Testing & Tuning** | 🟡 In Progress |
| **Phase 5** | **Deployment (Systemd)** | 🔴 Pending |

---

## Task List

### Completed
- [x] Architecture & Config Schema (`AudioBridge.yaml`).
- [x] Core Modules (System, Config, Overlay, Bot).
- [x] **Stream Mixer:** Fixed race conditions and latency accumulation.
- [x] **SRT output:** Stabilized with `pkt_size=1316` and removed `-re` flag.
- [x] **Overlay:** Headless Chromium with PulseAudio sink injection.

### Next Steps (Immediate)
- [ ] **Soft Clipper:** Implement `tanh` based soft-clipping in `mixer.go` to replace hard clipping distortion.
- [ ] **Deployment:** Verify systemd user service operation on target machine.
- [ ] **Code Cleanup:** Ensure all comments are in concise technical English.

### Known Limitations
- **Discord Soundboard:** Not supported due to variable packet sizes causing mixer instability.

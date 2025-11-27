# VLX_AudioBridge Roadmap & Tasks

**Current Status:** 
***Version 2.0 Ready***
Audio mixing uses soft-clipping. SRT latency is minimized.

### Completed Features
- [x] **Config:** YAML schema (`AudioBridge.yaml`) implemented.
- [x] **System:** Pipewire/PulseAudio virtual sink automation.
- [x] **Stream Mixer:** - [x] Fixed race conditions and latency accumulation.
    - [x] Implemented `tanh` Soft Clipper for high-quality mixing.
- [x] **SRT Output:** Optimized with `pkt_size=1316` and removed `-re` flag.
- [x] **Overlay:** Headless Chromium manager with audio routing.
- [x] **Bot Logic:** Discord connection handling and owner-only commands.
- [x] **Deployment:** Systemd user service configured.

### Known Limitations
- **Discord Soundboard:** Not supported due to variable packet sizes causing mixer instability. Support is deprecated to prioritize voice latency.

package stream

import (
	"encoding/binary"
	"math"
	"sync"
	"time"
)

const (
	SampleRate   = 48000
	Channels     = 2
	FrameSize    = 960 // 20ms at 48kHz
	MaxBufferLen = 50  // Jitter buffer size
)

type Mixer struct {
	userBuffers map[uint32][][]int16
	mutex       sync.Mutex
	mixedOut    chan []byte
}

func NewMixer() *Mixer {
	return &Mixer{
		userBuffers: make(map[uint32][][]int16),
		// Low latency optimization: 10 packets buffer (approx. 200ms) to ensure responsiveness
		mixedOut: make(chan []byte, 10),
	}
}

// AddFrame queues incoming PCM packets.
// Standard Discord packets are 20ms. 
// Note: Larger packets from Soundboard are handled but truncated to 20ms to maintain real-time sync.
func (m *Mixer) AddFrame(ssrc uint32, data []int16) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.userBuffers[ssrc]; !exists {
		m.userBuffers[ssrc] = make([][]int16, 0, MaxBufferLen)
	}

	// Ring buffer logic: drop oldest frame if buffer is full
	if len(m.userBuffers[ssrc]) >= MaxBufferLen {
		m.userBuffers[ssrc] = m.userBuffers[ssrc][1:]
	}

	// Data copy to prevent memory race conditions
	frameCopy := make([]int16, len(data))
	copy(frameCopy, data)
	m.userBuffers[ssrc] = append(m.userBuffers[ssrc], frameCopy)
}

// StartMixing initiates the 20ms ticking loop for audio processing.
func (m *Mixer) StartMixing(stopChan <-chan struct{}) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	// Reusable buffer for mathematical summing operations
	outputFrame := make([]int16, FrameSize*Channels)

	for {
		select {
		case <-stopChan:
			return
		case <-ticker.C:
			m.mixTick(outputFrame)
		}
	}
}

func (m *Mixer) mixTick(out []int16) {
	m.mutex.Lock()

	// 1. Reset output frame to silence
	for i := range out {
		out[i] = 0
	}

	// 2. Mix samples from all active users
	for ssrc, frames := range m.userBuffers {
		if len(frames) > 0 {
			currentFrame := frames[0]
			
			for i := 0; i < len(out) && i < len(currentFrame); i++ {
				// Summing samples
				sum := int32(out[i]) + int32(currentFrame[i])
				
				// Soft Clipping (Tanh)
				// Replaces hard clipping to prevent digital distortion on volume spikes.
				// Formula: output = 32768 * tanh(sum / 32768)
				val := 32768.0 * math.Tanh(float64(sum)/32768.0)

				// Final clamp to int16 range to ensure safety
				if val > 32767 {
					val = 32767
				} else if val < -32768 {
					val = -32768
				}
				out[i] = int16(val)
			}
			// Dequeue processed frame
			m.userBuffers[ssrc] = m.userBuffers[ssrc][1:]
		}
	}
	m.mutex.Unlock()

	// 3. Serialize to Little Endian
	// Critical: Allocate new slice for channel transmission to avoid race conditions with FFmpeg
	outBytes := make([]byte, len(out)*2)
	for i, sample := range out {
		binary.LittleEndian.PutUint16(outBytes[i*2:], uint16(sample))
	}

	// 4. Non-blocking send to output channel
	select {
	case m.mixedOut <- outBytes:
	default:
		// Drop frame if consumer (FFmpeg) is lagging to avoid latency accumulation
	}
}

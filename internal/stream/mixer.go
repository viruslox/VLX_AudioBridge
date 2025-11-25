package stream

import (
	"encoding/binary"
	"sync"
	"time"
)

const (
	SampleRate   = 48000
	Channels     = 2
	FrameSize    = 960 // 20ms at 48kHz
	MaxBufferLen = 50  // Frame buffer size to mitigate jitter
)

// Mixer manages audio buffers from multiple users and combines them into a single stream.
type Mixer struct {
	userBuffers map[uint32][][]int16 // Map SSRC -> PCM Frame Queue
	mutex       sync.Mutex
	mixedOut    chan []byte // Output channel for mixed audio ready for FFmpeg
}

func NewMixer() *Mixer {
	return &Mixer{
		userBuffers: make(map[uint32][][]int16),
		mixedOut:    make(chan []byte, 100),
	}
}

// AddFrame appends a decoded PCM frame to the specific user's buffer.
func (m *Mixer) AddFrame(ssrc uint32, data []int16) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.userBuffers[ssrc]; !exists {
		m.userBuffers[ssrc] = make([][]int16, 0, MaxBufferLen)
	}

	// Simple buffer management: drop oldest frame if full to prevent infinite latency accumulation.
	if len(m.userBuffers[ssrc]) >= MaxBufferLen {
		m.userBuffers[ssrc] = m.userBuffers[ssrc][1:]
	}

	// Copy data to prevent race conditions or reference issues.
	frameCopy := make([]int16, len(data))
	copy(frameCopy, data)
	m.userBuffers[ssrc] = append(m.userBuffers[ssrc], frameCopy)
}

// StartMixing initializes the 20ms ticker loop to generate the final mixed audio.
func (m *Mixer) StartMixing(stopChan <-chan struct{}) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	outputFrame := make([]int16, FrameSize*Channels)
	byteBuffer := make([]byte, FrameSize*Channels*2) // 2 bytes per int16 sample

	for {
		select {
		case <-stopChan:
			return
		case <-ticker.C:
			m.mixTick(outputFrame, byteBuffer)
		}
	}
}

func (m *Mixer) mixTick(out []int16, outBytes []byte) {
	m.mutex.Lock()

	// Reset output frame to silence
	for i := range out {
		out[i] = 0
	}

	// Sum samples from all users with available data
	for ssrc, frames := range m.userBuffers {
		if len(frames) > 0 {
			currentFrame := frames[0]

			// Mixing: Summation with Hard Clipping
			for i := 0; i < len(out) && i < len(currentFrame); i++ {
				sum := int32(out[i]) + int32(currentFrame[i])
				
				// Clamp values to int16 range to prevent digital overflow distortion
				if sum > 32767 {
					sum = 32767
				} else if sum < -32768 {
					sum = -32768
				}
				out[i] = int16(sum)
			}

			// Remove processed frame from queue
			m.userBuffers[ssrc] = m.userBuffers[ssrc][1:]
		}
	}
	m.mutex.Unlock()

	// Even if silence, send data to keep FFmpeg stream active and synchronized.
	// Convert []int16 to []byte (Little Endian)
	for i, sample := range out {
		binary.LittleEndian.PutUint16(outBytes[i*2:], uint16(sample))
	}

	// Non-blocking send to output channel.
	// If buffer is full, drop the frame to avoid blocking the critical mixing loop.
	select {
	case m.mixedOut <- outBytes:
	default:
	}
}

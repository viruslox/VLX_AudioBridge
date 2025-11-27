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

type Mixer struct {
	userBuffers map[uint32][][]int16
	mutex       sync.Mutex
	mixedOut    chan []byte
}

func NewMixer() *Mixer {
	return &Mixer{
		userBuffers: make(map[uint32][][]int16),
		// OTTIMIZZAZIONE LATENZA:
		// Ridotto da 100 a 10. Un buffer di 100 pacchetti = 2 secondi di ritardo potenziale.
		// 10 pacchetti = 200ms di buffer, molto più reattivo.
		mixedOut: make(chan []byte, 10),
	}
}

// AddFrame standard: accetta pacchetti da 20ms (standard Discord Voice).
// I pacchetti Soundboard (più grandi) verranno processati solo per i primi 20ms, 
// ma questo garantisce zero lag per la voce.
func (m *Mixer) AddFrame(ssrc uint32, data []int16) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.userBuffers[ssrc]; !exists {
		m.userBuffers[ssrc] = make([][]int16, 0, MaxBufferLen)
	}

	if len(m.userBuffers[ssrc]) >= MaxBufferLen {
		m.userBuffers[ssrc] = m.userBuffers[ssrc][1:]
	}

	// Copia i dati per stabilità (Race Condition Fix)
	frameCopy := make([]int16, len(data))
	copy(frameCopy, data)
	m.userBuffers[ssrc] = append(m.userBuffers[ssrc], frameCopy)
}

func (m *Mixer) StartMixing(stopChan <-chan struct{}) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

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

	// 1. Reset silence
	for i := range out {
		out[i] = 0
	}

	// 2. Mixing
	for ssrc, frames := range m.userBuffers {
		if len(frames) > 0 {
			currentFrame := frames[0]
			// Mix sicuro
			for i := 0; i < len(out) && i < len(currentFrame); i++ {
				sum := int32(out[i]) + int32(currentFrame[i])
				if sum > 32767 {
					sum = 32767
				} else if sum < -32768 {
					sum = -32768
				}
				out[i] = int16(sum)
			}
			m.userBuffers[ssrc] = m.userBuffers[ssrc][1:]
		}
	}
	m.mutex.Unlock()

	// 3. Allocazione Nuova Slice (Race Condition Fix)
	outBytes := make([]byte, len(out)*2)
	for i, sample := range out {
		binary.LittleEndian.PutUint16(outBytes[i*2:], uint16(sample))
	}

	// 4. Send non-bloccante
	select {
	case m.mixedOut <- outBytes:
	default:
		// Se il canale è pieno, droppa il pacchetto per non accumulare ritardo.
	}
}

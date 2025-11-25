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
	FrameSize    = 960 // 20ms a 48kHz
	MaxBufferLen = 50  // Numero di frame da tenere in buffer per jitter
)

// Mixer gestisce i buffer di più utenti e li somma
type Mixer struct {
	userBuffers map[uint32][][]int16 // Mappa SSRC -> Queue di Frame PCM
	mutex       sync.Mutex
	mixedOut    chan []byte // Canale dove esce l'audio mixato pronto per FFmpeg
}

func NewMixer() *Mixer {
	return &Mixer{
		userBuffers: make(map[uint32][][]int16),
		mixedOut:    make(chan []byte, 100),
	}
}

// AddFrame aggiunge un frame audio decodificato al buffer di un utente
func (m *Mixer) AddFrame(ssrc uint32, data []int16) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.userBuffers[ssrc]; !exists {
		m.userBuffers[ssrc] = make([][]int16, 0, MaxBufferLen)
	}

	// Semplice gestione buffer: se troppo pieno, droppa il vecchio (evita latenza infinita)
	if len(m.userBuffers[ssrc]) >= MaxBufferLen {
		m.userBuffers[ssrc] = m.userBuffers[ssrc][1:]
	}

	// Copia i dati per sicurezza
	frameCopy := make([]int16, len(data))
	copy(frameCopy, data)
	m.userBuffers[ssrc] = append(m.userBuffers[ssrc], frameCopy)
}

// StartMixing avvia il ticker a 20ms che produce l'audio finale
func (m *Mixer) StartMixing(stopChan <-chan struct{}) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	outputFrame := make([]int16, FrameSize*Channels)
	byteBuffer := make([]byte, FrameSize*Channels*2) // 2 bytes per int16

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

	// Reset output frame a silenzio
	for i := range out {
		out[i] = 0
	}

	activeUsers := 0

	// Somma i sample di tutti gli utenti che hanno dati pronti
	for ssrc, frames := range m.userBuffers {
		if len(frames) > 0 {
			activeUsers++
			currentFrame := frames[0]

			// Mixing (Somma con clamping)
			for i := 0; i < len(out) && i < len(currentFrame); i++ {
				sum := int32(out[i]) + int32(currentFrame[i])
				// Hard Clipping per evitare overflow (distorsione digitale)
				if sum > 32767 {
					sum = 32767
				} else if sum < -32768 {
					sum = -32768
				}
				out[i] = int16(sum)
			}

			// Rimuovi il frame usato
			m.userBuffers[ssrc] = m.userBuffers[ssrc][1:]
		} else {
			// Pulizia utenti inattivi per risparmiare memoria (opzionale)
			// delete(m.userBuffers, ssrc)
		}
	}
	m.mutex.Unlock()

	// Se nessun utente parla, inviamo comunque silenzio per mantenere lo stream FFmpeg attivo/sincronizzato
	// Convertiamo []int16 in []byte (Little Endian)
	for i, sample := range out {
		binary.LittleEndian.PutUint16(outBytes[i*2:], uint16(sample))
	}

	// Invio non bloccante al canale di uscita
	select {
	case m.mixedOut <- outBytes:
	default:
		// Buffer pieno, droppa frame (meglio che bloccare il mixer)
	}
}

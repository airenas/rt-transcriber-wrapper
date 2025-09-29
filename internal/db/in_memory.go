package db

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/airenas/go-app/pkg/goapp"
	"github.com/airenas/rt-transcriber-wrapper/internal/domain"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

type MemBuffer struct {
	buf []byte
	pos int64
}

func (m *MemBuffer) Write(p []byte) (int, error) {
	end := m.pos + int64(len(p))
	if end > int64(len(m.buf)) {
		newBuf := make([]byte, end)
		copy(newBuf, m.buf)
		m.buf = newBuf
	}
	copy(m.buf[m.pos:], p)
	m.pos = end
	return len(p), nil
}

func (m *MemBuffer) Seek(offset int64, whence int) (int64, error) {
	var newPos int64
	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos = m.pos + offset
	case io.SeekEnd:
		newPos = int64(len(m.buf)) + offset
	}
	if newPos < 0 {
		return 0, fmt.Errorf("negative position")
	}
	m.pos = newPos
	return newPos, nil
}

func (m *MemBuffer) Bytes() []byte {
	return m.buf
}

type MemoryDataManager struct {
	data    map[string][]byte
	configs map[string]*domain.User
	texts   map[string]*domain.Texts

	lock sync.RWMutex
}

func NewMemoryDataManager() *MemoryDataManager {
	return &MemoryDataManager{
		data:    make(map[string][]byte),
		configs: make(map[string]*domain.User),
		texts:   make(map[string]*domain.Texts),
	}
}

func (am *MemoryDataManager) SaveAudio(id string, chunks [][]byte) error {
	goapp.Log.Warn().Str("id", id).Msg("Save audio")
	am.lock.Lock()
	defer am.lock.Unlock()

	res, err := to_wav(chunks)
	if err != nil {
		return fmt.Errorf("to wav: %w", err)
	}
	am.data[id] = res
	return nil
}

func (am *MemoryDataManager) GetAudio(id string) ([]byte, error) {
	goapp.Log.Warn().Str("id", id).Msg("Getting audio")
	am.lock.RLock()
	defer am.lock.RUnlock()
	data, ok := am.data[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	return cp, nil
}

// GetConfig implements ConfigManager.
func (am *MemoryDataManager) GetConfig(userID string) (*domain.User, error) {
	am.lock.RLock()
	defer am.lock.RUnlock()
	data, ok := am.configs[userID]
	if !ok {
		return &domain.User{ID: userID}, nil
	}
	cp := *data
	return &cp, nil
}

// SaveConfig implements ConfigManager.
func (am *MemoryDataManager) SaveConfig(user *domain.User) error {
	am.lock.Lock()
	defer am.lock.Unlock()
	am.configs[user.ID] = user
	return nil
}

// GetTexts implements TextManager.
func (am *MemoryDataManager) GetTexts(ctx context.Context, userID string) (*domain.Texts, error) {
	am.lock.RLock()
	defer am.lock.RUnlock()

	data, ok := am.texts[userID]
	if !ok {
		return &domain.Texts{}, nil
	}
	cp := *data
	return &cp, nil
}

// SaveTexts implements TextManager.
func (am *MemoryDataManager) SaveTexts(ctx context.Context, userID string, input *domain.Texts) error {
	am.lock.Lock()
	defer am.lock.Unlock()

	am.texts[userID] = input
	return nil
}

func to_wav(chunks [][]byte) ([]byte, error) {
	var pcmData bytes.Buffer
	for _, chunk := range chunks {
		pcmData.Write(chunk)
	}

	raw := pcmData.Bytes()
	samples := make([]int, len(raw)/2)
	for i := 0; i < len(samples); i++ {
		samples[i] = int(int16(raw[2*i]) | int16(raw[2*i+1])<<8)
	}

	buf := &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: 1,
			SampleRate:  16000,
		},
		Data:           samples,
		SourceBitDepth: 16,
	}

	wavBuf := &MemBuffer{buf: make([]byte, 0)}
	enc := wav.NewEncoder(wavBuf, 16000, 16, 1, 1)
	if err := enc.Write(buf); err != nil {
		return nil, fmt.Errorf("write wav: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("close wav: %w", err)
	}

	return wavBuf.Bytes(), nil
}

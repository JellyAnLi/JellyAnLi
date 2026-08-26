package linker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type FileState struct {
	Mtime      time.Time `json:"mtime"`
	Size       int64     `json:"size"`
	TargetPath string    `json:"target_path"`
}

type SyncState struct {
	Files map[string]FileState `json:"files"`
}

func NewSyncState() *SyncState {
	return &SyncState{
		Files: make(map[string]FileState),
	}
}

// LoadState загружает состояние из файла state.json
func LoadState(path string) (*SyncState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewSyncState(), nil
		}
		return nil, err
	}

	state := NewSyncState()
	if err := json.Unmarshal(data, state); err != nil {
		return nil, err
	}
	return state, nil
}

// Save сохраняет состояние в файл state.json
func (s *SyncState) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

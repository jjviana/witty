package config

import (
	"github.com/autarch/testify/assert"
	"testing"
)

type TestConfig struct {
	Hello string
	N     int
	F     float64
}

func TestRepository(t *testing.T) {

	tempDir := t.TempDir()
	repo := NewRepository(tempDir)

	config := TestConfig{
		Hello: "world",
		N:     42,
		F:     3.14,
	}

	err := repo.Store("test", config)
	if err != nil {
		t.Errorf("Failed to store config: %v", err)
	}

	var loadedConfig TestConfig
	err = repo.Load("test", &loadedConfig)
	if err != nil {
		t.Errorf("Failed to load config: %v", err)
	}

	assert.Equal(t, config, loadedConfig)

	err = repo.Load("test2", config)
	assert.Error(t, err)

}

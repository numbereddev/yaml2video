package controller

import (
	"log"
	"os"
	"path/filepath"
	"time"
)

type OutputArtifact struct {
	controller *Controller
	dir        string
	key        string
	File       *os.File
	Info       os.FileInfo
}

func (c *Controller) OpenOutput(id string) (*OutputArtifact, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	dir := c.jobDir(id)
	metadata, err := readMetadata(dir)
	if err != nil || metadata.ID != id || !metadata.Ready || expired(metadata, time.Now()) {
		return nil, os.ErrNotExist
	}
	file, err := os.Open(filepath.Join(dir, "output.mp4"))
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		file.Close()
		return nil, os.ErrNotExist
	}
	key := jobKey(id)
	c.active[key]++
	return &OutputArtifact{controller: c, dir: dir, key: key, File: file, Info: info}, nil
}

func (a *OutputArtifact) Close() {
	defer a.File.Close()
	c := a.controller
	c.mu.Lock()
	defer c.mu.Unlock()
	c.release(a.key)
}

func (a *OutputArtifact) MarkFreeable() {
	c := a.controller
	c.mu.Lock()
	defer c.mu.Unlock()
	metadata, err := readMetadata(a.dir)
	if err == nil && metadata.FreeableAt == nil {
		now := time.Now().UTC()
		metadata.FreeableAt = &now
		if err := writeMetadata(a.dir, metadata); err != nil {
			log.Printf("mark render freeable: %v", err)
		}
	}
}

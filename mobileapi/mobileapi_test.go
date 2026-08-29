package mobileapi

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type recordingListener struct {
	mu     sync.Mutex
	events []string
}

func (listener *recordingListener) OnEvent(event string) {
	listener.mu.Lock()
	listener.events = append(listener.events, event)
	listener.mu.Unlock()
}

func TestSessionInspectsAndExtractsGenericZIP(t *testing.T) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	metadata, err := writer.Create("build.prop")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = metadata.Write([]byte("ro.product.model=Test Phone\nro.product.device=test_device\nro.build.version.release=15\n"))
	image, err := writer.Create("images/init_boot.img")
	if err != nil {
		t.Fatal(err)
	}
	wantImage := bytes.Repeat([]byte{0x42}, 4096)
	_, _ = image.Write(wantImage)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(t.TempDir(), "firmware.zip")
	if err := os.WriteFile(inputPath, archive.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	listener := &recordingListener{}
	session := NewSession(inputPath, "firmware.zip")
	defer session.Close()
	detailsText, err := session.Inspect(listener)
	if err != nil {
		t.Fatal(err)
	}
	var details detailsJSON
	if err := json.Unmarshal([]byte(detailsText), &details); err != nil {
		t.Fatal(err)
	}
	if details.Mode != "zip" || len(details.Partitions) != 1 || details.Partitions[0].Name != "init_boot" {
		t.Fatalf("unexpected details: %#v", details)
	}
	if details.Info.Model != "Test Phone" || details.Info.Device != "test_device" || details.Info.Android != "15" {
		t.Fatalf("unexpected device info: %#v", details.Info)
	}

	outputDir := t.TempDir()
	if err := session.Extract(outputDir, `["init_boot"]`, 4, listener); err != nil {
		t.Fatal(err)
	}
	gotImage, err := os.ReadFile(filepath.Join(outputDir, "init_boot.img"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotImage, wantImage) {
		t.Fatal("extracted image mismatch")
	}
	listener.mu.Lock()
	defer listener.mu.Unlock()
	if len(listener.events) == 0 {
		t.Fatal("no callback events were emitted")
	}
}

func TestSessionRejectsInvalidThreadsAndSelections(t *testing.T) {
	session := NewSession("missing.zip", "missing.zip")
	if err := session.Extract(t.TempDir(), `[]`, 4, nil); err == nil {
		t.Fatal("expected empty selection error")
	}
	if err := session.Extract(t.TempDir(), `["boot"]`, 0, nil); err == nil {
		t.Fatal("expected thread count error")
	}
}

func TestSetTempDir(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "mobile-temp")
	if err := SetTempDir(directory); err != nil {
		t.Fatal(err)
	}
	if stat, err := os.Stat(directory); err != nil || !stat.IsDir() {
		t.Fatalf("temporary directory was not created: %v", err)
	}
}

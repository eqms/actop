package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestParseVolumesFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/volumes.json")
	if err != nil {
		t.Fatal(err)
	}
	var vols []Volume
	if err := json.Unmarshal(data, &vols); err != nil {
		t.Fatalf("unmarshal volumes fixture: %v", err)
	}
	if len(vols) == 0 {
		t.Fatal("expected volumes in fixture")
	}
	v := vols[0]
	if v.ID == "" || v.Configuration.Name == "" {
		t.Errorf("missing volume identity: %+v", v)
	}
	if v.Configuration.Format != "ext4" {
		t.Errorf("expected ext4 format, got %q", v.Configuration.Format)
	}
	if v.Configuration.SizeInBytes <= 0 {
		t.Errorf("implausible size: %d", v.Configuration.SizeInBytes)
	}
	if v.Configuration.CreationDate.IsZero() {
		t.Error("missing creation date")
	}
}

func TestParseImagesFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/images.json")
	if err != nil {
		t.Fatal(err)
	}
	var imgs []Image
	if err := json.Unmarshal(data, &imgs); err != nil {
		t.Fatalf("unmarshal images fixture: %v", err)
	}
	if len(imgs) == 0 {
		t.Fatal("expected images in fixture")
	}
	img := imgs[0]
	if !strings.Contains(img.Configuration.Name, "/") {
		t.Errorf("unexpected image reference: %q", img.Configuration.Name)
	}
	if len(img.ShortDigest()) != 12 {
		t.Errorf("ShortDigest() = %q, want 12 chars", img.ShortDigest())
	}
	if len(img.Variants) == 0 {
		t.Error("expected variants")
	}
}

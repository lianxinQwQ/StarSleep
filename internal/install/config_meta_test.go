package install

import (
	"bytes"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestInjectConfigMetaReplacesMetaAtFileStart(t *testing.T) {
	input := []byte(`meta:
  work_dir: /old
layers:
  - base.yaml
inherit:
  - /etc/hostname
`)

	out, err := injectConfigMeta(input, installMetaConfig())
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := yaml.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is invalid yaml: %v\n%s", err, out)
	}
	if bytes.Count(out, []byte("meta:")) != 1 {
		t.Fatalf("meta appears %d times:\n%s", bytes.Count(out, []byte("meta:")), out)
	}
	if _, ok := decoded["layers"]; !ok {
		t.Fatalf("layers key was lost:\n%s", out)
	}
	if _, ok := decoded["inherit"]; !ok {
		t.Fatalf("inherit key was lost:\n%s", out)
	}

	meta := decoded["meta"].(map[string]any)
	if got := meta["work_dir"]; got != TargetStarsleepMount {
		t.Fatalf("work_dir = %v", got)
	}
	if got := meta["snapshot_dir"]; got != TargetStarsleepMount+"/snapshots" {
		t.Fatalf("snapshot_dir = %v", got)
	}
	if got := meta["pkg_cache"]; got != TargetStarsleepMount+"/shared/pacman-cache" {
		t.Fatalf("pkg_cache = %v", got)
	}
	if got := meta["db_path"]; got != "var/lib/pacman" {
		t.Fatalf("db_path = %v", got)
	}
}

func TestInjectConfigMetaAddsMetaWithoutDroppingLayers(t *testing.T) {
	input := []byte(`layers:
  - base.yaml
`)

	out, err := injectConfigMeta(input, installMetaConfig())
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := yaml.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is invalid yaml: %v\n%s", err, out)
	}
	if bytes.Count(out, []byte("meta:")) != 1 {
		t.Fatalf("meta appears %d times:\n%s", bytes.Count(out, []byte("meta:")), out)
	}
	if _, ok := decoded["layers"]; !ok {
		t.Fatalf("layers key was lost:\n%s", out)
	}
}

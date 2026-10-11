//go:build windows

package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestVetExtraRootRefusesEveryWindowsSpellingOfASecretDir(t *testing.T) {
	cfg, _, userHome, primary, _ := rootsFixture(t)
	ssh := filepath.Join(userHome, ".ssh")
	vol := filepath.VolumeName(ssh)
	rest := strings.TrimPrefix(ssh, vol)
	for name, dir := range map[string]string{
		"extended-length": `\\?\` + ssh,
		"device":          `\\.\` + ssh,
		"loopback share":  `\\localhost\` + strings.TrimSuffix(vol, ":") + `$` + rest,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := cfg.VetExtraRoot(primary, dir, nil); err == nil {
				t.Fatalf("%s was accepted", dir)
			}
			if _, refused := cfg.Roots().FolderGrantRefusal(dir); !refused {
				t.Fatalf("trust and allow_write accepted %s", dir)
			}
		})
	}
}

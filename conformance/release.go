package conformance

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Relayward/relayward-sdk/manifest"
)

const PluginReleaseManifest = "relayward-plugin.json"

func VerifyPluginRelease(directory string) (manifest.Manifest, error) {
	info, err := os.Lstat(directory)
	if err != nil {
		return manifest.Manifest{}, fmt.Errorf("inspect plugin release directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return manifest.Manifest{}, errors.New("plugin release path must be a directory")
	}
	value, err := LoadManifest(filepath.Join(directory, PluginReleaseManifest))
	if err != nil {
		return manifest.Manifest{}, err
	}
	for _, artifact := range value.Artifacts {
		path := filepath.Join(directory, artifact.File)
		info, err := os.Lstat(path)
		if err != nil {
			return manifest.Manifest{}, fmt.Errorf("inspect %s artifact: %w", artifact.Role, err)
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return manifest.Manifest{}, fmt.Errorf("%s artifact must be a regular file", artifact.Role)
		}
		if info.Size() != artifact.Size {
			return manifest.Manifest{}, fmt.Errorf("%s artifact size does not match the manifest", artifact.Role)
		}
		file, err := os.Open(path)
		if err != nil {
			return manifest.Manifest{}, fmt.Errorf("open %s artifact: %w", artifact.Role, err)
		}
		digest := sha256.New()
		_, copyErr := io.Copy(digest, io.LimitReader(file, artifact.Size+1))
		closeErr := file.Close()
		if copyErr != nil || closeErr != nil {
			return manifest.Manifest{}, fmt.Errorf("read %s artifact", artifact.Role)
		}
		if hex.EncodeToString(digest.Sum(nil)) != artifact.SHA256 {
			return manifest.Manifest{}, fmt.Errorf("%s artifact SHA-256 does not match the manifest", artifact.Role)
		}
	}
	return value, nil
}

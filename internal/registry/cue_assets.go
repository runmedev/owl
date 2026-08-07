package registry

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"cuelang.org/go/cue/load"

	cuemod "github.com/runmedev/owl/cue.mod"
	"github.com/runmedev/owl/schema"
	"github.com/runmedev/owl/types"
)

const embeddedCUERoot = "/owl-cue"

func embeddedCUEConfig() (load.Config, error) {
	overlay := make(map[string]load.Source)
	assets := []struct {
		prefix string
		fs     fs.FS
	}{
		{prefix: "cue.mod", fs: cuemod.BuiltInFS},
		{prefix: "schema", fs: schema.BuiltInFS},
		{prefix: "types", fs: types.BuiltInFS},
	}
	for _, asset := range assets {
		if err := addCUEOverlay(overlay, asset.prefix, asset.fs); err != nil {
			return load.Config{}, err
		}
	}
	return load.Config{Dir: embeddedCUERoot, Overlay: overlay}, nil
}

func addCUEOverlay(overlay map[string]load.Source, prefix string, assets fs.FS) error {
	return fs.WalkDir(assets, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		raw, err := fs.ReadFile(assets, name)
		if err != nil {
			return fmt.Errorf("read embedded CUE asset %s/%s: %w", prefix, name, err)
		}
		filename := filepath.Join(embeddedCUERoot, prefix, filepath.FromSlash(name))
		overlay[filename] = load.FromBytes(raw)
		return nil
	})
}

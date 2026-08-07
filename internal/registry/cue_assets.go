package registry

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue/load"

	cuemod "github.com/runmedev/owl/cue.mod"
	"github.com/runmedev/owl/schema"
	"github.com/runmedev/owl/types"
)

const embeddedCUERoot = "/owl-cue"

type embeddedCUECatalogSource struct{}

func (embeddedCUECatalogSource) LoadConfig() (load.Config, error) {
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

type directoryCUECatalogSource struct {
	root string
}

func (s directoryCUECatalogSource) LoadConfig() (load.Config, error) {
	if s.root == "" {
		return load.Config{}, errors.New("CUE catalog root is empty")
	}
	root, err := filepath.Abs(s.root)
	if err != nil {
		return load.Config{}, fmt.Errorf("resolve CUE catalog root %q: %w", s.root, err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return load.Config{}, fmt.Errorf("access CUE catalog root %q: %w", root, err)
	}
	if !info.IsDir() {
		return load.Config{}, fmt.Errorf("CUE catalog root %q is not a directory", root)
	}

	type requiredCatalogPath struct {
		name  string
		isDir bool
	}
	required := []requiredCatalogPath{
		{name: filepath.Join("cue.mod", "module.cue")},
		{name: "schema", isDir: true},
		{name: "types", isDir: true},
	}
	for _, spec := range builtInCUETypes {
		required = append(required, requiredCatalogPath{
			name:  filepath.FromSlash(strings.TrimPrefix(spec.importPath, "./")),
			isDir: true,
		})
	}
	for _, requiredPath := range required {
		path := filepath.Join(root, requiredPath.name)
		entry, statErr := os.Stat(path)
		if statErr != nil {
			return load.Config{}, fmt.Errorf("CUE catalog root %q is incomplete: required %s: %w", root, filepath.ToSlash(requiredPath.name), statErr)
		}
		if entry.IsDir() != requiredPath.isDir {
			kind := "directory"
			if !requiredPath.isDir {
				kind = "file"
			}
			return load.Config{}, fmt.Errorf("CUE catalog root %q is incomplete: required %s is not a %s", root, filepath.ToSlash(requiredPath.name), kind)
		}
	}
	return load.Config{Dir: root}, nil
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

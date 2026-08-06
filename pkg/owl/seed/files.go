package seed

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/runmedev/owl/internal/requirements"
)

func resolveConfigPath(workDir string, explicit string, required bool) (string, error) {
	if explicit != "" {
		path := pathInWorkDir(workDir, explicit)
		if _, err := os.Stat(path); err != nil {
			return "", err
		}
		return path, nil
	}
	var found []string
	for _, candidate := range []string{"owl.toml", "owl.yaml", "owl.yml", "owl.json"} {
		path := pathInWorkDir(workDir, candidate)
		if _, err := os.Stat(path); err == nil {
			found = append(found, path)
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	if len(found) > 1 {
		return "", errors.New("multiple Owl config files found; pass --config <path>")
	}
	if len(found) == 1 {
		return found[0], nil
	}
	if required {
		return "", errors.New("owl config not found; pass --config <path> or create owl.toml")
	}
	return "", nil
}

func validateNoHumanDotenvSpecs(workDir string, specFiles []string) error {
	files := specFiles
	if len(files) == 0 {
		files = []string{".env.sample", ".env.example", ".env.spec"}
	}
	for _, file := range files {
		raw, err := os.ReadFile(pathInWorkDir(workDir, file))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !isGeneratedDotenvSpec(raw) {
			return errors.New("dotenv spec file exists beside Owl config; move it aside or regenerate it with owl project spec --write")
		}
	}
	return nil
}

func isGeneratedDotenvSpec(raw []byte) bool {
	return strings.HasPrefix(string(raw), requirements.GeneratedDotenvSpecHeaderPrefix)
}

func filesOrDefaults(workDir string, files []string, defaults ...string) ([]string, error) {
	if len(files) > 0 {
		resolved := make([]string, 0, len(files))
		for _, file := range files {
			resolved = append(resolved, pathInWorkDir(workDir, file))
		}
		return resolved, nil
	}

	var existing []string
	for _, file := range defaults {
		path := pathInWorkDir(workDir, file)
		if _, err := os.Stat(path); err == nil {
			existing = append(existing, path)
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	return existing, nil
}

func pathInWorkDir(workDir string, path string) string {
	if workDir == "" || workDir == "." || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(workDir, path)
}

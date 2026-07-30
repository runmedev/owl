package requirements

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/runmedev/owl/internal/model"
)

func ReadConfigFile(path string) (model.ConfigInput, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return model.ConfigInput{}, err
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return ParseJSONConfig(raw)
	case ".yaml", ".yml":
		return ParseYAMLConfig(raw)
	case ".toml":
		return ParseTOMLConfig(raw)
	default:
		return model.ConfigInput{}, errors.New("unsupported Owl config extension; use .toml, .yaml, .yml, or .json")
	}
}

func ParseJSONConfig(raw []byte) (model.ConfigInput, error) {
	var input model.ConfigInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return model.ConfigInput{}, err
	}
	return input, nil
}

func ParseYAMLConfig(raw []byte) (model.ConfigInput, error) {
	var input model.ConfigInput
	if err := yaml.Unmarshal(raw, &input); err != nil {
		return model.ConfigInput{}, err
	}
	return input, nil
}

type tomlConfig struct {
	Needs map[string]map[string]tomlNeed `toml:"needs"`
}

type tomlNeed struct {
	Type   string            `toml:"type"`
	Dotenv map[string]string `toml:"dotenv"`
}

func ParseTOMLConfig(raw []byte) (model.ConfigInput, error) {
	var cfg tomlConfig
	if err := toml.Unmarshal(raw, &cfg); err != nil {
		return model.ConfigInput{}, err
	}
	var kinds []string
	for kind := range cfg.Needs {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	var input model.ConfigInput
	for _, kind := range kinds {
		var instances []string
		for instance := range cfg.Needs[kind] {
			instances = append(instances, instance)
		}
		sort.Strings(instances)
		for _, instance := range instances {
			need := cfg.Needs[kind][instance]
			inputNeed := model.NeedInput{
				ID:       kind + "." + instance,
				Type:     model.TypeID(need.Type),
				Instance: instance,
			}
			if len(need.Dotenv) > 0 {
				var fields []string
				for field := range need.Dotenv {
					fields = append(fields, field)
				}
				sort.Strings(fields)
				inputNeed.Dotenv = &model.DotenvProjectionInput{}
				for _, field := range fields {
					inputNeed.Dotenv.Fields = append(inputNeed.Dotenv.Fields, model.DotenvFieldBindingInput{
						Field: field,
						Key:   need.Dotenv[field],
					})
				}
			}
			input.Needs = append(input.Needs, inputNeed)
		}
	}
	return input, nil
}

package model

import (
	"fmt"
	"strings"
)

type TypeID string

const (
	typeIDPrefix = "github.com/runmedev/owl/types/"
)

const (
	TypeCoreOpaque TypeID = typeIDPrefix + "core/opaque"
	TypeCorePlain  TypeID = typeIDPrefix + "core/plain"
	TypeCoreSecret TypeID = typeIDPrefix + "core/secret"
	TypeCoreURL    TypeID = typeIDPrefix + "core/url"

	TypeUniverseRedis     TypeID = typeIDPrefix + "universe/redis"
	TypeUniverseOpenAI    TypeID = typeIDPrefix + "universe/openai"
	TypeUniverseAnthropic TypeID = typeIDPrefix + "universe/anthropic"
)

var canonicalTypeAliases = map[string]TypeID{
	"core/opaque":        TypeCoreOpaque,
	"core/plain":         TypeCorePlain,
	"core/secret":        TypeCoreSecret,
	"core/url":           TypeCoreURL,
	"universe/redis":     TypeUniverseRedis,
	"universe/openai":    TypeUniverseOpenAI,
	"universe/anthropic": TypeUniverseAnthropic,
}

var typeAliases = map[string]TypeID{
	"core/opaque":        TypeCoreOpaque,
	"core/plain":         TypeCorePlain,
	"core/secret":        TypeCoreSecret,
	"core/url":           TypeCoreURL,
	"universe/redis":     TypeUniverseRedis,
	"universe/openai":    TypeUniverseOpenAI,
	"universe/anthropic": TypeUniverseAnthropic,
}

var knownTypeIDs = map[TypeID]struct{}{
	TypeCoreOpaque:        {},
	TypeCorePlain:         {},
	TypeCoreSecret:        {},
	TypeCoreURL:           {},
	TypeUniverseRedis:     {},
	TypeUniverseOpenAI:    {},
	TypeUniverseAnthropic: {},
}

func ParseTypeID(ref string) (TypeID, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("type id is empty")
	}
	if alias, ok := typeAliases[ref]; ok {
		return alias, nil
	}
	if strings.HasPrefix(ref, typeIDPrefix) {
		if ref != strings.ToLower(ref) {
			return "", fmt.Errorf("type id %q must be lowercase", ref)
		}
		id := TypeID(ref)
		if _, ok := knownTypeIDs[id]; ok {
			return id, nil
		}
		return "", fmt.Errorf("unknown type id %q", ref)
	}
	return "", fmt.Errorf("unknown type alias %q", ref)
}

func (id TypeID) Alias() string {
	for alias, candidate := range canonicalTypeAliases {
		if candidate == id {
			return alias
		}
	}
	return string(id)
}

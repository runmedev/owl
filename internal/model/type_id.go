package model

import (
	"fmt"
	"strings"
)

type TypeID string

const typeIDPrefix = "https://owl.runme.dev/v1/types/"

const (
	TypeCoreOpaque TypeID = typeIDPrefix + "core/opaque"
	TypeCoreSecret TypeID = typeIDPrefix + "core/secret"
	TypeCoreURL    TypeID = typeIDPrefix + "core/url"
	TypeCoreHost   TypeID = typeIDPrefix + "core/host"
	TypeCorePort   TypeID = typeIDPrefix + "core/port"

	TypeUniverseRedis TypeID = typeIDPrefix + "universe/redis"
)

var typeAliases = map[string]TypeID{
	"core/opaque":    TypeCoreOpaque,
	"core/secret":    TypeCoreSecret,
	"core/url":       TypeCoreURL,
	"core/host":      TypeCoreHost,
	"core/port":      TypeCorePort,
	"universe/redis": TypeUniverseRedis,
}

var knownTypeIDs = map[TypeID]struct{}{
	TypeCoreOpaque:    {},
	TypeCoreSecret:    {},
	TypeCoreURL:       {},
	TypeCoreHost:      {},
	TypeCorePort:      {},
	TypeUniverseRedis: {},
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
	for alias, candidate := range typeAliases {
		if candidate == id {
			return alias
		}
	}
	return string(id)
}

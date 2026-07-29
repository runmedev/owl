package redis

import (
	corehost "github.com/runmedev/owl/types/core/host"
	coreport "github.com/runmedev/owl/types/core/port"
	coresecret "github.com/runmedev/owl/types/core/secret"
	owl "github.com/runmedev/owl/schema"
)

#Redis: owl.#Type & {
	id:          "github.com/runmedev/owl/types/universe/redis"
	kind:        "composite"
	description: "Redis connection configuration."

	fields: {
		host: owl.#Field & {
			type:        "github.com/runmedev/owl/types/core/host"
			description: "Redis server hostname."
			visibility:  "literal"
		}
		port: owl.#Field & {
			type:        "github.com/runmedev/owl/types/core/port"
			description: "Redis server port."
			visibility:  "literal"
		}
		password: owl.#Field & {
			type:        "github.com/runmedev/owl/types/core/secret"
			description: "Redis password."
			visibility:  "masked"
		}
	}
}

#RedisValue: {
	host:     corehost.HostValue
	port:     coreport.PortValue
	password: coresecret.SecretValue
}

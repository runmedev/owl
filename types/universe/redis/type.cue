package redis

import (
	"net"

	coresecret "github.com/runmedev/owl/types/core/secret"
	owl "github.com/runmedev/owl/schema"
)

#Redis: owl.#Type & {
	id:          "github.com/runmedev/owl/types/universe/redis"
	kind:        "composite"
	description: "Redis connection configuration."

	fields: {
		host: owl.#Field & {
			type:        "github.com/runmedev/owl/types/core/plain"
			description: "Redis server hostname."
			visibility:  "literal"
		}
		port: owl.#Field & {
			type:        "github.com/runmedev/owl/types/core/plain"
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
	host:     #RedisHostValue
	port:     uint & >=1 & <=65535
	password: coresecret.SecretValue
}

#RedisHostnameValue: string & =~"^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?(?:\\.[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)*\\.?$" & =~"[A-Za-z]"

#RedisHostValue: net.IP | #RedisHostnameValue

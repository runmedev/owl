package redis

import (
	"net"

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
			value:       #RedisHostValue
		}
		port: owl.#Field & {
			type:        "github.com/runmedev/owl/types/core/plain"
			description: "Redis server port."
			visibility:  "literal"
			value:       uint & >=1 & <=65535
		}
		password: owl.#Field & {
			type:        "github.com/runmedev/owl/types/core/secret"
			description: "Redis password."
			visibility:  "masked"
			value:       string & !=""
		}
	}
}

#RedisHostnameValue: string & =~"^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?(?:\\.[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)*\\.?$" & =~"[A-Za-z]"

#RedisHostValue: net.IP | #RedisHostnameValue

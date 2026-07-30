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
			value:       #RedisHostValue
		}
		port: owl.#Field & {
			type:        "github.com/runmedev/owl/types/core/plain"
			description: "Redis server port."
			value:       (uint & >=1 & <=65535) | error("must be an integer between 1 and 65535")
		}
		password: owl.#Field & {
			type:        "github.com/runmedev/owl/types/core/secret"
			description: "Redis password."
			value:       (string & !="") | error("must not be empty")
		}
	}
}

#RedisHostnameValue: string & =~"^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?(?:\\.[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)*\\.?$" & =~"[A-Za-z]"

#RedisHostValue: net.IP | #RedisHostnameValue | error("must be an IP address or DNS hostname")

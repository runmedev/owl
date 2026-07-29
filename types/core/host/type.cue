package host

import (
	"net"

	owl "github.com/runmedev/owl/schema"
)

#Host: owl.#Type & {
	id:          "github.com/runmedev/owl/types/core/host"
	kind:        "primitive"
	description: "Host-shaped string-carried environment value."
}

#HostnameValue: string & =~"^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?(?:\\.[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)*\\.?$" & =~"[A-Za-z]"

#HostValue: net.IP | #HostnameValue

HostValue: #HostValue

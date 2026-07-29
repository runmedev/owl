package host

import owl "github.com/runmedev/owl/schema"
import "net"

#Host: owl.#Type & {
	id:          "github.com/runmedev/owl/types/core/host"
	kind:        "primitive"
	description: "Host-shaped string-carried environment value."
}

#HostnameValue: string & =~"^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?(?:\\.[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)*\\.?$" & =~"[A-Za-z]"

#HostValue: net.IP | #HostnameValue

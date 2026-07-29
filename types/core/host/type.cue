package host

import owl "github.com/runmedev/owl/schema"

#Host: owl.#Type & {
	id:          "github.com/runmedev/owl/types/core/host"
	kind:        "primitive"
	description: "Host-shaped string-carried environment value."
}

#HostValue: string & !=""

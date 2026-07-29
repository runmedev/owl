package port

import owl "github.com/runmedev/owl/schema"

#Port: owl.#Type & {
	id:          "github.com/runmedev/owl/types/core/port"
	kind:        "primitive"
	description: "Port-shaped string-carried environment value."
}

#PortValue: int & >=1 & <=65535

PortValue: #PortValue

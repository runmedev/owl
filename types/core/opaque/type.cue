package opaque

import owl "github.com/runmedev/owl/schema"

#Opaque: owl.#Type & {
	id:          "github.com/runmedev/owl/types/core/opaque"
	kind:        "primitive"
	description: "Unknown string-carried environment value with unknown semantics and sensitivity."
	value:       string
}

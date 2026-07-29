package plain

import owl "github.com/runmedev/owl/schema"

#Plain: owl.#Type & {
	id:          "github.com/runmedev/owl/types/core/plain"
	kind:        "primitive"
	description: "Known non-sensitive string-carried environment value with no narrower semantic contract."
}

#PlainValue: string

package plain

import owl "github.com/runmedev/owl/schema"

#Plain: owl.#Type & {
	id:          "github.com/runmedev/owl/types/core/plain"
	kind:        "primitive"
	description: "Known plaintext string-carried environment value with no narrower semantic contract."
	sensitivity: "plaintext"
	value:       string
}

package secret

import owl "github.com/runmedev/owl/schema"

#Secret: owl.#Type & {
	id:          "github.com/runmedev/owl/types/core/secret"
	kind:        "primitive"
	description: "Sensitive string-carried environment value."
	sensitivity: "sensitive"
	value:       (string & !="") | error("must not be empty")
}

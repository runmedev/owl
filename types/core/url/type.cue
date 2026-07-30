package url

import owl "github.com/runmedev/owl/schema"

#URL: owl.#Type & {
	id:          "github.com/runmedev/owl/types/core/url"
	kind:        "primitive"
	description: "URL-shaped string-carried environment value."
	sensitivity: "plaintext"
	value:       (string & =~"^[a-zA-Z][a-zA-Z0-9+.-]*://") | error("must be an absolute URL")
}

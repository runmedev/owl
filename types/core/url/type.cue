package url

import owl "github.com/runmedev/owl/schema"

#URL: owl.#Type & {
	id:          "github.com/runmedev/owl/types/core/url"
	kind:        "primitive"
	description: "URL-shaped string-carried environment value."
	value:       string & =~"^[a-zA-Z][a-zA-Z0-9+.-]*://"
}

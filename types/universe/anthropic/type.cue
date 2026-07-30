package anthropic

import owl "github.com/runmedev/owl/schema"

#Anthropic: owl.#Type & {
	id:          "github.com/runmedev/owl/types/universe/anthropic"
	kind:        "composite"
	description: "Anthropic API client configuration."

	fields: {
		apiKey: owl.#Field & {
			type:        "github.com/runmedev/owl/types/core/secret"
			required:    true
			description: "Anthropic API key."
			value:       (string & !="") | error("must not be empty")
		}
		baseURL: owl.#Field & {
			type:        "github.com/runmedev/owl/types/core/url"
			required:    false
			description: "Anthropic API base URL."
			value:       (string & =~"^[a-zA-Z][a-zA-Z0-9+.-]*://") | error("must be an absolute URL")
		}
	}
}

package openai

import owl "github.com/runmedev/owl/schema"

#OpenAI: owl.#Type & {
	id:          "github.com/runmedev/owl/types/universe/openai"
	kind:        "composite"
	description: "OpenAI API client configuration."

	fields: {
		apiKey: owl.#Field & {
			type:        "github.com/runmedev/owl/types/core/secret"
			required:    true
			description: "OpenAI API key."
			value:       (string & !="") | error("must not be empty")
		}
		baseURL: owl.#Field & {
			type:        "github.com/runmedev/owl/types/core/url"
			required:    false
			description: "OpenAI API base URL."
			value:       (string & =~"^[a-zA-Z][a-zA-Z0-9+.-]*://") | error("must be an absolute URL")
		}
		organization: owl.#Field & {
			type:        "github.com/runmedev/owl/types/core/plain"
			required:    false
			description: "OpenAI organization ID."
			value:       (string & !="") | error("must not be empty")
		}
		project: owl.#Field & {
			type:        "github.com/runmedev/owl/types/core/plain"
			required:    false
			description: "OpenAI project ID."
			value:       (string & !="") | error("must not be empty")
		}
	}
}

package schema

#Kind: "primitive" | "composite"

#Visibility: "literal" | "masked" | "hidden" | "unresolved"

#Type: {
	id:          string
	kind:        #Kind
	description: string

	if kind == "primitive" {
		value: _
	}

	if kind == "composite" {
		fields: [string]: #Field
	}
}

#Field: {
	type:        string
	required:    bool | *true
	description: string
	visibility:  #Visibility
	value:       _
}

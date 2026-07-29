package schema

#Kind: "primitive" | "composite"

#Visibility: "literal" | "masked" | "hidden" | "unresolved"

#Type: {
	id:          string
	kind:        #Kind
	description: string

	fields?: [string]: #Field
}

#Field: {
	type:        string
	required:    bool | *true
	description: string
	visibility:  #Visibility
}

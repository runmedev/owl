package schema

#Kind: "primitive" | "composite"

#Sensitivity: "unknown" | "plaintext" | "sensitive"

#Type: {
	id:          string
	kind:        #Kind
	description: string

	if kind == "primitive" {
		sensitivity: #Sensitivity
		value:       _
	}

	if kind == "composite" {
		fields: [string]: #Field
	}
}

#Field: {
	type:        string
	required:    bool
	description: string
	sensitivity?: #Sensitivity
	value:       _
}

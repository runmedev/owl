package graph

import (
	"encoding/json"
	"fmt"
	"os"
)

func init() {
	if os.Getenv("OWL_GQL_TRACE") == "" {
		return
	}
	traceGraphQLQuery = func(query string, vars map[string]interface{}) {
		formattedVars, err := json.MarshalIndent(vars, "", "  ")
		if err != nil {
			formattedVars = []byte(fmt.Sprintf("%#v", vars))
		}
		_, _ = fmt.Fprintf(os.Stderr, "\n--- owl graphql ---\n%s\nvariables: %s\n", query, formattedVars)
	}
}

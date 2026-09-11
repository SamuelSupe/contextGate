package adapter

import (
	"regexp"
	"strings"

	"github.com/SamuelSupe/mcpdbhub/internal/model"
)

var dynamicCypherTarget = regexp.MustCompile(`:\s*\$(?:(?:any|all)\s*)?\(`)
var clickhouseIdentifierParameter = regexp.MustCompile(`(?i)\{[^{}]+:\s*Identifier\s*\}`)

// CheckTemplateTargets closes native parameter features that can select schema
// objects instead of values. The normal read-only classifiers still run later.
func CheckTemplateTargets(src model.Source, q model.Query) error {
	denied := func() error {
		return model.Fail("invalid_semantics", "Template database, table, label and relationship targets must be fixed; parameters may only supply values")
	}
	if src.Kind == "clickhouse" && clickhouseIdentifierParameter.MatchString(q.Query) {
		return denied()
	}
	if src.Kind == "neo4j" && dynamicCypherTarget.MatchString(q.Query) {
		return denied()
	}
	if src.Kind != "influxdb" || src.Version != "2" {
		return nil
	}
	tokens, err := lex(q.Query)
	if err != nil {
		return err
	}
	for i, token := range tokens {
		if token.quoted || token.text != "from" || i+1 >= len(tokens) || tokens[i+1].text != "(" {
			continue
		}
		// Flux from() accepts bucket or bucketID. Require its literal target rather
		// than a variable, conditional expression, concatenation or parameter.
		found := false
		for j := i + 2; j < len(tokens) && tokens[j].text != ")"; j++ {
			if strings.EqualFold(tokens[j].text, "bucket") || strings.EqualFold(tokens[j].text, "bucketID") {
				if j+3 >= len(tokens) || tokens[j+1].text != ":" || !tokens[j+2].quoted || (tokens[j+3].text != ")" && tokens[j+3].text != ",") {
					return denied()
				}
				found = true
			}
		}
		if !found {
			return denied()
		}
	}
	return nil
}

package schema

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed default_schema/mcp.json
var mcpDescriptionsData []byte

type KindDescription map[string][]string

func LoadKindDescriptions() (KindDescription, error) {
	var descriptions KindDescription
	err := json.Unmarshal(mcpDescriptionsData, &descriptions)
	if err != nil {
		return nil, fmt.Errorf("error loading kind descriptions: %w", err)
	}
	return descriptions, nil
}

func (k KindDescription) GetDescription(kind string) []string {
	return k[kind]
}

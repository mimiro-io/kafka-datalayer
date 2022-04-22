package coder

import (
	"encoding/json"
	"strings"
)

type Entity struct {
	Context    map[string]interface{} `json:"context,omitempty"`
	ID         string                 `json:"id"`
	Token      *string                `json:"token,omitempty"`
	IsDeleted  bool                   `json:"deleted"`
	References map[string]interface{} `json:"refs,omitempty"`
	Properties map[string]interface{} `json:"props"`
}

type Context struct {
	ID              string                 `json:"id"`
	Namespaces      map[string]interface{} `json:"namespaces"`
	NamespaceLookup map[string]string      `json:"omit"`
}

// NewEntity Create a new entity with global uuid and internal resource id
func NewEntity() *Entity {
	e := Entity{}
	e.Properties = make(map[string]interface{})
	e.References = make(map[string]interface{})
	return &e
}

func (entity *Entity) StripProps() ([]byte, error) {
	var stripped = make(map[string]interface{})
	stripped["id"] = strings.SplitAfter(entity.ID, ":")[1]
	stripped["deleted"] = entity.IsDeleted

	var singleMap = make(map[string]interface{})
	for e, _ := range entity.Properties {
		singleMap[strings.SplitAfter(e, ":")[1]] = entity.Properties[e]
	}
	stripped["props"] = singleMap
	return json.Marshal(stripped)
}

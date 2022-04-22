package coder

import (
	"github.com/bcicen/jstream"
	"io"
)

func ParseStream(reader io.Reader, emitEntity func(value *jstream.MetaValue) error) error {
	decoder := jstream.NewDecoder(reader, 1)

	for mv := range decoder.Stream() {
		err := emitEntity(mv)
		if err != nil {
			return err
		}
	}

	return nil
}

func AsContext(value *jstream.MetaValue) *Context {
	raw := value.Value.(map[string]interface{})
	ctx := &Context{
		ID: raw["id"].(string),
	}
	namespaces, ok := raw["namespaces"]
	if ok {
		ctx.Namespaces = namespaces.(map[string]interface{})

		// add the reverse namespace lookup
		lookup := make(map[string]string)
		for k, v := range ctx.Namespaces {
			lookup[v.(string)] = k
		}
		ctx.NamespaceLookup = lookup
	}
	return ctx
}

func AsEntity(value *jstream.MetaValue) *Entity {
	entity := NewEntity()
	raw := value.Value.(map[string]interface{})

	entity.ID = raw["id"].(string)

	if entity.ID == "@continuation" {
		t := raw["token"].(string)
		entity.Token = &t
	}

	deleted, ok := raw["deleted"]
	if ok {
		entity.IsDeleted = deleted.(bool)
	}

	props, ok := raw["props"]
	if ok {
		entity.Properties = props.(map[string]interface{})
	}
	return entity
}

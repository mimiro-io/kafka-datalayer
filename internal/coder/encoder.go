package coder

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	egdm "github.com/mimiro-io/entity-graph-data-model"
	"github.com/tidwall/gjson"

	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/conf"
)

type EntityEncoder struct {
	config  *conf.ConsumerConfig
	columns map[string]*conf.FieldMapping
}

func NewEntityEncoder(config *conf.ConsumerConfig) EntityEncoder {
	columns := make(map[string]*conf.FieldMapping)
	for _, m := range config.FieldMappings {
		columns[m.FieldName] = m
	}

	return EntityEncoder{config: config, columns: columns}
}

func (encoder EntityEncoder) Encode(kkey []byte, data []byte) *egdm.Entity {
	entity := egdm.NewEntity()

	js := string(data)
	key := string(kkey)

	// we need to convert the json into a map, so we can loop the fields
	items := make(map[string]interface{})
	_ = json.Unmarshal(data, &items)

	for k, v := range items {
		encoder.flatten("", k, v, js, entity, key)
	}

	return entity
}

func (encoder EntityEncoder) EncodeWithHeaders(kkey []byte, data []byte, kafkaHeaders []kafka.Header) *egdm.Entity {
	entity := egdm.NewEntity()

	js := string(data)
	key := string(kkey)

	// we need to convert the json into a map, so we can loop the fields
	items := make(map[string]interface{})
	_ = json.Unmarshal(data, &items)
	for k, v := range items {
		encoder.flatten("", k, v, js, entity, key)
	}
	// create the kafka headers map and marshal it as json so we can reuse flatten method
	headers := make(map[string]interface{})
	for i := range kafkaHeaders {
		headers[strings.ToLower(kafkaHeaders[i].Key)] = string(kafkaHeaders[i].Value)
	}
	headerData, _ := json.Marshal(headers)
	hd := string(headerData)
	// add the headers to the entity
	for k, v := range headers {
		encoder.flatten("kafka_header.", k, v, hd, entity, key)
	}
	return entity
}

func (encoder EntityEncoder) flatten(prefix string, k string, v interface{}, js string, entity *egdm.Entity, key string) {
	switch val := v.(type) {
	case map[string]interface{}:
		for k2, v2 := range val {
			encoder.flatten(prefix+k+".", k2, v2, js, entity, key)
		}
	case []interface{}:
		objArray := true
		for idx, i := range val {
			switch ival := i.(type) {
			case map[string]interface{}:
				for k2, v2 := range ival {
					encoder.flatten(fmt.Sprintf("%v%v[%v].", prefix, k, idx), k2, v2, js, entity, key)
				}
			default:
				objArray = false
			}
		}
		if !objArray && len(val) > 0 {
			switch val[0].(type) {
			case string:
				var stringArray []string
				for _, v := range val {
					stringArray = append(stringArray, v.(string))
				}
				encoder.flatten(prefix, k, stringArray, js, entity, key)
			case float64:
				var floatArray []float64
				for _, v := range val {
					floatArray = append(floatArray, v.(float64))
				}
				encoder.flatten(prefix, k, floatArray, js, entity, key)
			case bool:
				var boolArray []bool
				for _, v := range val {
					boolArray = append(boolArray, v.(bool))
				}
				encoder.flatten(prefix, k, boolArray, js, entity, key)
			}
		}
	default:
		fieldName := "ns0:" + prefix + k
		if mapping, ok := encoder.columns[k]; ok {
			if mapping.IgnoreField {
				return
			}

			propName := "ns0:" + mapping.FieldName
			if mapping.PropertyName != "" {
				propName = "ns0:" + mapping.PropertyName
			}

			value := gjson.Get(js, mapping.Path)
			if mapping.IsIDField {
				if mapping.Path == "kafkaKey" {
					entity.ID = encoder.config.BaseNameSpace + fmt.Sprintf(encoder.config.EntityIDConstructor, key)
				} else {
					entity.ID = encoder.config.BaseNameSpace + fmt.Sprintf(encoder.config.EntityIDConstructor, value.Value())
					entity.Properties[fieldName] = v
				}
			} else if mapping.IsDeletedField {
				entity.IsDeleted = value.Bool()
			} else if mapping.IsReference && value.Exists() {
				entity.References[propName] = fmt.Sprintf(mapping.ReferenceTemplate, value.Value())
				entity.Properties[fieldName] = v
			} else {
				entity.Properties[propName] = value.Value()
			}

		} else {
			entity.Properties[fieldName] = v
		}
	}
}
package conf

type KafkaConfig struct {
	ID        string           `json:"id"`
	Producers []ProducerConfig `json:"producers"`
	Consumers []ConsumerConfig `json:"consumers"`
}

type ProducerConfig struct {
	Dataset       string         `json:"dataset"`
	Topic         string         `json:"topic"`
	CreateTopic   bool           `json:"createTopic"`
	TopicSettings *TopicSettings `json:"topicSettings"`
	StripProps    bool           `json:"stripProps"`
	Key           *string        `json:"key"`
}

type TopicSettings struct {
	Partitions int                `json:"partitions"`
	Replicas   int                `json:"replicas"`
	Config     *map[string]string `json:"config"`
}

type SchemaRegistry struct {
	Location string `json:"location"`
}

type ProtobufSchema struct {
	// path on disk where protobuf schema files are located
	Path string `json:"path"`
	// file name or relative path (within `path`) of protobuf schema file that contains message type definition for given subscription
	FileName string `json:"fileName"`
	// name of root type for messages on given subscription
	Type string `json:"type"`
}

type ConsumerConfig struct {
	Dataset             string          `json:"dataset"`
	Topic               string          `json:"topic"`
	GroupID             string          `json:"groupId"`
	ValueDecoder        *string         `json:"valueDecoder"`
	Position            string          `json:"position"`
	NameSpace           string          `json:"nameSpace"`
	BaseNameSpace       string          `json:"baseNameSpace"`
	IncludeHeaders      bool            `json:"includeHeaders"`
	EntityIDConstructor string          `json:"entityIdConstructor"`
	Types               []string        `json:"types"`
	FieldMappings       []*FieldMapping `json:"fieldMappings"`
	SchemaRegistry      *SchemaRegistry `json:"schemaRegistry"`
	ProtobufSchema      *ProtobufSchema `json:"protobufSchema"`
}

type FieldMapping struct {
	Path              string `json:"path"`
	FieldName         string `json:"fieldName"`
	PropertyName      string `json:"propertyName"`
	IsIDField         bool   `json:"isIdField"`
	IsDeletedField    bool   `json:"isDeletedField"`
	IsReference       bool   `json:"isReference"`
	ReferenceTemplate string `json:"referenceTemplate"`
	IgnoreField       bool   `json:"ignoreField"`
}

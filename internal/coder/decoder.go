package coder

import (
	"encoding/binary"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/conf"
	"github.com/riferrei/srclient"
)

type Decoder interface {
	Decode(msg *kafka.Message) ([]byte, error)
}

func NewDecoder(config *conf.ConsumerConfig) (Decoder, error) {
	if config.ValueDecoder != nil {
		switch *config.ValueDecoder {
		case "avro":
			var schemaRegistryClient *srclient.SchemaRegistryClient
			if config.SchemaRegistry != nil && config.SchemaRegistry.Location != "" {
				schemaRegistryClient = srclient.CreateSchemaRegistryClient(config.SchemaRegistry.Location)
				return AvroDecoder{
					client: schemaRegistryClient,
					cache:  make(map[uint32]*srclient.Schema),
				}, nil
			}
			return nil, fmt.Errorf("avro decoder requires schemaRegistry.location."+
				" configured schemaRegistry: %+v", config.SchemaRegistry)
		case "protobuf":
			if config.ProtobufSchema != nil &&
				config.ProtobufSchema.FileName != "" &&
				config.ProtobufSchema.Type != "" &&
				config.ProtobufSchema.Path != "" {
				md, err := loadMessageDescriptor(config.ProtobufSchema)
				if err != nil {
					return nil, err

				}
				return GenericProtoDecoder{messageDescriptor: md}, nil
			}
			return nil, fmt.Errorf("protobuf decoder requires protobufSchema.path, type and fileName."+
				" configured protobufSchema: %+v", config.ProtobufSchema)
		}
	}
	return DefaultDecoder{}, nil
}

type DefaultDecoder struct{}

func (decoder DefaultDecoder) Decode(msg *kafka.Message) ([]byte, error) {
	return msg.Value, nil
}

type AvroDecoder struct {
	client *srclient.SchemaRegistryClient
	cache  map[uint32]*srclient.Schema
}

func (decoder AvroDecoder) Decode(msg *kafka.Message) ([]byte, error) {
	schemaID := binary.BigEndian.Uint32(msg.Value[1:5])

	var schema *srclient.Schema
	if s, ok := decoder.cache[schemaID]; ok {
		schema = s
	} else {
		s, err := decoder.client.GetSchema(int(schemaID))
		if err != nil {
			return nil, err
		}
		decoder.cache[schemaID] = s
		schema = s
	}

	native, _, _ := schema.Codec().NativeFromBinary(msg.Value[5:])
	return schema.Codec().TextualFromNative(nil, native)
}

type GenericProtoDecoder struct {
	messageDescriptor *desc.MessageDescriptor
}

func loadMessageDescriptor(protobufSchemaConf *conf.ProtobufSchema) (*desc.MessageDescriptor, error) {

	var protoParser protoparse.Parser
	//TODO: set protoParser.Accessor instead of importpaths - a function that can produce io.Readers from ConsumerConfig.
	protoParser.ImportPaths = append(protoParser.ImportPaths, protobufSchemaConf.Path)
	fds, err := protoParser.ParseFiles(protobufSchemaConf.FileName)
	if err != nil {
		return nil, err
	}
	fd := fds[0]
	//for _, x := range fd.GetMessageTypes() {
	//	fmt.Printf("%+v\n", x)
	//}
	messageDescriptor := fd.FindMessage(protobufSchemaConf.Type)
	return messageDescriptor, nil
}

func (decoder GenericProtoDecoder) Decode(msg *kafka.Message) ([]byte, error) {
	m := dynamic.NewMessage(decoder.messageDescriptor)
	err := m.Unmarshal(msg.Value)
	if err != nil {
		return nil, err
	}
	return m.MarshalJSONIndent()
}

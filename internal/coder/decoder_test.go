package coder

import (
	"encoding/json"
	"os"
	"path"
	"testing"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/franela/goblin"
	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/conf"
)

func TestDecoder(t *testing.T) {
	g := goblin.Goblin(t)
	g.Describe("A protobuf Decoder", func() {
		g.It("should covert a protobuf message to json text using generic protobuf Decoder", func() {

			var expected map[string]interface{}
			err := json.Unmarshal([]byte(`{"name": "test",
									"age": 2,
									"phones": [
										{"number": "555-100-200", "type": "HOME"},
										{"number": "555-100-300", "type": "WORK"}
									],
									"address": {
										"street": "Tøyengata",
										"houseNumber": 601
									},
									"lastUpdated": "2022-04-27T13:59:01Z"}`), &expected)
			g.Assert(err).IsNil()
			// the following file is a binary protobuf message. generated like this:
			// cd resources/test/integration_test_producer && npm run generate-wire-msg
			// it should contain the above data
			resourcesTestPath := "../../resources/test"
			overridePath := os.Getenv("RESOURCES_TEST_DIR")
			if overridePath != "" {
				resourcesTestPath = overridePath
			}
			wireMsg, err := os.ReadFile(path.Join(resourcesTestPath, "/protobuf-wire-person"))
			g.Assert(err).IsNil()

			d := "protobuf"
			dec, err := NewDecoder(&conf.ConsumerConfig{
				ValueDecoder: &d,
				ProtobufSchema: &conf.ProtobufSchema{
					Path:     path.Join(resourcesTestPath, "/protoschema"),
					FileName: "person.proto",
					Type:     "testdata.Person",
				},
			})
			g.Assert(err).IsNil()
			result, err := dec.Decode(&kafka.Message{
				Value: wireMsg,
			})
			g.Assert(err).IsNil()
			resMap := map[string]interface{}{}
			err = json.Unmarshal(result, &resMap)

			g.Assert(resMap).Eql(expected)
		})
	})
}

package coder

import (
	"testing"

	"github.com/franela/goblin"
	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/conf"
)

func TestEncoder(t *testing.T) {
	g := goblin.Goblin(t)
	g.Describe("The Entity encoder", func() {
		enc := NewEntityEncoder(&conf.ConsumerConfig{
			BaseNameSpace:       "test_ns/people/",
			EntityIdConstructor: "mainId/%v.json",
			FieldMappings: []*conf.FieldMapping{
				{
					Path:         "idkey",
					FieldName:    "idkey",
					PropertyName: "idkeyprop",
					IsIdField:    true,
				}, {
					Path:         "mappedkey",
					FieldName:    "mappedkey",
					PropertyName: "mappedprop",
				}, {
					Path:              "refkey",
					FieldName:         "refkey",
					PropertyName:      "refkeyprop",
					IsReference:       true,
					ReferenceTemplate: "http://foo/%v",
				}, {
					Path:              "refkey2",
					FieldName:         "refkey2",
					IsReference:       true,
					ReferenceTemplate: "http://bar/%v",
				}, {
					Path:              "refkey3",
					FieldName:         "refkey3",
					IsReference:       true,
					ReferenceTemplate: "http://bar/%v",
					IgnoreField:       true,
				}},
		})
		emptyEntity := Entity{
			References: map[string]interface{}{},
			Properties: map[string]interface{}{}}
		g.Describe("Encode", func() {
			g.It("Should encode primitives", func() {
				res := enc.Encode([]byte("kafkakey1"), []byte(`"hello"`))
				g.Assert(*res).Eql(emptyEntity, "cannot map properties, but should not stop streaming")
			})
			g.It("Should encode invalid json", func() {
				res := enc.Encode([]byte("kafkakey1"), []byte("{,sse]}"))
				g.Assert(*res).Eql(emptyEntity, "cannot map properties, but should not stop streaming")
			})
			g.It("Should encode nil", func() {
				res := enc.Encode([]byte("kafkakey1"), nil)
				g.Assert(*res).Eql(emptyEntity, "cannot map properties, but should not stop streaming")
			})
			g.It("Should encode simple object", func() {
				res := enc.Encode([]byte("kafkakey1"), []byte(`{
					"key1": "value1"
				}`))
				g.Assert(res).IsNotNil()
				g.Assert(res.ID).Eql("")
				g.Assert(res.IsDeleted).IsFalse()
				g.Assert(res.References).Eql(emptyEntity.References)
				g.Assert(res.Properties["ns0:key1"]).Eql("value1")
			})
			g.It("Should map id from simple object", func() {
				res := enc.Encode([]byte("kafkakey1"), []byte(`{
					"idkey": "value1"
				}`))
				g.Assert(res.ID).Eql("test_ns/people/mainId/value1.json")
				g.Assert(res.References).Eql(emptyEntity.References)
				g.Assert(res.Properties["ns0:idkey"]).Eql("value1")
			})
			g.It("Should map id from simple object", func() {
				res := enc.Encode([]byte("kafkakey1"), []byte(`{
					"idkey": "value1"
				}`))
				g.Assert(res.ID).Eql("test_ns/people/mainId/value1.json")
				g.Assert(res.References).Eql(emptyEntity.References)
				g.Assert(res.Properties["ns0:idkey"]).Eql("value1")
			})
			g.It("Should map reference from simple object", func() {
				res := enc.Encode([]byte("kafkakey1"), []byte(`{
					"refkey": "value1",
					"refkey2": "value2",
					"refkey3": "value3"
				}`))
				g.Assert(len(res.References)).Eql(2, "ignored ref field refkey3")
				g.Assert(len(res.Properties)).Eql(2, "ignored ref field refkey3")
				g.Assert(res.References["ns0:refkeyprop"]).Eql("http://foo/value1")
				g.Assert(res.References["ns0:refkey2"]).Eql("http://bar/value2")
				g.Assert(res.Properties["ns0:refkey"]).Eql("value1")
				g.Assert(res.Properties["ns0:refkey2"]).Eql("value2")
			})
			g.It("Should map nested object", func() {
				res := enc.Encode([]byte("kafkakey1"), []byte(`{
					"o1": {
						"n1": 1,
						"a1": ["av1", "av2"],
						"o2": {
							"a2": [
								{"n2": 2 },
								{"s2": "sv2"},
								{"a3": [3,4]},
								{"a4": [true,false]}
							]
						}
					},
					"s1": "sv1"
				}`))
				g.Assert(len(res.Properties)).Eql(7)
				g.Assert(res.Properties["ns0:s1"]).Eql("sv1")
				g.Assert(res.Properties["ns0:o1.n1"]).Eql(1.0)
				g.Assert(res.Properties["ns0:o1.a1"]).Eql([]string{"av1", "av2"})
				g.Assert(res.Properties["ns0:o1.o2.a2[0].n2"]).Eql(2.0)
				g.Assert(res.Properties["ns0:o1.o2.a2[1].s2"]).Eql("sv2")
				g.Assert(res.Properties["ns0:o1.o2.a2[2].a3"]).Eql([]float64{3, 4})
				g.Assert(res.Properties["ns0:o1.o2.a2[3].a4"]).Eql([]bool{true, false})
			})
		})
	})
}

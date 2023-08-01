//go:build integration

package kafkalayer_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/franela/goblin"
	egdm "github.com/mimiro-io/entity-graph-data-model"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"

	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/app"
)

type Foo struct{}

func (f Foo) Accept(log testcontainers.Log) {
	fmt.Fprintf(os.Stderr, "%s\n", log.Content)
}

func TestLayer(t *testing.T) {
	g := goblin.Goblin(t)
	g.Describe("The layer API", func() {
		var fxApp *fx.App
		layerUrl := "http://localhost:19899/datasets"
		var redPanda string
		var kafkaContainer testcontainers.Container
		g.Before(func() {
			resourcesTestPath := "./resources/test"
			overridePath := os.Getenv("RESOURCES_TEST_DIR")
			if overridePath != "" {
				resourcesTestPath = overridePath
			}

			p, _ := testcontainers.NewDockerProvider()
			addr, err := p.GetGatewayIP(context.Background())
			skipReaper := false
			if err != nil {
				// for some reason, there is no gateway address in our github actions runner.
				// we also must run without reaper there
				addr = "172.17.0.1"
				skipReaper = true
			}
			r := testcontainers.GenericContainerRequest{
				ContainerRequest: testcontainers.ContainerRequest{
					Image:        "docker.vectorized.io/vectorized/redpanda",
					ExposedPorts: []string{"9092:9092", "8081:8081", "29092:29092"},
					Cmd: []string{
						"redpanda", "start", "--smp", "1", "--reserve-memory", "0M", "--overprovisioned",
						"--node-id", "0",
						"--kafka-addr", "PLAINTEXT://0.0.0.0:29092,OUTSIDE://0.0.0.0:9092",
						"--advertise-kafka-addr", "PLAINTEXT://" + addr + ":29092,OUTSIDE://" + addr + ":9092",
					},
					// NetworkMode: "host",
					SkipReaper: skipReaper,
					WaitingFor: wait.ForLog("Redpanda!").WithStartupTimeout(time.Minute * 1),
				}, Started: true,
			}
			t.Log(r.Cmd)
			kafkaContainer, err = testcontainers.GenericContainer(context.Background(), r)

			if err != nil {
				t.Errorf("Failed when testcontainers: %v", err)
				t.FailNow()
			}

			// kafkaContainer.FollowOutput(Foo{})
			// kafkaContainer.StartLogProducer(context.Background())

			redPanda = fmt.Sprintf("%v:%v", addr, 9092)
			// t.Log(redPanda)

			_, err = testcontainers.GenericContainer(context.Background(), testcontainers.GenericContainerRequest{
				ContainerRequest: testcontainers.ContainerRequest{
					FromDockerfile: testcontainers.FromDockerfile{
						Context:    resourcesTestPath,
						Dockerfile: "/integration_test_producer/Dockerfile",
					},
					Env:        map[string]string{"ADDR": redPanda},
					WaitingFor: wait.ForExit().WithExitTimeout(30 * time.Second),
					SkipReaper: skipReaper,
				}, Started: true,
			})
			// extProducer.FollowOutput(Foo{})
			// extProducer.StartLogProducer(context.Background())
			if err != nil {
				t.Errorf("Failed when testcontainers: %v", err)
				t.FailNow()
			}
			if err != nil {
				t.Errorf("Failed when testcontainers: %v", err)
				t.FailNow()
			}
			os.Setenv("LOG_LEVEL", "DEBUG")
			os.Setenv("SERVER_PORT", "19899")
			os.Setenv("AUTHORIZATION_MIDDLEWARE", "noop")
			os.Setenv("BOOTSTRAP_SERVERS", redPanda)

			patchTestConfig(resourcesTestPath, addr)
			os.Setenv("CONFIG_LOCATION", "file:///tmp/test-config-patched.json")

			fxApp = app.Wire()
			err = fxApp.Start(context.Background())
			if err != nil {
				t.Errorf("Failed when running fx: %v", err)
				t.FailNow()
			}
		})
		g.After(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			fxApp.Stop(ctx)
			cancel()
			kafkaContainer.Terminate(ctx)
		})

		g.It("Should start", func() {
			g.Timeout(30 * time.Second)
			read := readTopic(g, "json-consumer", redPanda)
			g.Assert(len(read)).Eql(1100)
		})
		g.It("Should list all configured datasets", func() {
			res, err := http.Get(layerUrl)
			g.Assert(err).IsNil()
			g.Assert(res).IsNotZero()
			bodyBytes, _ := io.ReadAll(res.Body)
			body := string(bodyBytes)
			// t.Log(body)
			g.Assert(strings.Contains(body, "{\"name\":\"json-producer-ds\",\"type\":[\"POST\"]}")).IsTrue()
			g.Assert(strings.Contains(body, "{\"name\":\"json-consumer-ds\",\"type\":[\"GET\"]}")).IsTrue()
			g.Assert(strings.Contains(body, "{\"name\":\"proto-consumer-ds\",\"type\":[\"GET\"]}")).IsTrue()
			g.Assert(strings.Contains(body, "{\"name\":\"avro-consumer-ds\",\"type\":[\"GET\"]}")).IsTrue()
		})
		g.It("Should expose proto dataset", func() {
			g.Timeout(30 * time.Second)
			resp, err := http.Get(layerUrl + "/proto-consumer-ds/entities")
			g.Assert(err).IsNil()
			g.Assert(resp.StatusCode).Eql(200)
			bodyBytes, _ := io.ReadAll(resp.Body)
			// t.Log(string(bodyBytes))
			var entities []map[string]interface{}
			err = json.Unmarshal(bodyBytes, &entities)
			g.Assert(err).IsNil()

			var expected []map[string]interface{}
			json.Unmarshal([]byte(`[
				 {"id":"@context","namespaces":{"ns0":"http://data.example.com/persons/peopleNamespace/","rdf":"http://www.w3.org/1999/02/22-rdf-syntax-ns#"}}
				,{"id":"http://data.example.com/persons/person/test","props":{
					"ns0:address.houseNumber":601,
					"ns0:address.street":"Tøyengata",
					"ns0:age":2,
					"ns0:lastUpdated":"2022-04-27T13:59:01Z",
					"ns0:name":"test",
					"ns0:phones[0].number":"555-100-200",
					"ns0:phones[0].type":"HOME",
					"ns0:phones[1].number":"555-100-300",
					"ns0:phones[1].type":"WORK"},
					"refs":{"ns0:houseNumberRef":"http://data.example.com/houses/601"}}
				,{"id":"@continuation","token":"eyIwIjowfQ=="}
				]`), &expected)
			g.Assert(entities).Eql(expected)
		})
		g.It("Should expose avro dataset", func() {
			g.Timeout(30 * time.Second)
			resp, err := http.Get(layerUrl + "/avro-consumer-ds/entities")
			g.Assert(err).IsNil()
			g.Assert(resp.StatusCode).Eql(200)
			bodyBytes, _ := io.ReadAll(resp.Body)
			// t.Log(string(bodyBytes))
			var entities []map[string]interface{}
			err = json.Unmarshal(bodyBytes, &entities)
			g.Assert(err).IsNil()

			var expected []map[string]interface{}
			json.Unmarshal([]byte(`[{"id":"@context","namespaces":{"ns0":"http://data.example.com/pets/pet/","rdf":"http://www.w3.org/1999/02/22-rdf-syntax-ns#"}}
                ,{"id":"http://data.example.com/pets/animal/Bob","deleted":true,"props":{"ns0:age":3,"ns0:kind":"Cat","ns0:name":"Bob"},"refs":{}}
                ,{"id":"@continuation","token":"eyIwIjowfQ=="}
            ]`), &expected)
			g.Assert(entities).Eql(expected)
		})
		g.It("Should expose complete json dataset", func() {
			g.Timeout(30 * time.Second)
			resp, err := http.Get(layerUrl + "/json-consumer-ds/entities")
			g.Assert(err).IsNil()
			g.Assert(resp.StatusCode).Eql(200)
			bodyBytes, _ := io.ReadAll(resp.Body)
			arr := []map[string]interface{}{}
			err = json.Unmarshal(bodyBytes, &arr)
			g.Assert(err).IsNil()
			//for _, l := range arr {
			//fmt.Println(l)
			//}
			g.Assert(len(arr)).Eql(1102)
			g.Assert(len(bodyBytes)).Eql(200393, "no limit or since parameters should yield all 11000 entities")
		})

		g.It("Should expose json dataset with limit", func() {
			g.Timeout(30 * time.Second)
			resp, err := http.Get(layerUrl + "/json-consumer-ds/entities?limit=3&since=foo")
			g.Assert(err).IsNil()
			g.Assert(resp.StatusCode).Eql(200)
			bodyBytes, _ := io.ReadAll(resp.Body)
			// t.Log(string(bodyBytes))
			var entities []map[string]interface{}
			err = json.Unmarshal(bodyBytes, &entities)
			g.Assert(err).IsNil()

			var expected []map[string]interface{}
			json.Unmarshal([]byte(`[{"id":"@context","namespaces":{"ns0":"http://data.example.com/cities/places/","rdf":"http://www.w3.org/1999/02/22-rdf-syntax-ns#"}}
        ,{"id":"http://data.example.com/cities/city/City-0","refs":{"ns0:postCodeRef":"http://data.example.com/cities/post-code/3000"},"props":{"ns0:name":"City-0","ns0:postCode":3000}}
        ,{"id":"http://data.example.com/cities/city/City-5","refs":{"ns0:postCodeRef":"http://data.example.com/cities/post-code/3005"},"props":{"ns0:name":"City-5","ns0:postCode":3005}}
        ,{"id":"http://data.example.com/cities/city/City-6","refs":{"ns0:postCodeRef":"http://data.example.com/cities/post-code/3006"},"props":{"ns0:name":"City-6","ns0:postCode":3006}}
        ,{"id":"@continuation","token":"eyIwIjoyfQ=="}
        ]`), &expected)
			g.Assert(entities).Eql(expected)
		})
		g.It("Should expose json dataset with since", func() {
			g.Timeout(30 * time.Second)
			resp, err := http.Get(layerUrl + "/json-consumer-ds/entities?since=eyIwIjozMDEsIjEiOjI5NSwiMiI6MjUwLCIzIjoyNDd9")
			// resp, err := http.Get(layerUrl + "/json-consumer-ds/entities?since=eyIzIjoyNTB9")
			// resp, err := http.Get(layerUrl + "/json-consumer-ds/entities?since=eyIwIjozMDEsIjEiOjI5NSwiMiI6MjUwLCIzIjoyNTB9")

			g.Assert(err).IsNil()
			g.Assert(resp.StatusCode).Eql(200)
			bodyBytes, _ := io.ReadAll(resp.Body)
			// t.Log(string(bodyBytes))
			var entities []map[string]interface{}
			err = json.Unmarshal(bodyBytes, &entities)
			g.Assert(err).IsNil()

			var expected []map[string]interface{}
			json.Unmarshal([]byte(`[{"id":"@context","namespaces":{"ns0":"http://data.example.com/cities/places/","rdf":"http://www.w3.org/1999/02/22-rdf-syntax-ns#"}}
        ,{"id":"http://data.example.com/cities/city/City-1091","refs":{"ns0:postCodeRef":"http://data.example.com/cities/post-code/4091"},"props":{"ns0:name":"City-1091","ns0:postCode":4091}}
        ,{"id":"http://data.example.com/cities/city/City-1092","refs":{"ns0:postCodeRef":"http://data.example.com/cities/post-code/4092"},"props":{"ns0:name":"City-1092","ns0:postCode":4092}}
        ,{"id":"http://data.example.com/cities/city/City-1098","refs":{"ns0:postCodeRef":"http://data.example.com/cities/post-code/4098"},"props":{"ns0:name":"City-1098","ns0:postCode":4098}}
        ,{"id":"@continuation","token":"eyIwIjozMDEsIjEiOjI5NSwiMiI6MjUwLCIzIjoyNTB9"}
        ]`), &expected)
			g.Assert(entities).Eql(expected)
			resp, err = http.Get(layerUrl + "/json-consumer-ds/entities?since=eyIwIjozMDEsIjEiOjI5NSwiMiI6MjUwLCIzIjoyNTB9")
			g.Assert(err).IsNil()
			g.Assert(resp.StatusCode).Eql(200)
			bodyBytes, _ = io.ReadAll(resp.Body)
			fmt.Println(string(bodyBytes))
			g.Assert(len(bodyBytes)).Eql(213, "topic should be empty")
		})

		g.It("Should add entity batch to dataset", func() {
			g.Timeout(30 * time.Second)
			input := []*egdm.Entity{
				{
					ID:         "ns0:howdy",
					References: map[string]interface{}{"ns0:type": "ns0:greeting"},
					Properties: map[string]interface{}{"ns0:line": "How are all y'all doing!"},
				},
				{
					ID:         "ns0:hi",
					References: map[string]interface{}{"ns0:type": "ns0:greeting"},
					Properties: map[string]interface{}{"ns0:line": "Hi!"},
				},
				{
					ID:         "ns0:hello",
					References: map[string]interface{}{"ns0:type": "ns0:greeting"},
					Properties: map[string]interface{}{"ns0:line": "Hello there!"},
				},
			}
			b, err := json.Marshal(input)
			g.Assert(err).IsNil()
			ctx, err := json.Marshal(map[string]any{"id": "@context", "namespaces": map[string]interface{}{"ns0": "http://test/"}})
			g.Assert(err).IsNil()
			s := strings.ReplaceAll(string(b), "[", ("[" + string(ctx) + ","))
			b = []byte(s)
			resp, err := http.Post(layerUrl+"/json-producer-ds/entities", "application/json", bytes.NewReader(b))
			g.Assert(err).IsNil()
			g.Assert(resp.StatusCode).Eql(200)
			receivedMessages := readTopic(g, "json-producer", redPanda)
			// for _, m := range receivedMessages { t.Log(string(m.Value)) }
			g.Assert(len(receivedMessages)).Eql(3)
			g.Assert(receivedMessages[0].Value).Eql([]byte(`{"deleted":false,"id":"howdy","props":{"line":"How are all y'all doing!"}}`))
			g.Assert(receivedMessages[1].Value).Eql([]byte(`{"deleted":false,"id":"hi","props":{"line":"Hi!"}}`))
			g.Assert(receivedMessages[2].Value).Eql([]byte(`{"deleted":false,"id":"hello","props":{"line":"Hello there!"}}`))
		})
	})
}

func patchTestConfig(testPath string, addr string) {
	origPath := path.Join(testPath, "/test-config.json")
	input, err := os.ReadFile(origPath)
	if err != nil {
		fmt.Println(err)
	}

	output := bytes.ReplaceAll(input, []byte("RESOURCEPATH"), []byte(testPath))
	output = bytes.ReplaceAll(output, []byte("0.0.0.0"), []byte(addr))

	if err = os.WriteFile("/tmp/test-config-patched.json", output, 0o666); err != nil {
		fmt.Println(err)
	}
}

func readTopic(g *goblin.G, topic string, addr string) []*kafka.Message {
	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": addr,
		"group.id":          "foo",
		"auto.offset.reset": "smallest",
	})
	defer consumer.Close()
	g.Assert(err).IsNil()
	err = consumer.Subscribe(topic, nil)
	g.Assert(err).IsNil()
	run := true
	hasRead := false
	var result []*kafka.Message
	for run == true {
		ev := consumer.Poll(100)
		switch e := ev.(type) {
		case *kafka.Message:
			hasRead = true
			result = append(result, e)
		case kafka.PartitionEOF:
			fmt.Printf("%% Reached %v\n", e)
		case kafka.Error:
			fmt.Fprintf(os.Stderr, "%% Error: %v\n", e)
			// run = false
		default:
			// fmt.Printf("Ignored %v\n", e)
			if hasRead {
				run = false
			}
		}
	}
	return result
}
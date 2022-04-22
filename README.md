# Kafka Data Layer

A Universal Data API (UDA) compliant data layer for Kafka topics. This datalayer can be used to expose an UDA api over kafka topics. It allows for reading and writing of entities to datasets backed with Kafka topics.

## Testing

You can run
```bash
make test
```

## Run

Either do:
```bash
make run
```
or
```bash
make build && bin/server
```

Ensure a config file exists in the location configured in the CONFIG_LOCATION
variable

With Docker

```bash
make docker
docker run -d -p 4646:4646 -v $(pwd)/local.config.json:/root/config.json -e PROFILE=dev -e CONFIG_LOCATION=file://config.json kafka-datalayer
```

## Env

Configuration is done via environment variables. For convenience, .env and .env-[profile] files are supported.

The only mandatory configuration parameter is `CONFIG_LOCATION`.

Every 120s (or otherwise configured) the server will look for updated config's, and load these if it detect changes.

It supports configuration locations that either start with "file://" or "http(s)://".

### Example configuration values
```bash
# the http server port
SERVER_PORT=4646

# how verbose the logger should be
LOG_LEVEL=INFO

# setting up token integration with jwks token provider
TOKEN_WELL_KNOWN=https://token-service/.well-known/jwks.json
TOKEN_AUDIENCE=https://my.audience.domain
TOKEN_ISSUER=https://token-service/

# statsd agent location, if left empty, statsd collection is turned off
DD_AGENT_HOST=

# if config is read from the file system, refer to the file here, for example "file://.config.json",
# if not, point it to an http(s) endpoint. The http endpoint needs to fulfill the api requrements set
# here: https://github.com/mimiro-io/datahub/blob/ffadbc15daf380b28863b8a2f39684abe73d6321/api/datahub.oas3.yml#L632
CONFIG_LOCATION=

# how often should the system look for changes in the configuration. This uses the cron system to
# schedule jobs at the given interval. If omitted, the default is every 120s.
CONFIG_REFRESH_INTERVAL=@every 120s

# to be able to connect to Kafka, you need to give it a set of bootstrap servers.
BOOTSTRAP_SERVERS=localhost:9092 localhost:9093 localhost:9094

```
By default the PROFILE is set to local. This also disables security features, and recommended to override in production.
It should be PROFILE=dev or PROFILE=prod.

Setting a non-locao PROFILE also enables json as log format.

## Configuration

The configuration is divided into Producers (writers to Kafka Topics) and Consumers (readers from Kafka topics).

A Producer dataset accepts entity batches as POST request payload and writes the received entities to the configured kafka topic.

### Producers

```json
"producers": [
    {
        "dataset": "my.topic",
        "topic": "my-topic",
        "createTopic": true,
        "stripProps": true,
        "topicSettings": {
            "partitions": 3,
            "replicas": 3
        }
    }
],
```

Each producer dataset is listed in the /datasets endpoint.

A topic must either exist, or you can tell the datalayer to create it for you
by setting `createTopic=true`. The layer will then use `topicSettings` to set partitions and replicas.
You can also optionally give it a `config` map with raw Kafka configuration options.
If you do not provide a `retention.ms` key, then the retention is set to -1 (forever).

The producer normally will write all received entities unmodified and encoded as json to it's topic.
With `stripProps=true` it will however emit only the Properties part of each entity as `props`,
supplemented with `id` and `deleted`. It will also remove namespace prefixes from keys.

```json
{
    "id": "1",
    "deleted": false,
    "props": [
        "key1": "value1"
    ]
}
```

Producers support additionally setting `key=uuid` (generates a random key)
or `key=id` (uses the Entity.ID) to attach kafka message keys for balancing. If the `key` setting is omitted,
messages are produced without key.
The Message is produced using Murmur2 balancing on the keys, to be compatible with the original
Java producer.

### Consumers

A consumer dataset reads from a topic and returns kafka messages as entities. Consumers are configured in the following way:

```json
"consumers": [
    {
        "dataset": "my.topic",
        "topic": "my-topic",
        "groupId": "my-group2",
        "valueDecoder": "json",
        "position": "earliest",
        "nameSpace": "adomain",
        "baseNameSpace": "http://data.mimiro.io/adomain/",
        "entityIdConstructor": "adomain/%s",
        "types": [
            "http://data.mimiro.io/adomain/person",
            "http://data.mimiro.io/adomain/address"
        ],
        "fieldMappings": [
            {
                "fieldName": "id",
                "path": "id",
                "isIdField": true
            },
            {
                "fieldName": "deleted",
                "path": "deleted",
                "isDeletedField": true
            },
            {
                "fieldName": "FirstName",
                "propertyName": "firstName",
                "path": "props.FirstName"
            },
            {
                "fieldName": "Id",
                "ignoreField": true
            },
            {
                "fieldName": "PostalCode",
                "propertyName": "postalCode",
                "path": "props.PostalCode",
                "isReference": true,
                "referenceTemplate": "http://data.mimiro.io/adomain/address/%.0f"
            }
        ]
    }
]
```

Each consumer dataset should only be used by a single client at a time, and it should have a unique GroupId. Otherwise, data skipping or other unforeseen inconsistencies may occur.

The reason is that topic offsets are not stored in kafka per GroupId, instead the layer resets the datasets configured consumer group to either the beginning - or an offset provided by clients as `since` request parameter.

Position supports `position=earliest` or `position=latest`, to either start consuming of the beginning or of the end of the topic. If you want a full dataset, you need earliest. `baseNameSpace` and `nameSpace` will together form the full namespace for the dataset, while the `baseNameSpace` + `entityIdConstructor` creates the id of the Entity.

`types` is a list of the declared namespaces for this Entity.

### Decoders

The `valueDecoder` configuration option defaults to `json`, but the datalayer also can decode `protobuf` and `avro` message payloads.

```
"valueDecoder": "protobuf",
"protobufSchema": {
    "type": "testdata.Person",
    "path": "RESOURCEPATH/protoschema",
    "fileName": "person.proto"
},
```

Protocol buffer decoding is supported by supplying schema files on disk. Add a root protobuf `type` for messages on the consumed topic, in addition to disk path and name of the schema file that contains the root type definition. The datalayer will decode the protobuf messages into json objects with the same structure, and then apply field mappings.

`avro` decoding works similarly, except for that schemas must be provided through a schema registry service.

```
"valueDecoder": "avro",
"schemaRegistry": {
    "location": "http://0.0.0.0:8081"
},
```

### Field mappings

Each consumer config can take an (optional) list of field mappings.
It is at least suggested that you add an identifier field.

Below is an example with all valid settings, not all can be used together.

```json
{
    "fieldName": "PostalCode",
    "propertyName": "postalCode",
    "path": "props.PostalCode",
    "isIdField": true,
    "isReference": true,
    "isDeletedField": true,
    "ignoreField": true,
    "includeHeaders":true,
    "referenceTemplate": "http://data.mimiro.io/test/post-code/%.0f"
}
```

 - `fieldName` this is the name of the field, and is used as a key for lookup. This must match the field name in the json.
 - `propertyName` if this is set, then the field name will be replaced to this.
 - `path` this is using [gjson syntax](https://github.com/tidwall/gjson#path-syntax) to be able to map out values.
 - `isIdField` if this is true, then the `entityIdConstructor` string interpolation expression will be applied to the field value. There should only be one of these.
 - `isReference` is used together with `referenceTemplate` to produce a reference link.
 - `isDeletedField` is used to set the deleted flag on the Entity. Must resolve to a bool.
 - `ignoreField` tells the consumer to ignore the field, that is remove it from the Entity.
 - `referenceTemplate` is used to generate reference links, only useful if `isReference` is true.
 - `includeHeaders` is used to add kafka headers to the entity output.

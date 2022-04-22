(async () => {
    const {Kafka} = require('kafkajs')
    const addr = process.env.ADDR || "redpanda:29092"
    const regAddr = addr.split(":")[0]+":8081"
    console.log("addr",addr,"regAddr",regAddr)
    const producer = new Kafka({brokers: [addr]}).producer()
    await producer.connect()

    // emit protobuf sample
    await new Promise(resolve => {
        require('protobufjs').load('protoschema/person.proto', async (err, root) => {
            const Person = root.lookupType('testdata.Person')
            const dt = Date.parse("2022-04-27T12:59:01-01:00")
            const secs = Math.floor(dt / 1000);
            const jsObj = {
                name: "test",
                age: 2,
                phones: [
                    {number: "555-100-200", type: 1},
                    {number: "555-100-300", type: 2}
                ],
                address: {
                    street: "Tøyengata",
                    houseNumber: 601
                },
                lastUpdated: {seconds: secs}
            };
            const message = Person.create(jsObj);
            const sendResult = await producer.send({
                topic: 'proto-consumer', messages: [{key: 'proto1', value: Person.encode(message).finish()}]
            })
            console.log('sending protobuf ', message, '. result OK? ', sendResult[0].errorCode === 0)
            resolve("done")
        })
    })

    // register pet schema in registry
    const { SchemaRegistry, SchemaType, readAVSCAsync} = require('@kafkajs/confluent-schema-registry')
    const registry = new SchemaRegistry({ host: 'http://'+regAddr })
    //const registry = new SchemaRegistry({ host: 'http://redpanda:8081' })
    const schema = await readAVSCAsync('avroschema/Pet.avsc')
    const {id} = await registry.register({ type: SchemaType.AVRO, schema: JSON.stringify(schema) })
    // emit avro sample
    const pet = {name: 'Bob', kind: 'Cat', age: 3, dead: true}
    const sendResult = await producer.send({
        topic: 'avro-consumer', messages: [{
            key: 'avro1', value: await registry.encode(id,pet)
        }]
    })
    console.log('sending avro ', pet, '. result OK? ', sendResult[0].errorCode === 0)



    // emit 11000 json samples
    for (let i = 0; i < 1100; i++) {
        const city = {name: 'City-' + i, population: i * 1000, postCode: 3000 + i}
        const sendResult = await producer.send({
            topic: 'json-consumer', messages: [{
                key: 'json/' + i, value: JSON.stringify(city)
            }]
        })
        console.log('sending json ', city, '. result OK? ', sendResult[0].errorCode === 0)

    }
    await producer.disconnect()
})()

fs = require('fs');
require('protobufjs').load('../protoschema/person.proto', async (err, root) => {
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
    //console.log(  Person.verify(jsObj))
    const buf = Person.encode(message).finish()

    fs.writeFileSync("../protobuf-wire-person", buf);
})

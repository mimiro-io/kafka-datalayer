FROM node:16-alpine

# Create app directory
WORKDIR /usr/src/app

# Install app dependencies
# A wildcard is used to ensure both package.json AND package-lock.json are copied
# where available (npm@5+)
COPY integration_test_producer/package*.json ./

RUN npm install
# If you are building your code for production
# RUN npm ci --only=production

# Bundle app source
COPY integration_test_producer/producer.js .
COPY protoschema/*.proto ./protoschema/
COPY avroschema/*.avsc ./avroschema/

EXPOSE 8080
CMD [ "node", "producer.js" ]

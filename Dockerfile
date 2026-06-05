# Minify client side assets (JavaScript)
FROM node:24 AS build-js

RUN npm install gulp gulp-cli -g

WORKDIR /build
COPY . .
RUN npm ci
RUN npx gulp build


# Build Golang binary
FROM golang:1.26.4 AS build-golang

WORKDIR /go/src/github.com/AegisAX/AegisCampus
COPY . .
RUN go get -v && go build -v -o aegiscampus


# Runtime container
FROM debian:stable-slim

RUN useradd -m -d /opt/aegiscampus -s /bin/bash app

RUN apt-get update && \
	apt-get install --no-install-recommends -y jq libcap2-bin ca-certificates && \
	apt-get clean && \
	rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

WORKDIR /opt/aegiscampus
COPY --from=build-golang /go/src/github.com/AegisAX/AegisCampus/ ./
COPY --from=build-js /build/static/js/dist/ ./static/js/dist/
COPY --from=build-js /build/static/css/dist/ ./static/css/dist/
COPY --from=build-golang /go/src/github.com/AegisAX/AegisCampus/config.json ./
RUN chown app. config.json

RUN setcap 'cap_net_bind_service=+ep' /opt/aegiscampus/aegiscampus

USER app
RUN sed -i 's/127.0.0.1/0.0.0.0/g' config.json
RUN touch config.json.tmp

EXPOSE 3333 8088

CMD ["./docker/run.sh"]

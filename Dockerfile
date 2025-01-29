FROM golang:alpine3.21

RUN apk add --no-cache iproute2-tc

WORKDIR /usr/src/goapp/
USER root
ADD ./go.mod /usr/src/goapp/

ENV GO111MODULE="on" CGO_ENABLED="0" GO_GC="off"
RUN go mod download

ADD *.go /usr/src/goapp/
RUN go mod tidy && go mod verify && go build -o main .

COPY entry.sh /usr/src/goapp/
RUN chmod +x entry.sh

EXPOSE 8080
ENTRYPOINT [ "./entry.sh" ]
FROM golang:alpine3.21

WORKDIR /usr/src/goapp/
USER root
ADD ./go.mod /usr/src/goapp/

ENV GO111MODULE="on" CGO_ENABLED="0" GO_GC="off"
RUN go mod download

ADD *.go /usr/src/goapp/
RUN go mod tidy && go mod verify && go build -o main .

EXPOSE 8080
ENTRYPOINT [ "./main" ]
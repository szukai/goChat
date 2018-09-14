FROM golang:1.8

WORKDIR /go/src/app
ADD . /go/src/app

CMD ["go", "run", "gochat.go"]
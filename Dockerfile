FROM golang:1.8

WORKDIR /go/src/app
ADD . /go/src/app

EXPOSE 6000

CMD ["go", "run", "gochat.go"]
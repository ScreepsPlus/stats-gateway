FROM golang:1.9

WORKDIR /go/src/app
COPY . .
CMD ["go","run","main.go"]
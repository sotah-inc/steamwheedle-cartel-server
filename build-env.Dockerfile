# building
FROM golang:1.11-alpine

# installing deps
RUN apk update \
  && apk upgrade \
  && apk add --no-cache bash git openssh

# copying in source
COPY ./app /srv/app
WORKDIR /srv/app

# building the project
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -mod=vendor -a -installsuffix cgo -o /go/bin/app github.com/sotah-inc/steamwheedle-cartel-server/app

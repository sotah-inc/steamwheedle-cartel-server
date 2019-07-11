# building
FROM golang:1.11-alpine

# installing deps
RUN apk update \
  && apk upgrade \
  && apk add --no-cache bash git openssh

# misc
ENV APP_PROJECT github.com/sotah-inc/steamwheedle-cartel-server/app

# working dir
WORKDIR /srv/app

# copying in source
COPY ./app /srv/app

# building the project
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -mod=vendor -a -installsuffix cgo -o /go/bin/app .

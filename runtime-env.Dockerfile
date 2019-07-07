# running
FROM alpine as runtime-env

# installing deps
RUN apk update \
  && apk upgrade \
  && apk add --no-cache curl ca-certificates \
  && update-ca-certificates

# runtime dir
WORKDIR /srv/app

# copying in built app
COPY --from=ihsw/sotah-server/build /go/bin/app /go/bin/app

ENTRYPOINT ["/go/bin/app"]

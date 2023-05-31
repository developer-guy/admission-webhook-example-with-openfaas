FROM alpine:3

ADD admission-webhook-example /admission-webhook-example
ENTRYPOINT ["./admission-webhook-example"]
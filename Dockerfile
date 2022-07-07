FROM alpine:3.16

ADD admission-webhook-example /admission-webhook-example
ENTRYPOINT ["./admission-webhook-example"]
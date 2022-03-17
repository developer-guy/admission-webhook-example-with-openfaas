FROM alpine:3.15

ADD admission-webhook-example /admission-webhook-example
ENTRYPOINT ["./admission-webhook-example"]
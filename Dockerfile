# build stage
FROM golang:1.16.2-alpine

COPY . /go/src/github.com/KKEU/aws-autoscaling-exporter
WORKDIR /go/src/github.com/KKEU/aws-autoscaling-exporter
RUN go build -o /bin/aws-autoscaling-exporter .

FROM alpine:latest
RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*
COPY --from=0 /bin/aws-autoscaling-exporter /bin
ENTRYPOINT ["/bin/aws-autoscaling-exporter"]

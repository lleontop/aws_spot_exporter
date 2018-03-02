FROM golang:1.9.2
WORKDIR /go/src/github.com/lleontop/aws_audit_exporter/
COPY . .
RUN go get -d -v ./...
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o aws_audit_exporter .
#RUN go build -o aws_audit_exporter .

FROM alpine:latest
RUN apk add --no-cache ca-certificates && update-ca-certificates
WORKDIR /root/
COPY --from=0 /go/src/github.com/lleontop/aws_audit_exporter/aws_audit_exporter .
CMD ["./aws_audit_exporter"]
EXPOSE 9190

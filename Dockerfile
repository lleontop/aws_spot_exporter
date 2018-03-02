FROM golang:1.9.2
WORKDIR /go/src/github.com/lleontop/aws_spot_exporter/
COPY . .
RUN go get -d -v ./...
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o aws_spot_exporter .
#RUN go build -o aws_spot_exporter .

FROM alpine:latest
RUN apk add --no-cache ca-certificates && update-ca-certificates
WORKDIR /root/
COPY --from=0 /go/src/github.com/lleontop/aws_spot_exporter/aws_spot_exporter .
CMD ["./aws_spot_exporter"]
EXPOSE 9190

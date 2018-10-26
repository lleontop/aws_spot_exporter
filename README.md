# aws_spot_exporter

Exports aws spot market prices for all regions in a Prometheus compatible format.

### Docker
Generated images are stored here: https://hub.docker.com/r/lleontop/aws_spot_exporter/

### Running exporter locally

Start the latest image with `docker run -p9190:9190 lleontop/aws_spot_exporter:latest`

Metrics should be available at `127.0.0.1:9190/metrics`

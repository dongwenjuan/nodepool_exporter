FROM  quay.io/prometheus/busybox:latest
LABEL maintainer="The Prometheus Authors <dong.wenjuan@zte.com.cn>"

COPY nodepool_exporter /bin/nodepool_exporter

EXPOSE      9533 9533 9533/udp
ENTRYPOINT  [ "/bin/nodepool_exporter" ]

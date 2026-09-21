FROM otel/opentelemetry-collector-contrib:0.139.0

COPY config.yaml /etc/otelcol-contrib/config.yaml

ENTRYPOINT ["/otelcol-contrib"]

CMD ["--config=/etc/otelcol-contrib/config.yaml"]
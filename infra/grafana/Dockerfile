FROM grafana/grafana:12.2.0

USER root

RUN mkdir -p /etc/grafana/dashboards

COPY provisioning /etc/grafana/provisioning
COPY dashboards /etc/grafana/dashboards
FROM amazon/aws-cli:2.36.49

COPY init-queues.sh /scripts/init-queues.sh

ENTRYPOINT ["/bin/sh", "/scripts/init-queues.sh"]
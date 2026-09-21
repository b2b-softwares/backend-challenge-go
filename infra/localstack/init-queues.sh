#!/bin/bash

set -e

echo "============================================================"
echo "Initializing SQS queues"
echo "============================================================"

REGION="${AWS_DEFAULT_REGION:-us-east-1}"
ENDPOINT_URL="${AWS_ENDPOINT_URL:-http://ministack:4566}"

AWS_CLI="aws --endpoint-url ${ENDPOINT_URL}"

echo "AWS region: ${REGION}"
echo "AWS endpoint: ${ENDPOINT_URL}"

echo "Waiting for SQS endpoint..."

until ${AWS_CLI} sqs list-queues --region "$REGION" >/dev/null 2>&1; do
  echo "SQS is not ready yet. Retrying..."
  sleep 2
done

echo "SQS endpoint is ready."

echo "Creating DLQ..."

${AWS_CLI} sqs create-queue \
  --queue-name wager-transactions-dlq.fifo \
  --region "$REGION" \
  --attributes '{
    "FifoQueue":"true",
    "ContentBasedDeduplication":"false"
  }' \
  >/dev/null

DLQ_URL="$(
  ${AWS_CLI} sqs get-queue-url \
    --queue-name wager-transactions-dlq.fifo \
    --region "$REGION" \
    --query 'QueueUrl' \
    --output text
)"

DLQ_ARN="$(
  ${AWS_CLI} sqs get-queue-attributes \
    --queue-url "$DLQ_URL" \
    --region "$REGION" \
    --attribute-names QueueArn \
    --query 'Attributes.QueueArn' \
    --output text
)"

echo "DLQ URL: ${DLQ_URL}"
echo "DLQ ARN: ${DLQ_ARN}"

echo "Creating main queue..."

${AWS_CLI} sqs create-queue \
  --queue-name wager-transactions.fifo \
  --region "$REGION" \
  --attributes "{
    \"FifoQueue\":\"true\",
    \"ContentBasedDeduplication\":\"false\",
    \"RedrivePolicy\":\"{\\\"deadLetterTargetArn\\\":\\\"${DLQ_ARN}\\\",\\\"maxReceiveCount\\\":\\\"5\\\"}\"
  }" \
  >/dev/null

QUEUE_URL="$(
  ${AWS_CLI} sqs get-queue-url \
    --queue-name wager-transactions.fifo \
    --region "$REGION" \
    --query 'QueueUrl' \
    --output text
)"

echo "Main queue URL: ${QUEUE_URL}"

echo "Creating events queue..."

${AWS_CLI} sqs create-queue \
  --queue-name wager-events.fifo \
  --region "$REGION" \
  --attributes '{
    "FifoQueue":"true",
    "ContentBasedDeduplication":"false"
  }' \
  >/dev/null

EVENTS_QUEUE_URL="$(
  ${AWS_CLI} sqs get-queue-url \
    --queue-name wager-events.fifo \
    --region "$REGION" \
    --query 'QueueUrl' \
    --output text
)"

echo "Events queue URL: ${EVENTS_QUEUE_URL}"

echo
echo "============================================================"
echo "SQS queues"
echo "============================================================"

${AWS_CLI} sqs list-queues \
  --region "$REGION"

echo
echo "SQS initialization completed."
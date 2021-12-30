## Autoscaler for SQS based worker application

This example show how to deploy autoscaler for worker with backend on AWS Simple Query Service.

**Note**: it uses simple Ubuntu image as worker, it won't process queue, to simulate "work" you should manually purge/seed that queue using AWS CLI.

- [Autoscaler for SQS based worker application](#autoscaler-for-sqs-based-worker-application)
  - [Prerequsities](#prerequsities)
  - [Setup](#setup)
  - [Useful command to play with](#useful-command-to-play-with)


### Prerequsities

1. AWS account to play with
2. AWS CLI version 2 installed locally
3. `jq`, available here: https://stedolan.github.io/jq/
4. K8S cluster


### Setup

```bash
# Create autoscaler iam user
aws iam create-user --user-name autoscaler

# Create SQS queue
QUEUE_URL=$(aws sqs create-queue --queue-name autoscaler-example-queue | jq -r .QueueUrl)

# Create IAM policy
QUEUE_ARN=$(aws sqs get-queue-attributes --queue-url "$QUEUE_URL" --attribute-names QueueArn | jq -r .Attributes.QueueArn)
POLICY_ARN=$(aws iam create-policy --policy-name autoscaler-policy --policy-document "{
    \"Version\": \"2012-10-17\",
    \"Statement\": [
        {
            \"Sid\": \"GetQueue\",
            \"Effect\": \"Allow\",
            \"Action\": [
                \"sqs:GetQueueUrl\",
                \"sqs:GetQueueAttributes\"
            ],
            \"Resource\": [\"$QUEUE_ARN\"]
        }
    ]
}" | jq -r .Policy.Arn)

# Attach IAM policy
aws iam attach-user-policy --user-name autoscaler --policy-arn $POLICY_ARN

# Generate Access keys for Autoscaler user
ACCESS_KEYS=$(aws iam create-access-key --user-name autoscaler | jq -r .AccessKey)

# Create testing namespace in K8S
kubectl create namespace autoscaler-example

# Create IAM keys in K8S cluster
kubectl create secret generic autoscaler-example-credentials \
  --from-literal=AWS_ACCESS_KEY_ID="$(echo $ACCESS_KEYS | jq -r .AccessKeyId)" \
  --from-literal=AWS_SECRET_ACCESS_KEY="$(echo $ACCESS_KEYS | jq -r .SecretAccessKey)"

# Deploy payloads to kubernetes
kubectl apply -f ./k8s_files
```

### Useful command to play with

```bash
QUEUE_URL=$(aws sqs get-queue-url --queue-name autoscaler-example-queue | jq -r .QueueUrl)

# Send message to queue
aws sqs send-message --queue-url $QUEUE_URL --message-body "Example message"

# Send multiple messages to queue
for i in $(seq 1 30); do
  echo $i
  aws sqs send-message --queue-url $QUEUE_URL --message-body "Message no. $i" > /dev/null
done

# Purge queue
aws sqs purge-queue --queue-url $QUEUE_URL
```

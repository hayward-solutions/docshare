# AWS CloudFormation Deployment

Deploy DocShare to AWS using ECS Fargate, Application Load Balancer, RDS PostgreSQL, and S3.

## Prerequisites

- AWS account with appropriate permissions
- Route53 hosted zone for your domain
- VPC with public and private subnets
- AWS CLI configured (`aws configure`)

## Quick Start

```bash
# Generate JWT secret
JWT_SECRET=$(openssl rand -hex 32)

# Deploy stack
aws cloudformation create-stack \
  --stack-name docshare \
  --template-body file://docshare.yaml \
  --parameters \
      ParameterKey=DomainName,ParameterValue=docshare.example.com \
      ParameterKey=Route53HostedZoneId,ParameterValue=Z1234567890ABC \
      ParameterKey=VpcId,ParameterValue=vpc-abc123 \
      ParameterKey=PublicSubnetIds,ParameterValue=\"subnet-public-1,subnet-public-2\" \
      ParameterKey=PrivateSubnetIds,ParameterValue=\"subnet-private-1,subnet-private-2\" \
      ParameterKey=JwtSecret,ParameterValue=$JWT_SECRET \
  --capabilities CAPABILITY_IAM \
  --region us-east-1

# Wait for completion (10-15 minutes)
aws cloudformation wait stack-create-complete --stack-name docshare

# Get outputs
aws cloudformation describe-stacks --stack-name docshare --query 'Stacks[0].Outputs'
```

## Parameters

| Parameter             | Required | Default        | Description                          |
|-----------------------|----------|----------------|--------------------------------------|
| `DomainName`          | Yes      | -              | Domain for the application           |
| `Route53HostedZoneId` | Yes      | -              | Route53 hosted zone ID               |
| `VpcId`               | Yes      | -              | VPC ID                               |
| `PublicSubnetIds`     | Yes      | -              | Public subnet IDs (comma-separated)  |
| `PrivateSubnetIds`    | Yes      | -              | Private subnet IDs (comma-separated) |
| `DbInstanceClass`     | No       | `db.t4g.micro` | RDS instance type                    |
| `DbUsername`          | No       | `docshare`     | Database username                    |
| `JwtSecret`           | Yes      | -              | JWT signing secret (32+ chars)       |
| `S3BucketName`        | No       | auto-generated | S3 bucket name                       |
| `BackendCpu`          | No       | 256            | Backend CPU (256 = 0.25 vCPU)        |
| `BackendMemory`       | No       | 512            | Backend memory (MB)                  |
| `FrontendCpu`         | No       | 256            | Frontend CPU                         |
| `FrontendMemory`      | No       | 512            | Frontend memory (MB)                 |
| `GotenbergCpu`        | No       | 512            | Gotenberg CPU                        |
| `GotenbergMemory`     | No       | 1024           | Gotenberg memory (MB)                |
| `Environment`         | No       | `production`   | Environment tag                      |

## Architecture

```
                       ┌─────────────────────┐
                       │     Route53 DNS     │
                       └──────────┬──────────┘
                                  │
                       ┌──────────▼──────────┐
                       │   ACM Certificate   │
                       └──────────┬──────────┘
                                  │
┌────────────────────────────────────────────────────────────────────┐
│                                VPC                                 │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                        Public Subnets                        │  │
│  │   ┌─────────────────────────────────────────────────────┐    │  │
│  │   │               Application Load Balancer             │    │  │
│  │   │            (HTTPS → HTTP, Path Routing)             │    │  │
│  │   └──────────────────────────┬──────────────────────────┘    │  │
│  └──────────────────────────────┼───────────────────────────────┘  │
│                                 │                                  │
│  ┌──────────────────────────────▼───────────────────────────────┐  │
│  │                       Private Subnets                        │  │
│  │                                                              │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐   │  │
│  │  │  Frontend   │  │   Backend   │  │      Gotenberg      │   │  │
│  │  │  (Fargate)  │  │  (Fargate)  │  │      (Fargate)      │   │  │
│  │  │   :3000     │  │   :8080     │  │       :3000         │   │  │
│  │  └─────────────┘  └──────┬──────┘  └─────────────────────┘   │  │
│  │                          │                                   │  │
│  │                          │    ┌─────────────────────────┐    │  │
│  │                          └────┤     RDS PostgreSQL      │    │  │
│  │                               │     (Encrypted)         │    │  │
│  │                               └─────────────────────────┘    │  │
│  └──────────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────────┘
                                   │
                         ┌─────────▼──────────┐
                         │      S3 Bucket     │
                         │   (KMS Encrypted)  │
                         └────────────────────┘
```

## Resources Created

| Resource | Type | Description |
|----------|------|-------------|
| Security Groups | AWS::EC2::SecurityGroup | 5 security groups (ALB, Backend, Frontend, Gotenberg, RDS) |
| KMS Key | AWS::KMS::Key | Customer-managed key for S3 encryption |
| S3 Bucket | AWS::S3::Bucket | Encrypted file storage with versioning |
| RDS PostgreSQL | AWS::RDS::DBInstance | PostgreSQL 16, encrypted at rest |
| Application Load Balancer | AWS::ElasticLoadBalancingV2::LoadBalancer | Public ALB with HTTPS |
| ACM Certificate | AWS::CertificateManager::Certificate | SSL/TLS certificate with DNS validation |
| ECS Cluster | AWS::ECS::Cluster | Fargate cluster with Container Insights |
| ECS Services | AWS::ECS::Service | 3 services (web, api, gotenberg) |
| Auto Scaling | AWS::ApplicationAutoScaling::ScalableTarget | CPU-based scaling policies |
| Route53 Record | AWS::Route53::RecordSetGroup | Alias record to ALB |

## Scaling Configuration

| Service   | Min | Max | Scale Trigger |
|-----------|-----|-----|---------------|
| API       | 1   | 4   | CPU > 70%     |
| Web       | 1   | 4   | CPU > 70%     |
| Gotenberg | 1   | 2   | CPU > 80%     |

## Updating

```bash
# Update stack with new parameters
aws cloudformation update-stack \
  --stack-name docshare \
  --template-body file://docshare.yaml \
  --parameters \
      ParameterKey=DomainName,ParameterValue=docshare.example.com \
      ParameterKey=Route53HostedZoneId,ParameterValue=Z1234567890ABC \
      ParameterKey=VpcId,ParameterValue=vpc-abc123 \
      ParameterKey=PublicSubnetIds,ParameterValue=\"subnet-public-1,subnet-public-2\" \
      ParameterKey=PrivateSubnetIds,ParameterValue=\"subnet-private-1,subnet-private-2\" \
      ParameterKey=JwtSecret,ParameterValue=$JWT_SECRET \
      ParameterKey=BackendCpu,ParameterValue=512 \
      ParameterKey=BackendMemory,ParameterValue=1024 \
  --capabilities CAPABILITY_IAM
```

## Deleting

```bash
# Delete stack (retains RDS snapshot)
aws cloudformation delete-stack --stack-name docshare

# Optionally delete the RDS final snapshot manually via AWS Console
```

## Estimated Costs (us-east-1)

| Resource      | Instance/Config           | Monthly Cost        |
|---------------|---------------------------|---------------------|
| ALB           | Application Load Balancer | ~$20                |
| NAT Gateway   | 1x (if needed)            | ~$32                |
| RDS           | db.t4g.micro              | ~$15                |
| ECS Backend   | 0.25 vCPU, 512MB          | ~$15                |
| ECS Frontend  | 0.25 vCPU, 512MB          | ~$15                |
| ECS Gotenberg | 0.5 vCPU, 1GB             | ~$25                |
| S3            | Storage + Requests        | Variable            |
| **Total**     |                           | **~$120-150/month** |

## Troubleshooting

### Certificate Validation Timeout

ACM certificates require DNS validation. The template creates the CNAME record automatically, but propagation can take 5-30 minutes. Check status:

```bash
aws acm describe-certificate --certificate-arn <arn> --query 'Certificate.Status'
```

### ECS Tasks Not Starting

Check CloudWatch logs and task events:

```bash
aws ecs describe-services --cluster docshare-cluster --services api --query 'services[0].events[0:5]'
```

### Database Connection Issues

Verify security group rules:

```bash
aws ec2 describe-security-groups --group-ids <security-group-id>
```

## See Also

- [Detailed Documentation](../../docs/CLOUDFORMATION.md)
- [Deployment Policy](./deploy-policy.json) - Minimum IAM permissions required
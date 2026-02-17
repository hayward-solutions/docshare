# AWS CloudFormation Deployment

Deploy DocShare to AWS using CloudFormation with ECS Fargate, Application Load Balancer, RDS PostgreSQL, and S3 with KMS encryption.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Prerequisites](#prerequisites)
3. [Quick Start](#quick-start)
4. [Parameters Reference](#parameters-reference)
5. [Resources Created](#resources-created)
6. [Security Configuration](#security-configuration)
7. [Auto Scaling](#auto-scaling)
8. [Cost Estimation](#cost-estimation)
9. [Deployment Walkthrough](#deployment-walkthrough)
10. [Updating the Stack](#updating-the-stack)
11. [Deleting the Stack](#deleting-the-stack)
12. [Troubleshooting](#troubleshooting)
13. [Advanced Configuration](#advanced-configuration)

---

## Architecture Overview

```
                                   Internet
                                      │
                                      │ HTTPS (443)
                                      │
                         ┌────────────▼────────────┐
                         │      Route53 DNS        │
                         │   (Alias → ALB)         │
                         └────────────┬────────────┘
                                      │
                         ┌────────────▼────────────┐
                         │    ACM Certificate      │
                         │   (DNS Validation)      │
                         └────────────┬────────────┘
                                      │
┌─────────────────────────────────────────────────────────────────────────────┐
│                                    VPC                                      │
│                                                                             │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │                         Public Subnets                                 │ │
│  │  ┌─────────────────────────────────────────────────────────────────┐   │ │
│  │  │              Application Load Balancer                          │   │ │
│  │  │                                                                 │   │ │
│  │  │   • HTTP → HTTPS Redirect                                       │   │ │
│  │  │   • Path Routing:                                               │   │ │
│  │  │     - /api/* → Backend Target Group                             │   │ │
│  │  │     - /*     → Frontend Target Group                            │   │ │
│  │  │   • TLS 1.2/1.3 Only                                            │   │ │
│  │  └──────────────────────────────┬──────────────────────────────────┘   │ │
│  └─────────────────────────────────┼──────────────────────────────────────┘ │
│                                    │                                        │
│  ┌─────────────────────────────────▼──────────────────────────────────────┐ │
│  │                         Private Subnets                                │ │
│  │                                                                        │ │
│  │  ┌──────────────────┐  ┌──────────────────┐  ┌───────────────────────┐ │ │
│  │  │    Frontend      │  │     Backend      │  │      Gotenberg        │ │ │
│  │  │    (Fargate)     │  │    (Fargate)     │  │      (Fargate)        │ │ │
│  │  │                  │  │                  │  │                       │ │ │
│  │  │  • Next.js 16    │  │  • Go Fiber API  │  │  • Document Convert   │ │ │
│  │  │  • Port 3000     │  │  • Port 8080     │  │  • Port 3000          │ │ │
│  │  │  • 0.25 vCPU     │  │  • 0.25 vCPU     │  │  • 0.5 vCPU           │ │ │
│  │  │  • 512 MB        │  │  • 512 MB        │  │  • 1024 MB            │ │ │
│  │  └──────────────────┘  └────────┬─────────┘  └───────────────────────┘ │ │
│  │                                 │                           │          │ │
│  │                                 │ Service Connect           │          │ │
│  │                                 │ (internal DNS)            │          │ │
│  │                                 ▼                           │          │ │
│  │                    ┌────────────────────────┐               │          │ │
│  │                    │   RDS PostgreSQL 16    │               │          │ │
│  │                    │                        │               │          │ │
│  │                    │  • Encrypted at rest   │               │          │ │
│  │                    │  • db.t4g.micro        │               │          │ │
│  │                    │  • Port 5432           │               │          │ │
│  │                    └────────────────────────┘               │          │ │
│  └─────────────────────────────────────────────────────────────┼──────────┘ │
│                                                                │            │
│                                ┌───────────────────────────────┘            │
│                                │                                            │
│                    ┌───────────▼────────────┐                               │
│                    │      S3 Bucket         │                               │
│                    │                        │                               │
│                    │  • KMS Encrypted       │                               │
│                    │  • Versioning Enabled  │                               │
│                    │  • Public Access Block │                               │
│                    └────────────────────────┘                               │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Network Flow

1. **DNS Resolution**: Route53 resolves domain to ALB DNS name
2. **TLS Termination**: ALB terminates HTTPS, forwards HTTP to containers
3. **Path Routing**: 
   - `/api/*` requests → Backend (8080)
   - All other requests → Frontend (3000)
4. **Backend Dependencies**:
   - RDS PostgreSQL (5432) - User data, files metadata
   - S3 Bucket - File storage
   - Gotenberg (3000) - Document conversion via Service Connect

### Security Boundaries

```
┌─────────────────────────────────────────────────────────────┐
│                    Security Groups                          │
│                                                             │
│  AlbSecurityGroup                                           │
│  ├── Ingress: 80, 443 from 0.0.0.0/0                        │
│  └── Egress: 3000, 8080 to Backend/Frontend SG              │
│                                                             │
│  FrontendSecurityGroup                                      │
│  ├── Ingress: 3000 from AlbSecurityGroup                    │
│  └── Egress: None (no outbound needed)                      │
│                                                             │
│  BackendSecurityGroup                                       │
│  ├── Ingress: 8080 from AlbSecurityGroup                    │
│  └── Egress: 3000 to GotenbergSG, 5432 to DatabaseSG        │
│                                                             │
│  GotenbergSecurityGroup                                     │
│  ├── Ingress: 3000 from BackendSecurityGroup                │
│  └── Egress: None                                           │
│                                                             │
│  DatabaseSecurityGroup                                      │
│  ├── Ingress: 5432 from BackendSecurityGroup                │
│  └── Egress: None                                           │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Prerequisites

### Required

1. **AWS Account** with appropriate permissions (see [deploy-policy.json](../examples/cloudformation/deploy-policy.json))

2. **Route53 Hosted Zone**
   ```bash
   # List your hosted zones
   aws route53 list-hosted-zones --query 'HostedZones[*].{Id:Id,Name:Name}'
   ```

3. **VPC with Subnets**
   - 2+ public subnets (for ALB)
   - 2+ private subnets (for ECS and RDS)
   - Private subnets must have NAT Gateway for container image pulls

4. **AWS CLI** configured
   ```bash
   aws configure
   ```

### Recommended

- AWS CloudFormation service role for deployments
- NAT Gateway in public subnets for private subnet internet access

---

## Quick Start

```bash
# 1. Generate secrets
JWT_SECRET=$(openssl rand -hex 32)
echo "JWT_SECRET=$JWT_SECRET"

# 2. Get your VPC and subnet IDs
aws ec2 describe-vpcs --filters Name=isDefault,Values=true --query 'Vpcs[0].VpcId' --output text

# 3. Deploy the stack
aws cloudformation create-stack \
  --stack-name docshare \
  --template-body file://examples/cloudformation/docshare.yaml \
  --parameters \
      ParameterKey=DomainName,ParameterValue=docshare.example.com \
      ParameterKey=Route53HostedZoneId,ParameterValue=Z1234567890ABC \
      ParameterKey=VpcId,ParameterValue=vpc-abc123 \
      ParameterKey=PublicSubnetIds,ParameterValue="subnet-aaa,subnet-bbb" \
      ParameterKey=PrivateSubnetIds,ParameterValue="subnet-xxx,subnet-yyy" \
      ParameterKey=JwtSecret,ParameterValue=$JWT_SECRET \
  --capabilities CAPABILITY_IAM \
  --region us-east-1

# 4. Wait for completion (10-15 minutes)
aws cloudformation wait stack-create-complete --stack-name docshare --region us-east-1

# 5. Get the application URL
aws cloudformation describe-stacks \
  --stack-name docshare \
  --query 'Stacks[0].Outputs[?OutputKey==`AlbUrl`].OutputValue' \
  --output text
```

---

## Parameters Reference

### Required Parameters

| Parameter             | Type                         | Description                                                    |
|-----------------------|------------------------------|----------------------------------------------------------------|
| `DomainName`          | String                       | Domain name for the application (e.g., `docshare.example.com`) |
| `Route53HostedZoneId` | AWS::Route53::HostedZone::Id | Route53 hosted zone ID for the domain                          |
| `VpcId`               | AWS::EC2::VPC::Id            | VPC ID for deployment                                          |
| `PublicSubnetIds`     | List<AWS::EC2::Subnet::Id>   | Comma-separated public subnet IDs (2+ required for ALB)        |
| `PrivateSubnetIds`    | List<AWS::EC2::Subnet::Id>   | Comma-separated private subnet IDs (2+ recommended for RDS)    |
| `JwtSecret`           | String                       | JWT signing secret (32+ characters recommended)                |

### Optional Parameters

| Parameter           | Type   | Default           | Description                              |
|---------------------|--------|-------------------|------------------------------------------|
| `DbInstanceClass`   | String | `db.t4g.micro`    | RDS PostgreSQL instance class            |
| `DbUsername`        | String | `docshare`        | PostgreSQL master username               |
| `S3BucketName`      | String | `{Stack}-storage` | S3 bucket name (auto-generated if empty) |
| `BackendCpu`        | Number | 256               | Backend task CPU units (256 = 0.25 vCPU) |
| `BackendMemory`     | Number | 512               | Backend task memory in MB                |
| `BackendMinCount`   | Number | 1                 | Minimum api tasks                    |
| `BackendMaxCount`   | Number | 4                 | Maximum api tasks                    |
| `FrontendCpu`       | Number | 256               | Frontend task CPU units                  |
| `FrontendMemory`    | Number | 512               | Frontend task memory in MB               |
| `FrontendMinCount`  | Number | 1                 | Minimum web tasks                   |
| `FrontendMaxCount`  | Number | 4                 | Maximum web tasks                   |
| `GotenbergCpu`      | Number | 512               | Gotenberg task CPU units                 |
| `GotenbergMemory`   | Number | 1024              | Gotenberg task memory in MB              |
| `GotenbergMinCount` | Number | 1                 | Minimum Gotenberg tasks                  |
| `GotenbergMaxCount` | Number | 2                 | Maximum Gotenberg tasks                  |
| `Environment`       | String | `production`      | Environment tag                          |

### CPU and Memory Options (Fargate)

| CPU (vCPU) | Memory Range                    |
|------------|---------------------------------|
| 256 (0.25) | 512, 1024, 2048                 |
| 512 (0.5)  | 1024-4096 (in 1024 increments)  |
| 1024 (1)   | 2048-8192 (in 1024 increments)  |
| 2048 (2)   | 4096-16384 (in 1024 increments) |
| 4096 (4)   | 8192-30720 (in 1024 increments) |

---

## Resources Created

### Networking

| Resource                 | Type                    | Description                                      |
|--------------------------|-------------------------|--------------------------------------------------|
| `AlbSecurityGroup`       | AWS::EC2::SecurityGroup | ALB security group (80, 443 from anywhere)       |
| `BackendSecurityGroup`   | AWS::EC2::SecurityGroup | Backend ECS security group (8080 from ALB)       |
| `FrontendSecurityGroup`  | AWS::EC2::SecurityGroup | Frontend ECS security group (3000 from ALB)      |
| `GotenbergSecurityGroup` | AWS::EC2::SecurityGroup | Gotenberg ECS security group (3000 from Backend) |
| `DatabaseSecurityGroup`  | AWS::EC2::SecurityGroup | RDS security group (5432 from Backend)           |

### Storage

| Resource         | Type                  | Description                                  |
|------------------|-----------------------|----------------------------------------------|
| `S3KmsKey`       | AWS::KMS::Key         | Customer-managed KMS key for S3 encryption   |
| `S3KmsKeyAlias`  | AWS::KMS::Alias       | Key alias `{stack}-s3-key`                   |
| `S3Bucket`       | AWS::S3::Bucket       | S3 bucket with KMS encryption and versioning |
| `S3BucketPolicy` | AWS::S3::BucketPolicy | Enforces encryption and HTTPS                |

### Database

| Resource           | Type                        | Description                       |
|--------------------|-----------------------------|-----------------------------------|
| `DbSubnetGroup`    | AWS::RDS::DBSubnetGroup     | DB subnet group (private subnets) |
| `DbParameterGroup` | AWS::RDS::DBParameterGroup  | PostgreSQL 16 parameter group     |
| `DbPassword`       | AWS::SecretsManager::Secret | Auto-generated database password  |
| `DbInstance`       | AWS::RDS::DBInstance        | PostgreSQL 16, encrypted at rest  |

### Load Balancing

| Resource               | Type                                      | Description                       |
|------------------------|-------------------------------------------|-----------------------------------|
| `Alb`                  | AWS::ElasticLoadBalancingV2::LoadBalancer | Application Load Balancer         |
| `FrontendTargetGroup`  | AWS::ElasticLoadBalancingV2::TargetGroup  | Frontend target group (3000)      |
| `BackendTargetGroup`   | AWS::ElasticLoadBalancingV2::TargetGroup  | Backend target group (8080)       |
| `GotenbergTargetGroup` | AWS::ElasticLoadBalancingV2::TargetGroup  | Gotenberg target group (3000)     |
| `AlbHttpListener`      | AWS::ElasticLoadBalancingV2::Listener     | HTTP listener (redirect to HTTPS) |
| `AlbHttpsListener`     | AWS::ElasticLoadBalancingV2::Listener     | HTTPS listener with certificate   |
| `BackendListenerRule`  | AWS::ElasticLoadBalancingV2::ListenerRule | Path-based routing `/api/*`       |

### Certificate

| Resource               | Type                                 | Description           |
|------------------------|--------------------------------------|-----------------------|
| `AcmCertificate`       | AWS::CertificateManager::Certificate | SSL/TLS certificate   |
| `AcmCertificateRecord` | AWS::Route53::RecordSetGroup         | DNS validation record |

### ECS

| Resource                  | Type                     | Description                             |
|---------------------------|--------------------------|-----------------------------------------|
| `EcsCluster`              | AWS::ECS::Cluster        | Fargate cluster with Container Insights |
| `BackendLogGroup`         | AWS::Logs::LogGroup      | Backend CloudWatch log group            |
| `FrontendLogGroup`        | AWS::Logs::LogGroup      | Frontend CloudWatch log group           |
| `GotenbergLogGroup`       | AWS::Logs::LogGroup      | Gotenberg CloudWatch log group          |
| `TaskExecutionRole`       | AWS::IAM::Role           | ECS task execution role                 |
| `BackendTaskRole`         | AWS::IAM::Role           | Backend task role (S3, KMS access)      |
| `BackendTaskDefinition`   | AWS::ECS::TaskDefinition | Backend task definition                 |
| `FrontendTaskDefinition`  | AWS::ECS::TaskDefinition | Frontend task definition                |
| `GotenbergTaskDefinition` | AWS::ECS::TaskDefinition | Gotenberg task definition               |
| `BackendService`          | AWS::ECS::Service        | Backend ECS service                     |
| `FrontendService`         | AWS::ECS::Service        | Frontend ECS service                    |
| `GotenbergService`        | AWS::ECS::Service        | Gotenberg ECS service                   |

### Auto Scaling

| Resource                    | Type                                        | Description                    |
|-----------------------------|---------------------------------------------|--------------------------------|
| `BackendScalableTarget`     | AWS::ApplicationAutoScaling::ScalableTarget | Backend scaling target         |
| `BackendCpuScalingPolicy`   | AWS::ApplicationAutoScaling::ScalingPolicy  | CPU-based scaling (70% target) |
| `FrontendScalableTarget`    | AWS::ApplicationAutoScaling::ScalableTarget | Frontend scaling target        |
| `FrontendCpuScalingPolicy`  | AWS::ApplicationAutoScaling::ScalingPolicy  | CPU-based scaling (70% target) |
| `GotenbergScalableTarget`   | AWS::ApplicationAutoScaling::ScalableTarget | Gotenberg scaling target       |
| `GotenbergCpuScalingPolicy` | AWS::ApplicationAutoScaling::ScalingPolicy  | CPU-based scaling (80% target) |

### DNS

| Resource    | Type                         | Description           |
|-------------|------------------------------|-----------------------|
| `DnsRecord` | AWS::Route53::RecordSetGroup | Alias A record to ALB |

---

## Security Configuration

### KMS Key Policy

The KMS key is configured with:
- Root account full access
- Backend task role access (Decrypt, GenerateDataKey)

```json
{
  "Statement": [
    {
      "Sid": "EnableRootAccess",
      "Effect": "Allow",
      "Principal": {"AWS": "arn:aws:iam::<account>:root"},
      "Action": "kms:*",
      "Resource": "*"
    },
    {
      "Sid": "AllowECSTaskRole",
      "Effect": "Allow",
      "Principal": {"AWS": "<api-task-role-arn>"},
      "Action": ["kms:Decrypt", "kms:GenerateDataKey"],
      "Resource": "*"
    }
  ]
}
```

### S3 Bucket Policy

Enforces:
- All uploads must use KMS encryption
- All requests must use HTTPS

### IAM Roles

#### TaskExecutionRole

Used by all ECS tasks for:
- ECR image pulls
- CloudWatch Logs
- Secrets Manager access (for DB password)

#### BackendTaskRole

Backend-specific permissions:
- S3: PutObject, GetObject, DeleteObject, ListBucket, GetBucketLocation
- KMS: Decrypt, GenerateDataKey

### RDS Encryption

- Storage encrypted at rest using AWS-managed key
- Connections require SSL (`DB_SSLMODE=require`)

---

## Auto Scaling

### Scaling Policies

| Service   | Metric | Target | Scale Out | Scale In |
|-----------|--------|--------|-----------|----------|
| Backend   | CPU    | 70%    | 60s       | 120s     |
| Frontend  | CPU    | 70%    | 60s       | 120s     |
| Gotenberg | CPU    | 80%    | 60s       | 180s     |

### Manual Scaling

```bash
# Update desired count
aws ecs update-service \
  --cluster docshare-cluster \
  --service api \
  --desired-count 3

# Update scaling limits (requires stack update)
aws cloudformation update-stack \
  --stack-name docshare \
  --use-previous-template \
  --parameters \
      ParameterKey=BackendMinCount,ParameterValue=2 \
      ParameterKey=BackendMaxCount,ParameterValue=8 \
      [... other parameters ...]
  --capabilities CAPABILITY_IAM
```

---

## Cost Estimation

### Monthly Costs (us-east-1, single AZ)

| Resource                  | Config               | Estimated Cost      |
|---------------------------|----------------------|---------------------|
| Application Load Balancer | 1 ALB                | ~$20                |
| ALB LCU                   | ~1000 requests/hour  | ~$5                 |
| NAT Gateway               | 1 NAT (optional)     | ~$32                |
| RDS PostgreSQL            | db.t4g.micro         | ~$15                |
| RDS Storage               | 20 GB GP3            | ~$2                 |
| ECS Backend               | 0.25 vCPU, 512 MB x1 | ~$15                |
| ECS Frontend              | 0.25 vCPU, 512 MB x1 | ~$15                |
| ECS Gotenberg             | 0.5 vCPU, 1 GB x1    | ~$25                |
| S3 Storage                | 10 GB                | ~$0.23              |
| S3 Requests               | Variable             | ~$1                 |
| CloudWatch Logs           | 1 GB/day             | ~$0.50              |
| Secrets Manager           | 1 secret             | ~$0.40              |
| **Total (no NAT)**        |                      | **~$98-100/month**  |
| **Total (with NAT)**      |                      | **~$130-150/month** |

### Cost Optimization Tips

1. **Use Savings Plans** for RDS and ECS
2. **Reduce task sizes** for dev/staging
3. **Use Spot capacity** for stateless services
4. **Reduce log retention** to 7 days for non-production
5. **Use NAT Instance** instead of NAT Gateway for low traffic

---

## Deployment Walkthrough

### Step-by-Step Process

1. **Certificate Validation (5-30 min)**
   - ACM certificate created
   - DNS CNAME record created in Route53
   - Certificate validated automatically

2. **RDS Instance Creation (10-15 min)**
   - DB subnet group created
   - Secret generated in Secrets Manager
   - RDS instance created (encrypted)
   - Database password attached to secret

3. **S3 Bucket Creation**
   - KMS key created
   - Bucket created with encryption
   - Public access blocked
   - Bucket policy applied

4. **ECS Cluster Creation**
   - Cluster created with Container Insights
   - Log groups created
   - Task definitions registered
   - IAM roles created

5. **Load Balancer Creation**
   - ALB created in public subnets
   - Target groups created
   - HTTPS listener configured
   - Path-based routing configured

6. **ECS Services Deployment**
   - Services created
   - Tasks launched in private subnets
   - Health checks passing

7. **DNS Configuration**
   - Route53 alias record created
   - DNS propagates globally

### Monitoring Progress

```bash
# Watch stack events
aws cloudformation describe-stack-events \
  --stack-name docshare \
  --query 'StackEvents[*].[Timestamp,ResourceStatus,ResourceType,LogicalResourceId]' \
  --output table

# Check specific resource
aws cloudformation describe-stack-resource \
  --stack-name docshare \
  --logical-resource-id DbInstance \
  --query 'StackResourceDetail.ResourceStatus'
```

---

## Updating the Stack

### Update Container Images

```bash
# Force new deployment with same task definition
aws ecs update-service \
  --cluster docshare-cluster \
  --service api \
  --force-new-deployment

# Or update to new image version (requires stack update)
# Modify BackendTaskDefinition in the template or use:
aws cloudformation update-stack \
  --stack-name docshare \
  --use-previous-template \
  --parameters \
      [... existing parameters ...] \
      ParameterKey=BackendCpu,ParameterValue=512 \
      ParameterKey=BackendMemory,ParameterValue=1024 \
  --capabilities CAPABILITY_IAM
```

### Update Environment Variables

Update the template and redeploy, or use AWS CLI:

```bash
# Register new task definition with updated env vars
aws ecs register-task-definition \
  --family docshare-api \
  --container-definitions '[...]'

# Update service to use new task definition
aws ecs update-service \
  --cluster docshare-cluster \
  --service api \
  --task-definition docshare-api:N
```

### Upgrade RDS Instance

```bash
aws cloudformation update-stack \
  --stack-name docshare \
  --use-previous-template \
  --parameters \
      [... existing parameters ...] \
      ParameterKey=DbInstanceClass,ParameterValue=db.t4g.small \
  --capabilities CAPABILITY_IAM
```

---

## Deleting the Stack

### Standard Deletion

```bash
aws cloudformation delete-stack --stack-name docshare
```

### What Gets Retained

- **RDS Final Snapshot**: Automatically created (unless disabled)
- **S3 Objects**: Bucket cannot be deleted if non-empty

### Clean Up S3 Bucket Before Deletion

```bash
# Empty the bucket
aws s3 rm s3://docshare-storage --recursive

# Then delete the stack
aws cloudformation delete-stack --stack-name docshare

# Or disable RDS final snapshot (not recommended for production)
aws cloudformation delete-stack \
  --stack-name docshare \
  --deletion-mode FORCE_DELETE_STACK
```

### Delete RDS Final Snapshot Manually

```bash
# List snapshots
aws rds describe-db-snapshots --db-instance-identifier docshare-postgres

# Delete snapshot
aws rds delete-db-snapshot --db-snapshot-identifier docshare-postgres-final-snapshot
```

---

## Troubleshooting

### Certificate Validation Stuck

**Symptom**: Certificate remains in `PENDING_VALIDATION` status

**Diagnosis**:
```bash
aws acm describe-certificate \
  --certificate-arn <arn> \
  --query 'Certificate.DomainValidationOptions'
```

**Solution**:
- Verify Route53 record exists
- Wait for DNS propagation (up to 30 minutes)
- Check the CNAME record matches ACM requirements

### ECS Tasks Not Starting

**Symptom**: Tasks in `PENDING` status, then stopped

**Diagnosis**:
```bash
# Check task events
aws ecs describe-tasks \
  --cluster docshare-cluster \
  --tasks <task-arn> \
  --query 'tasks[0].stops[0]'

# Check service events
aws ecs describe-services \
  --cluster docshare-cluster \
  --services api \
  --query 'services[0].events[0:5]'
```

**Common Causes**:
1. **Image pull error**: Check ECR permissions, image exists
2. **Out of memory**: Increase task memory
3. **ENI attachment failed**: Check subnet capacity
4. **Security group**: Check outbound rules for image pulls

### Database Connection Failures

**Symptom**: Backend tasks fail health checks

**Diagnosis**:
```bash
# Check RDS status
aws rds describe-db-instances \
  --db-instance-identifier docshare-postgres \
  --query 'DBInstances[0].DBInstanceStatus'

# Check security group rules
aws ec2 describe-security-groups \
  --group-ids <security-group-id> \
  --query 'SecurityGroups[0].IpPermissions'
```

**Solutions**:
- Verify BackendSecurityGroup has access to DatabaseSecurityGroup
- Check DB password secret exists
- Verify RDS is in available status

### ALB 502 Errors

**Symptom**: Intermittent 502 Bad Gateway errors

**Diagnosis**:
```bash
# Check target health
aws elbv2 describe-target-health \
  --target-group-arn <arn> \
  --query 'TargetHealthDescriptions[*].TargetHealth'
```

**Common Causes**:
1. **Task startup time**: Increase health check grace period
2. **Container crash**: Check CloudWatch logs
3. **Memory pressure**: Increase task memory

### Logs Not Appearing

**Symptom**: No logs in CloudWatch

**Diagnosis**:
```bash
# Check log group exists
aws logs describe-log-groups \
  --log-group-name-prefix /ecs/docshare

# Check task execution role permissions
aws iam get-role-policy \
  --role-name docshare-task-execution-role \
  --policy-name logs-policy
```

**Solution**: Ensure TaskExecutionRole has `logs:CreateLogStream` and `logs:PutLogEvents`

---

## Advanced Configuration

### Custom Domain Configuration

For domains outside the hosted zone:

```bash
# Create a CNAME record in your DNS provider
# Point to the ALB DNS name
docshare.example.com CNAME docshare-alb-123456.us-east-1.elb.amazonaws.com
```

### Multi-AZ RDS

Modify the template for production:

```yaml
DbInstance:
  Type: AWS::RDS::DBInstance
  Properties:
    MultiAZ: true
```

### Cross-Region S3 Replication

Add to the template for disaster recovery:

```yaml
ReplicationRole:
  Type: AWS::IAM::Role
  Properties:
    AssumeRolePolicyDocument:
      Version: '2012-10-17'
      Statement:
        - Effect: Allow
          Principal:
            Service: s3.amazonaws.com
          Action: sts:AssumeRole

ReplicationConfig:
  Type: AWS::S3::BucketReplicationConfiguration
  Properties:
    Bucket: !Ref S3Bucket
    Role: !GetAtt ReplicationRole.Arn
    Rules:
      - Id: ReplicateToBackupRegion
        Status: Enabled
        Destination:
          Bucket: arn:aws:s3:::docshare-backup-bucket
          StorageClass: STANDARD_IA
```

### Fargate Spot

Reduce costs using Spot capacity:

```yaml
BackendService:
  Properties:
    CapacityProviderStrategy:
      - CapacityProvider: FARGATE_SPOT
        Weight: 1
      - CapacityProvider: FARGATE
        Weight: 1
        Base: 1  # At least 1 on-demand
```

### Private ECR Repository

For private ECR repositories, add:

```yaml
BackendTaskDefinition:
  Properties:
    ContainerDefinitions:
      - Name: api
        Image: <account-id>.dkr.ecr.<region>.amazonaws.com/docshare/api:latest
```

Update TaskExecutionRole to include:

```json
{
  "Effect": "Allow",
  "Action": [
    "ecr:GetAuthorizationToken",
    "ecr:BatchCheckLayerAvailability",
    "ecr:GetDownloadUrlForLayer",
    "ecr:BatchGetImage"
  ],
  "Resource": "*"
}
```

---

## See Also

- [Deployment Guide](DEPLOYMENT.md) - General deployment options
- [Helm Chart Reference](HELM.md) - Kubernetes deployment
- [API Documentation](API.md) - API reference
- [Examples](../examples/) - Docker Compose and Helm examples
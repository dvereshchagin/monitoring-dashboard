# 🗺️ Terraform в проекте Monitoring Dashboard

## Общая архитектура

```
┌─────────────────────────────────────────────────────────────────────┐
│                         AWS Cloud                                    │
│                                                                       │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  Staging Environment (terraform/live/staging)                  │  │
│  │                                                                 │  │
│  │  ┌────────────────────────────────────────────────────────┐   │  │
│  │  │ VPC 10.0.0.0/16 (module "network")                     │   │  │
│  │  │                                                          │   │  │
│  │  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │   │  │
│  │  │  │ Public       │  │ Public       │  │ Public       │  │   │  │
│  │  │  │ Subnet       │  │ Subnet       │  │ Subnet       │  │   │  │
│  │  │  │ us-east-1a   │  │ us-east-1b   │  │ us-east-1c   │  │   │  │
│  │  │  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  │   │  │
│  │  │         │                  │                  │          │   │  │
│  │  │  ┌──────▼───────┐  ┌──────▼───────┐  ┌──────▼───────┐  │   │  │
│  │  │  │ Private      │  │ Private      │  │ Private      │  │   │  │
│  │  │  │ Subnet       │  │ Subnet       │  │ Subnet       │  │   │  │
│  │  │  │ (EKS Nodes)  │  │ (EKS Nodes)  │  │ (EKS Nodes)  │  │   │  │
│  │  │  └──────────────┘  └──────────────┘  └──────────────┘  │   │  │
│  │  └────────────────────────────────────────────────────────┘   │  │
│  │                                                                 │  │
│  │  ┌────────────────────────────────────────────────────────┐   │  │
│  │  │ EKS Cluster (module "eks")                             │   │  │
│  │  │  - Kubernetes 1.28                                      │   │  │
│  │  │  - Node Groups (t3.medium x 2-4)                       │   │  │
│  │  │  - Autoscaling                                          │   │  │
│  │  └────────────────────────────────────────────────────────┘   │  │
│  │                                                                 │  │
│  │  ┌────────────────────────────────────────────────────────┐   │  │
│  │  │ RDS PostgreSQL (module "rds")                          │   │  │
│  │  │  - Instance: db.t3.micro                               │   │  │
│  │  │  - Multi-AZ: false                                     │   │  │
│  │  │  - Storage: 20 GB                                      │   │  │
│  │  └────────────────────────────────────────────────────────┘   │  │
│  │                                                                 │  │
│  │  ┌────────────────────────────────────────────────────────┐   │  │
│  │  │ ECR Repository (module "ecr")                          │   │  │
│  │  │  - monitoring-dashboard                                │   │  │
│  │  │  - Image retention: 30 days                            │   │  │
│  │  └────────────────────────────────────────────────────────┘   │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                                                                       │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  Production Environment (terraform/live/prod)                 │  │
│  │  [Аналогичная структура, но с большими ресурсами]            │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                       │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  Terraform Backend (terraform/bootstrap)                      │  │
│  │                                                                 │  │
│  │  ┌────────────────┐         ┌─────────────────────┐          │  │
│  │  │ S3 Bucket      │         │ DynamoDB Table      │          │  │
│  │  │ - tfstate files│         │ - State locks       │          │  │
│  │  │ - Versioning   │         │ - LockID            │          │  │
│  │  │ - Encryption   │         │                     │          │  │
│  │  └────────────────┘         └─────────────────────┘          │  │
│  └─────────────────────────────────────────────────────────────┘  │
└───────────────────────────────────────────────────────────────────┘
```

---

## Последовательность создания (Dependency Graph)

```
┌──────────────────────────┐
│ 0. Bootstrap             │
│ (создаём backend)        │
│  - S3 Bucket             │
│  - DynamoDB Table        │
└──────────┬───────────────┘
           │
           ▼
┌──────────────────────────┐
│ 1. Network Module        │
│  - VPC                   │
│  - Internet Gateway      │
│  - Public Subnets (3)    │
│  - Private Subnets (3)   │
│  - NAT Gateway           │
│  - Route Tables          │
└──────────┬───────────────┘
           │
           ├──────────┐
           │          │
           ▼          ▼
┌──────────────────┐  ┌──────────────────┐
│ 2a. EKS Module   │  │ 2b. ECR Module   │
│  - Cluster       │  │  - Repository    │
│  - Node Groups   │  │  - Lifecycle     │
│  - Security      │  │    Policy        │
└──────────┬───────┘  └──────────────────┘
           │
           ▼
┌──────────────────────────┐
│ 3. RDS Module            │
│  - Subnet Group          │
│  - Security Group        │
│  - DB Instance           │
│    (использует SG из EKS)│
└──────────────────────────┘
```

**Почему такой порядок?**
- **Network** первый - все остальные зависят от VPC и подсетей
- **EKS и ECR** параллельно - не зависят друг от друга
- **RDS** последний - нужен Security Group от EKS (для доступа из кластера)

---

## Файловая структура с пояснениями

```
infra/terraform/
│
├── bootstrap/                          # Шаг 0: Создание backend
│   ├── main.tf                        # S3 bucket + DynamoDB table
│   ├── variables.tf                   # Параметры: bucket name, region
│   ├── outputs.tf                     # Выводит имена созданных ресурсов
│   └── terraform.tfvars.example       # Пример конфига
│
├── modules/                            # Переиспользуемые блоки
│   │
│   ├── network/                       # VPC, Subnets, NAT
│   │   ├── main.tf                   # Логика создания сети
│   │   ├── variables.tf              # Входы: vpc_cidr, az_zones, etc.
│   │   └── outputs.tf                # Выходы: vpc_id, subnet_ids
│   │
│   ├── eks/                           # Kubernetes кластер
│   │   ├── main.tf                   # EKS cluster + node groups
│   │   ├── variables.tf              # Входы: cluster_version, node_size
│   │   └── outputs.tf                # Выходы: cluster_endpoint, sg_id
│   │
│   ├── rds/                           # PostgreSQL база
│   │   ├── main.tf                   # RDS instance + security groups
│   │   ├── variables.tf              # Входы: db_name, db_password
│   │   └── outputs.tf                # Выходы: endpoint, port
│   │
│   └── ecr/                           # Docker registry
│       ├── main.tf                   # ECR repository + policies
│       ├── variables.tf              # Входы: repository_name
│       └── outputs.tf                # Выходы: repository_url
│
└── live/                               # Окружения (используют modules)
    │
    ├── staging/                       # Staging environment
    │   ├── main.tf                   # Вызывает все 4 модуля
    │   │                             # module "network" { ... }
    │   │                             # module "eks" { ... }
    │   │                             # module "rds" { ... }
    │   │                             # module "ecr" { ... }
    │   │
    │   ├── variables.tf              # Определение переменных
    │   ├── terraform.tfvars          # Значения для staging
    │   │                             # environment = "staging"
    │   │                             # vpc_cidr = "10.0.0.0/16"
    │   │                             # node_instance_types = ["t3.medium"]
    │   │
    │   ├── backend.tf                # Настройка S3 backend
    │   ├── backend.hcl               # Параметры backend (не в Git)
    │   ├── providers.tf              # AWS provider config
    │   └── outputs.tf                # Выводит все важные данные
    │
    └── prod/                          # Production environment
        ├── main.tf                   # Те же модули, другие параметры
        ├── variables.tf
        ├── terraform.tfvars          # environment = "prod"
        │                             # node_instance_types = ["t3.large"]
        ├── backend.tf
        ├── backend.hcl
        ├── providers.tf
        └── outputs.tf
```

---

## Пример: Как создаётся Staging

### 1. Bootstrap Backend (один раз для всего проекта)

```bash
cd infra/terraform/bootstrap
terraform init    # Локальный state
terraform apply   # Создаёт S3 и DynamoDB
```

**Что создаётся:**
- S3 bucket: `monitoring-terraform-state`
- DynamoDB table: `terraform-locks`

### 2. Staging Environment

```bash
cd infra/terraform/live/staging
terraform init -backend-config=backend.hcl  # Использует S3 backend
terraform plan    # Показывает что будет создано
terraform apply   # Создаёт инфраструктуру
```

**Что происходит внутри:**

```
1. Terraform читает main.tf:
   ┌─────────────────────────────────────────┐
   │ module "network" {                      │
   │   source = "../../modules/network"      │
   │   vpc_cidr = "10.0.0.0/16"             │
   │   ...                                   │
   │ }                                       │
   │                                         │
   │ module "eks" {                          │
   │   source = "../../modules/eks"          │
   │   subnet_ids = module.network.subnet_ids│ ← Зависимость!
   │   ...                                   │
   │ }                                       │
   │                                         │
   │ module "rds" {                          │
   │   source = "../../modules/rds"          │
   │   vpc_id = module.network.vpc_id       │ ← Зависимость!
   │   ...                                   │
   │ }                                       │
   └─────────────────────────────────────────┘

2. Строит граф зависимостей:
   network → eks
   network → rds (+ eks для security group)

3. Выполняет в правильном порядке:
   Step 1: module.network (VPC, Subnets, NAT)
   Step 2: module.eks + module.ecr (параллельно)
   Step 3: module.rds (использует outputs от network и eks)

4. Сохраняет state в S3:
   s3://monitoring-terraform-state/staging/terraform.tfstate
```

---

## Как работает каждый модуль

### Network Module

**Входы (variables.tf):**
```terraform
variable "vpc_cidr" {}            # "10.0.0.0/16"
variable "public_subnet_cidrs" {} # ["10.0.1.0/24", "10.0.2.0/24", ...]
variable "availability_zones" {}  # ["us-east-1a", "us-east-1b", ...]
```

**Создаёт (main.tf):**
```
1. VPC (10.0.0.0/16)
2. Internet Gateway (для публичного доступа)
3. Public Subnets x3 (в разных AZ)
4. Private Subnets x3 (в разных AZ)
5. NAT Gateway (для private → internet)
6. Route Tables (маршрутизация)
```

**Выходы (outputs.tf):**
```terraform
output "vpc_id" {}              # vpc-abc123
output "public_subnet_ids" {}   # [subnet-pub1, subnet-pub2, ...]
output "private_subnet_ids" {}  # [subnet-priv1, subnet-priv2, ...]
```

### EKS Module

**Входы:**
```terraform
variable "subnet_ids" {}          # Из network module
variable "cluster_version" {}     # "1.28"
variable "node_instance_types" {} # ["t3.medium"]
```

**Создаёт:**
```
1. EKS Cluster (control plane)
2. Node Groups (worker nodes)
3. IAM Roles (для кластера и нод)
4. Security Groups
```

**Выходы:**
```terraform
output "cluster_endpoint" {}           # https://...eks.amazonaws.com
output "cluster_security_group_id" {}  # sg-abc123
```

### RDS Module

**Входы:**
```terraform
variable "vpc_id" {}                      # Из network
variable "subnet_ids" {}                  # Из network
variable "allowed_security_group_ids" {}  # Из eks (для доступа)
variable "db_password" {}                 # Секрет!
```

**Создаёт:**
```
1. DB Subnet Group (где размещать RDS)
2. Security Group (кто может подключиться)
3. RDS Instance (PostgreSQL)
```

**Выходы:**
```terraform
output "endpoint" {}  # monitoring-staging.abc.rds.amazonaws.com:5432
output "port" {}      # 5432
```

---

## State Management

### Как хранится State

```
S3 Bucket: monitoring-terraform-state
│
├── staging/
│   └── terraform.tfstate         ← Staging environment state
│       {
│         "resources": [
│           {
│             "module": "module.network",
│             "type": "aws_vpc",
│             "instances": [{
│               "attributes": {
│                 "id": "vpc-staging123",
│                 "cidr_block": "10.0.0.0/16"
│               }
│             }]
│           },
│           {
│             "module": "module.eks",
│             "type": "aws_eks_cluster",
│             "instances": [{
│               "attributes": {
│                 "id": "monitoring-staging",
│                 "endpoint": "https://..."
│               }
│             }]
│           }
│         ]
│       }
│
└── prod/
    └── terraform.tfstate          ← Production environment state
```

### Блокировки (DynamoDB)

```
DynamoDB Table: terraform-locks
│
├── LockID: "monitoring-terraform-state/staging/terraform.tfstate-md5"
│   Status: LOCKED
│   Who: user@example.com
│   When: 2026-02-07 16:30:00
│   Info: "terraform apply"
│
└── LockID: "monitoring-terraform-state/prod/terraform.tfstate-md5"
    Status: UNLOCKED
```

**Зачем?**
- Если два человека одновременно делают `terraform apply`, второй получит ошибку
- Защита от race conditions и повреждения state

---

## Переменные и их приоритет

Terraform ищет значения переменных в порядке (последнее перезаписывает):

```
1. Default в variables.tf
   variable "environment" {
     default = "dev"
   }

2. Файл terraform.tfvars
   environment = "staging"

3. Файл *.auto.tfvars
   environment = "staging"

4. Переменная окружения
   export TF_VAR_environment="staging"

5. Флаг командной строки
   terraform apply -var="environment=staging"

6. Интерактивный ввод
   terraform apply
   # var.environment
   #   Enter a value: staging
```

**В вашем проекте:**
```bash
# Секреты через переменные окружения
export TF_VAR_db_password="super-secret"
terraform apply

# Остальное в terraform.tfvars
environment = "staging"
vpc_cidr = "10.0.0.0/16"
```

---

## CI/CD Integration (GitHub Actions)

```yaml
# .github/workflows/deploy-staging.yml

- name: Terraform Init
  run: |
    cd infra/terraform/live/staging
    terraform init -backend-config=backend.hcl

- name: Terraform Plan
  run: terraform plan -out=tfplan
  env:
    TF_VAR_db_password: ${{ secrets.DB_PASSWORD }}

- name: Terraform Apply
  run: terraform apply tfplan
```

**Переменные хранятся в:**
- GitHub Secrets (секреты)
- GitHub Variables (несекретные параметры)
- AWS Secrets Manager (для production)

---

## Практические сценарии

### Сценарий 1: Добавить новый регион

```terraform
# terraform.tfvars
availability_zones = ["us-east-1a", "us-east-1b", "us-east-1c"]

# Добавляем us-east-1d
availability_zones = ["us-east-1a", "us-east-1b", "us-east-1c", "us-east-1d"]

# terraform plan покажет:
# + module.network.aws_subnet.public[3] will be created
# + module.network.aws_subnet.private[3] will be created
```

### Сценарий 2: Масштабировать EKS

```terraform
# Было
node_desired_size = 2
node_instance_types = ["t3.medium"]

# Стало
node_desired_size = 4
node_instance_types = ["t3.large"]

# terraform plan покажет:
# ~ module.eks.aws_eks_node_group.main will be updated
#   ~ desired_size = 2 -> 4
#   ~ instance_types = ["t3.medium"] -> ["t3.large"]
```

### Сценарий 3: Откат изменений

```bash
# Посмотреть версии state в S3
aws s3api list-object-versions --bucket monitoring-terraform-state \
  --prefix staging/terraform.tfstate

# Восстановить предыдущую версию
aws s3api get-object --bucket monitoring-terraform-state \
  --key staging/terraform.tfstate \
  --version-id <VERSION_ID> \
  terraform.tfstate.backup

# Загрузить обратно
aws s3 cp terraform.tfstate.backup \
  s3://monitoring-terraform-state/staging/terraform.tfstate
```

---

## Мониторинг затрат

```terraform
# Добавить теги для Cost Allocation
locals {
  common_tags = {
    Project     = "monitoring-dashboard"
    Environment = var.environment
    ManagedBy   = "terraform"
    CostCenter  = "engineering"
    Owner       = "platform-team"
  }
}

# Применить ко всем ресурсам
resource "aws_vpc" "main" {
  tags = local.common_tags
}
```

**В AWS Cost Explorer можно фильтровать по:**
- Project: monitoring-dashboard
- Environment: staging / prod
- ManagedBy: terraform

---

## Безопасность

### Что НЕ коммитить в Git:

```gitignore
# .gitignore
**/.terraform/*
*.tfstate
*.tfstate.*
terraform.tfvars      # Может содержать секреты
*.auto.tfvars
backend.hcl           # Содержит bucket name
override.tf
.terraformrc
terraform.rc
```

### Секреты в AWS Secrets Manager:

```terraform
# Читать секрет из AWS
data "aws_secretsmanager_secret_version" "db_password" {
  secret_id = "monitoring/staging/db_password"
}

resource "aws_db_instance" "main" {
  password = data.aws_secretsmanager_secret_version.db_password.secret_string
}
```

---

## Полезные команды для вашего проекта

```bash
# Форматировать все файлы
terraform fmt -recursive

# Проверить модули
terraform validate

# Граф зависимостей (требует graphviz)
terraform graph | dot -Tsvg > graph.svg

# Импорт существующего VPC
terraform import module.network.aws_vpc.this vpc-existing123

# Показать только outputs
terraform output

# Показать output в JSON (для скриптов)
terraform output -json > outputs.json

# Удалить только один модуль
terraform destroy -target=module.rds
```

---

Теперь у вас есть полная картина работы Terraform в вашем проекте! 🎉

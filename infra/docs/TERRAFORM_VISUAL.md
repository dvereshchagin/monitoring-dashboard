# 🎨 Terraform Visual Guide

Визуальное представление работы Terraform в одной схеме.

---

## Terraform за 1 минуту

```
┌─────────────────────────────────────────────────────────────────┐
│  ВЫ ПИШЕТЕ КОД (.tf файлы)                                      │
│                                                                  │
│  resource "aws_vpc" "main" {                                    │
│    cidr_block = "10.0.0.0/16"                                   │
│  }                                                               │
│                                                                  │
│  resource "aws_subnet" "public" {                               │
│    vpc_id = aws_vpc.main.id  ← зависимость                     │
│  }                                                               │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│  TERRAFORM КОМАНДЫ                                               │
│                                                                  │
│  $ terraform init     ← Скачивает провайдеры (AWS, etc)        │
│  $ terraform plan     ← Показывает ЧТО изменится               │
│  $ terraform apply    ← Реально СОЗДАЁТ в облаке               │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│  TERRAFORM АНАЛИЗИРУЕТ                                           │
│                                                                  │
│  1. Читает ваш код                                              │
│  2. Читает текущее состояние (state)                           │
│  3. Сравнивает: "Что есть" vs "Что должно быть"                │
│  4. Строит граф: "Сначала VPC, потом Subnet"                   │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│  AWS API ВЫЗОВЫ                                                  │
│                                                                  │
│  1. CreateVpc(cidr="10.0.0.0/16")                               │
│     Response: vpc_id = "vpc-abc123" ✅                          │
│                                                                  │
│  2. CreateSubnet(vpc_id="vpc-abc123", ...)                      │
│     Response: subnet_id = "subnet-xyz789" ✅                    │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│  STATE ФАЙЛ ОБНОВЛЯЕТСЯ                                          │
│                                                                  │
│  terraform.tfstate = {                                          │
│    "aws_vpc.main": {                                            │
│      "id": "vpc-abc123",                                        │
│      "cidr_block": "10.0.0.0/16"                                │
│    },                                                            │
│    "aws_subnet.public": {                                       │
│      "id": "subnet-xyz789",                                     │
│      "vpc_id": "vpc-abc123"                                     │
│    }                                                             │
│  }                                                               │
└─────────────────────────────────────────────────────────────────┘
```

---

## Основные концепции на примерах

### 1️⃣ Resource (Ресурс)

```
┌──────────────────────────────────────┐
│  КОД                                  │
│  resource "aws_s3_bucket" "my_data" {│
│    bucket = "my-unique-bucket"       │
│  }                                    │
└──────────────┬───────────────────────┘
               │
               ▼
┌──────────────────────────────────────┐
│  AWS                                  │
│  ☁️  S3 Bucket                       │
│     Name: my-unique-bucket           │
│     Status: Active                   │
└──────────────────────────────────────┘
```

### 2️⃣ Variable (Переменная)

```
┌──────────────────────────────────────┐
│  ОПРЕДЕЛЕНИЕ                          │
│  variable "env" {                    │
│    type    = string                  │
│    default = "dev"                   │
│  }                                    │
└──────────────┬───────────────────────┘
               │
               ▼
┌──────────────────────────────────────┐
│  ИСПОЛЬЗОВАНИЕ                        │
│  resource "aws_vpc" "main" {         │
│    tags = {                          │
│      Environment = var.env           │
│    }                                  │
│  }                                    │
└──────────────────────────────────────┘
```

### 3️⃣ Module (Модуль)

```
┌────────────────────────────────────────────────────────┐
│  МОДУЛЬ (modules/network/)                             │
│                                                         │
│  variables.tf         main.tf          outputs.tf      │
│  ┌──────────┐        ┌──────────┐     ┌──────────┐    │
│  │ vpc_cidr │──────▶ │ VPC      │────▶│ vpc_id   │    │
│  │ env      │        │ Subnets  │     │ subnet_* │    │
│  └──────────┘        │ NAT      │     └──────────┘    │
│                      └──────────┘                      │
└────────────────────────────────────────────────────────┘
                          ▲
                          │
┌─────────────────────────┴──────────────────────────────┐
│  ИСПОЛЬЗОВАНИЕ МОДУЛЯ (live/staging/main.tf)           │
│                                                         │
│  module "network" {                                    │
│    source   = "../../modules/network"                 │
│    vpc_cidr = "10.0.0.0/16"                           │
│    env      = "staging"                               │
│  }                                                     │
│                                                         │
│  output "vpc" {                                        │
│    value = module.network.vpc_id                      │
│  }                                                     │
└────────────────────────────────────────────────────────┘
```

### 4️⃣ State (Состояние)

```
┌────────────────────────────────────────────────────────┐
│  КОД (desired state)                                    │
│                                                         │
│  resource "aws_instance" "app" {                       │
│    instance_type = "t3.large"  ← Хотим LARGE          │
│  }                                                      │
└──────────────┬─────────────────────────────────────────┘
               │
               ▼
┌────────────────────────────────────────────────────────┐
│  STATE (current state)                                  │
│                                                         │
│  {                                                      │
│    "aws_instance.app": {                               │
│      "id": "i-abc123",                                 │
│      "instance_type": "t3.medium"  ← Сейчас MEDIUM    │
│    }                                                    │
│  }                                                      │
└──────────────┬─────────────────────────────────────────┘
               │
               ▼
┌────────────────────────────────────────────────────────┐
│  TERRAFORM PLAN                                         │
│                                                         │
│  ~ aws_instance.app will be updated in-place           │
│    ~ instance_type = "t3.medium" -> "t3.large"         │
│                                                         │
│  Plan: 0 to add, 1 to change, 0 to destroy.           │
└────────────────────────────────────────────────────────┘
```

---

## Граф зависимостей в проекте

```
                    bootstrap/
                        │
                        ▼
                 ┌─────────────┐
                 │ S3 Bucket   │
                 │ DynamoDB    │
                 └──────┬──────┘
                        │
                        ▼
                 [Remote State]
                        │
        ┌───────────────┴───────────────┐
        │                                │
        ▼                                ▼
  live/staging/                    live/prod/
        │                                │
        └──────────┬─────────────────────┘
                   │
                   ▼
           ┌───────────────┐
           │ modules/      │
           │  - network    │──┐
           │  - eks        │  │
           │  - rds        │  │
           │  - ecr        │  │
           └───────────────┘  │
                              │
        ┌─────────────────────┘
        │
        ▼
   ┌─────────────────────────────────────────┐
   │ Порядок создания в AWS:                 │
   │                                          │
   │  1. Network (VPC, Subnets, NAT)         │
   │       ↓                                  │
   │  2. EKS Cluster + ECR (параллельно)     │
   │       ↓                                  │
   │  3. RDS (использует network + eks)      │
   └─────────────────────────────────────────┘
```

---

## Workflow: От кода до AWS

```
                   РАЗРАБОТЧИК
                        │
                        │ 1. Пишет код
                        ▼
                   ┌─────────┐
                   │ main.tf │
                   │ vars.tf │
                   └────┬────┘
                        │
                        │ 2. git commit & push
                        ▼
                   ┌─────────┐
                   │ GitHub  │
                   └────┬────┘
                        │
                        │ 3. Trigger
                        ▼
              ┌──────────────────┐
              │ GitHub Actions   │
              │                  │
              │ terraform init   │
              │ terraform plan   │
              │ terraform apply  │
              └────┬────────┬────┘
                   │        │
     4. Read State │        │ 5. Write State
                   │        │
                   ▼        ▼
            ┌───────────────────┐
            │ S3 + DynamoDB     │
            │ (Remote State)    │
            └─────────┬─────────┘
                      │
                      │ 6. API Calls
                      ▼
            ┌───────────────────┐
            │       AWS         │
            │                   │
            │  VPC    EKS   RDS │
            │                   │
            └───────────────────┘
```

---

## Plan vs Apply

```
┌──────────────────────────────────────────────────────┐
│  terraform plan                                       │
│  (Только показывает, НЕ изменяет)                   │
│                                                       │
│  Terraform will perform the following actions:       │
│                                                       │
│  + aws_vpc.main                                      │
│      id:         (known after apply)                 │
│      cidr_block: "10.0.0.0/16"                       │
│                                                       │
│  + aws_subnet.public                                 │
│      vpc_id: (known after apply)                     │
│                                                       │
│  Plan: 2 to add, 0 to change, 0 to destroy.         │
└──────────────────────────────────────────────────────┘
                          │
                          │ Проверили, всё ОК
                          ▼
┌──────────────────────────────────────────────────────┐
│  terraform apply                                      │
│  (Реально создаёт в AWS)                             │
│                                                       │
│  aws_vpc.main: Creating...                           │
│  aws_vpc.main: Creation complete [id=vpc-abc123]     │
│                                                       │
│  aws_subnet.public: Creating...                      │
│  aws_subnet.public: Creation complete [id=sub-xyz]   │
│                                                       │
│  Apply complete! Resources: 2 added, 0 changed.      │
└──────────────────────────────────────────────────────┘
```

---

## Remote State: Команда работает вместе

```
        DEVELOPER A                    DEVELOPER B
             │                              │
             │ terraform apply              │
             ▼                              │
      ┌────────────┐                       │
      │ Получает   │                       │
      │ LOCK       │                       │
      └─────┬──────┘                       │
            │                              │
            │ Работает с AWS...            │
            │                              │
            │                              │ terraform apply
            │                              ▼
            │                       ┌────────────┐
            │                       │ Ждёт...    │
            │                       │ State      │
            │                       │ locked!    │
            │                       └────────────┘
            │
            │ Готово!
            ▼
      ┌────────────┐
      │ Освобождает│
      │ LOCK       │
      └─────┬──────┘
            │
            │
            ▼
     ┌─────────────┐                      │
     │ S3 State    │                      │
     │ + DynamoDB  │◀─────────────────────┘
     │ Locks       │     Теперь B может работать
     └─────────────┘
```

---

## Практический пример: Staging Environment

```
Файлы:
infra/terraform/live/staging/
├── main.tf          ← Вызывает модули
├── variables.tf     ← Объявляет переменные
├── terraform.tfvars ← Значения для staging
└── backend.hcl      ← S3 конфигурация

┌─────────────────────────────────────────────────────┐
│ main.tf                                              │
│                                                      │
│ module "network" {                                  │
│   source   = "../../modules/network"               │
│   vpc_cidr = var.vpc_cidr                          │
│ }                                                    │
│                                                      │
│ module "eks" {                                      │
│   source     = "../../modules/eks"                 │
│   subnet_ids = module.network.subnet_ids ← output  │
│ }                                                    │
└─────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────┐
│ terraform.tfvars                                     │
│                                                      │
│ environment = "staging"                             │
│ vpc_cidr    = "10.0.0.0/16"                        │
│ db_password = "super-secret"                        │
└─────────────────────────────────────────────────────┘
                          │
                          ▼
              terraform init + plan + apply
                          │
                          ▼
┌─────────────────────────────────────────────────────┐
│ AWS Staging Environment                              │
│                                                      │
│  ┌──────────────────────────────────────────┐      │
│  │ VPC 10.0.0.0/16                          │      │
│  │   ├─ Public Subnets (3 AZ)               │      │
│  │   └─ Private Subnets (3 AZ)              │      │
│  └──────────────────────────────────────────┘      │
│                                                      │
│  ┌──────────────────────────────────────────┐      │
│  │ EKS Cluster                              │      │
│  │   └─ Node Groups (2 nodes)               │      │
│  └──────────────────────────────────────────┘      │
│                                                      │
│  ┌──────────────────────────────────────────┐      │
│  │ RDS PostgreSQL (db.t3.micro)             │      │
│  └──────────────────────────────────────────┘      │
└─────────────────────────────────────────────────────┘
```

---

## Типичные операции

### ➕ Добавить новый ресурс

```
1. Добавить в код:
   resource "aws_s3_bucket" "new" {
     bucket = "my-new-bucket"
   }

2. terraform plan
   → Plan: 1 to add, 0 to change, 0 to destroy

3. terraform apply
   → aws_s3_bucket.new: Creating...
   → aws_s3_bucket.new: Creation complete!
```

### ✏️ Изменить существующий

```
1. Изменить параметр:
   instance_type = "t3.medium" → "t3.large"

2. terraform plan
   → Plan: 0 to add, 1 to change, 0 to destroy
   → ~ instance_type = "t3.medium" -> "t3.large"

3. terraform apply
   → aws_instance.app: Modifying...
   → aws_instance.app: Modifications complete!
```

### ❌ Удалить ресурс

```
1. Удалить из кода:
   # resource "aws_s3_bucket" "old" {...}  ← Закомментировать/удалить

2. terraform plan
   → Plan: 0 to add, 0 to change, 1 to destroy
   → - aws_s3_bucket.old will be destroyed

3. terraform apply
   → aws_s3_bucket.old: Destroying...
   → aws_s3_bucket.old: Destruction complete!
```

---

## Debugging: Что делать если...

### ❌ "Error: Cycle"

```
Проблема: Циклическая зависимость
   A зависит от B
   B зависит от A

Решение: Проверить depends_on и ссылки между ресурсами
```

### ❌ "Error locking state"

```
Проблема: Предыдущий процесс не снял блокировку

Решение:
   terraform force-unlock <LOCK_ID>
```

### ❌ "No valid credential sources"

```
Проблема: AWS credentials не настроены

Решение:
   aws configure
   # Или
   export AWS_ACCESS_KEY_ID="..."
   export AWS_SECRET_ACCESS_KEY="..."
```

---

## Best Practices (одна картинка)

```
✅ DO                           ❌ DON'T

Remote State (S3)              Local State
Версионирование провайдеров    Произвольные версии
Переменные для всего           Hardcoded значения
Модули для переиспользования   Дублирование кода
Секреты в Secrets Manager      Секреты в коде
Теги на всех ресурсах          Ресурсы без тегов
terraform plan перед apply     Сразу apply
Backend.hcl в .gitignore       Backend в Git
DRY принцип                    Copy-paste
```

---

## Заключение

```
           ╔════════════════════════════════════╗
           ║  Terraform в одном предложении:    ║
           ║                                    ║
           ║  Вы описываете ЖЕЛАЕМОЕ состояние  ║
           ║  инфраструктуры в коде,            ║
           ║  Terraform создаёт/изменяет AWS    ║
           ║  ресурсы чтобы достичь этого.      ║
           ╚════════════════════════════════════╝
```

**Следующий шаг:** Прочитайте [TERRAFORM_EXPLAINED.md](./TERRAFORM_EXPLAINED.md) для детального понимания! 🚀

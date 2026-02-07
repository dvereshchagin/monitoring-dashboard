# 🏗️ Как работает Terraform - Подробное объяснение

## 📚 Содержание

1. [Что такое Terraform](#что-такое-terraform)
2. [Основные концепции](#основные-концепции)
3. [Жизненный цикл Terraform](#жизненный-цикл-terraform)
4. [Структура проекта](#структура-проекта)
5. [Работа с состоянием (State)](#работа-с-состоянием-state)
6. [Модули](#модули)
7. [Примеры из проекта](#примеры-из-проекта)
8. [Практическое использование](#практическое-использование)

---

## Что такое Terraform

**Terraform** - это инструмент Infrastructure as Code (IaC) от HashiCorp, который позволяет:

- ✅ Описывать инфраструктуру **декларативно** (что хотим получить, а не как)
- ✅ Управлять облачными ресурсами через **код**
- ✅ Версионировать инфраструктуру в **Git**
- ✅ Создавать **повторяемые** и **предсказуемые** окружения
- ✅ Автоматически отслеживать **зависимости** между ресурсами

### Простая аналогия

Представьте строительство дома:

**Без Terraform (ручной способ):**
```
1. Купить кирпичи вручную
2. Нанять рабочих вручную
3. Построить стены вручную
4. Поставить крышу вручную
5. Провести электричество вручную
...каждый раз одни и те же шаги
```

**С Terraform (автоматизация):**
```terraform
resource "house" "my_home" {
  walls      = 4
  roof       = "tile"
  electricity = true
  
  # Terraform сам знает в каком порядке всё строить
  # Можем построить 10 одинаковых домов одной командой
}
```

---

## Основные концепции

### 1. **Provider** (Провайдер)

Провайдер - это плагин, который знает как работать с конкретным облаком или сервисом.

```terraform
# Провайдер AWS - умеет создавать EC2, S3, VPC и т.д.
provider "aws" {
  region = "us-east-1"
}

# Другие примеры провайдеров:
# provider "google"     - Google Cloud
# provider "azurerm"    - Azure
# provider "kubernetes" - Kubernetes
# provider "helm"       - Helm charts
```

### 2. **Resource** (Ресурс)

Ресурс - это объект инфраструктуры (сервер, база данных, сеть и т.д.)

```terraform
# Синтаксис: resource "тип" "имя_в_коде" { ... }
resource "aws_s3_bucket" "my_bucket" {
  bucket = "my-unique-bucket-name"
  
  tags = {
    Environment = "staging"
  }
}
```

### 3. **Data Source** (Источник данных)

Data Source - это способ **прочитать** существующие данные (не создавать новые).

```terraform
# Получить информацию о существующем VPC
data "aws_vpc" "existing" {
  id = "vpc-12345678"
}

# Теперь можем использовать: data.aws_vpc.existing.cidr_block
```

### 4. **Variable** (Переменная)

Переменные делают код переиспользуемым.

```terraform
# Определение переменной
variable "environment" {
  type        = string
  description = "Environment name (staging, prod)"
  default     = "staging"
}

# Использование
resource "aws_vpc" "main" {
  cidr_block = "10.0.0.0/16"
  
  tags = {
    Environment = var.environment  # var.имя_переменной
  }
}
```

### 5. **Output** (Вывод)

Output - это значения, которые Terraform покажет после apply или которые могут использовать другие модули.

```terraform
output "vpc_id" {
  value       = aws_vpc.main.id
  description = "ID созданного VPC"
}

# После apply покажет: vpc_id = "vpc-abc123"
```

### 6. **Module** (Модуль)

Модуль - это набор ресурсов, упакованных вместе для переиспользования.

```terraform
# Вызов модуля
module "network" {
  source = "./modules/network"  # Путь к модулю
  
  # Передаём параметры
  vpc_cidr    = "10.0.0.0/16"
  environment = "staging"
}

# Используем выходы модуля
resource "aws_instance" "app" {
  subnet_id = module.network.subnet_id  # module.имя.output
}
```

---

## Жизненный цикл Terraform

### Основные команды

```bash
# 1. Инициализация - скачивает провайдеры и модули
terraform init

# 2. Планирование - показывает что будет изменено (не меняет ничего реально)
terraform plan

# 3. Применение - реально создаёт/изменяет/удаляет ресурсы
terraform apply

# 4. Просмотр состояния
terraform show

# 5. Удаление всех ресурсов
terraform destroy
```

### Как это работает внутри?

```
┌─────────────────────────────────────────────────────────┐
│  1. terraform init                                      │
│  - Скачивает провайдер AWS                             │
│  - Скачивает модули                                    │
│  - Готовит backend для хранения состояния              │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│  2. terraform plan                                      │
│  - Читает ваш код (.tf файлы)                         │
│  - Читает текущее состояние (state) из S3             │
│  - Сравнивает желаемое и фактическое                  │
│  - Строит граф зависимостей                           │
│  - Показывает: что создать (+), изменить (~), удалить (-) │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│  3. terraform apply                                     │
│  - Блокирует state через DynamoDB (lock)               │
│  - Выполняет API вызовы к AWS в нужном порядке        │
│  - Обновляет state файл с реальными ID ресурсов       │
│  - Снимает блокировку                                  │
└─────────────────────────────────────────────────────────┘
```

### Пример плана

```hcl
Terraform will perform the following actions:

  # aws_vpc.main will be created
  + resource "aws_vpc" "main" {
      + id         = (known after apply)  # Terraform пока не знает ID
      + cidr_block = "10.0.0.0/16"
    }

  # aws_subnet.public[0] will be created
  + resource "aws_subnet" "public" {
      + id               = (known after apply)
      + vpc_id           = (known after apply)  # Будет взят из aws_vpc.main.id
      + cidr_block       = "10.0.1.0/24"
      + availability_zone = "us-east-1a"
    }

Plan: 2 to add, 0 to change, 0 to destroy.
```

---

## Работа с состоянием (State)

### Что такое State?

**State** - это файл, в котором Terraform хранит **текущее состояние** вашей инфраструктуры.

```json
// terraform.tfstate (упрощённый пример)
{
  "version": 4,
  "resources": [
    {
      "type": "aws_vpc",
      "name": "main",
      "instances": [{
        "attributes": {
          "id": "vpc-abc123",           // ← Реальный ID в AWS
          "cidr_block": "10.0.0.0/16"
        }
      }]
    }
  ]
}
```

### Зачем нужен State?

1. **Связь кода и реальности**: Terraform знает, что ресурс `aws_vpc.main` в коде = `vpc-abc123` в AWS
2. **Отслеживание изменений**: Может понять что изменилось между прошлым и текущим запуском
3. **Управление зависимостями**: Знает в каком порядке создавать/удалять ресурсы
4. **Метаданные**: Хранит дополнительную информацию о ресурсах

### Remote State (Удалённое хранение)

В вашем проекте используется **S3 + DynamoDB** для хранения state:

```terraform
# backend.tf
terraform {
  backend "s3" {
    bucket         = "my-terraform-state"       # S3 bucket для state файла
    key            = "staging/terraform.tfstate" # Путь внутри bucket
    region         = "us-east-1"
    dynamodb_table = "terraform-locks"           # DynamoDB для блокировок
    encrypt        = true                        # Шифрование
  }
}
```

**Преимущества:**
- ✅ Команда работает с одним state (нет конфликтов)
- ✅ Блокировки через DynamoDB (один человек применяет изменения)
- ✅ Версионирование (можно откатиться)
- ✅ Шифрование (безопасность)

### Bootstrap (Создание backend)

В вашем проекте сначала создаётся сам backend:

```terraform
// infra/terraform/bootstrap/main.tf
resource "aws_s3_bucket" "tf_state" {
  bucket = "monitoring-terraform-state"
}

resource "aws_dynamodb_table" "tf_locks" {
  name     = "terraform-locks"
  hash_key = "LockID"
}
```

**Порядок действий:**
```bash
# 1. Сначала создаём S3 и DynamoDB (локальный state)
cd infra/terraform/bootstrap
terraform init
terraform apply

# 2. Теперь другие окружения могут использовать remote state
cd ../live/staging
terraform init -backend-config=backend.hcl
```

---

## Модули

### Что такое модуль?

**Модуль** - это контейнер с набором ресурсов, который можно переиспользовать.

### Структура модуля

```
modules/network/
├── main.tf       # Основная логика (ресурсы)
├── variables.tf  # Входные параметры
└── outputs.tf    # Выходные значения
```

### Пример модуля Network

```terraform
// modules/network/variables.tf
variable "vpc_cidr" {
  type        = string
  description = "CIDR block for VPC"
}

variable "environment" {
  type = string
}
```

```terraform
// modules/network/main.tf
resource "aws_vpc" "this" {
  cidr_block = var.vpc_cidr
  
  tags = {
    Name        = "${var.environment}-vpc"
    Environment = var.environment
  }
}

resource "aws_subnet" "public" {
  vpc_id     = aws_vpc.this.id  # ← Ссылка на ресурс выше
  cidr_block = "10.0.1.0/24"
}
```

```terraform
// modules/network/outputs.tf
output "vpc_id" {
  value = aws_vpc.this.id
}

output "subnet_id" {
  value = aws_subnet.public.id
}
```

### Использование модуля

```terraform
// live/staging/main.tf
module "network" {
  source = "../../modules/network"  # Путь к модулю
  
  # Передаём входные переменные
  vpc_cidr    = "10.0.0.0/16"
  environment = "staging"
}

# Используем outputs модуля
output "staging_vpc_id" {
  value = module.network.vpc_id  # module.имя_модуля.output_имя
}
```

---

## Примеры из проекта

### Пример 1: Создание VPC и подсетей

```terraform
// modules/network/main.tf (упрощённо)

# 1. Создаём VPC
resource "aws_vpc" "this" {
  cidr_block = "10.0.0.0/16"
  
  tags = {
    Name = "monitoring-staging-vpc"
  }
}

# 2. Создаём Internet Gateway (для публичного доступа)
resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id  # ← Зависимость от VPC
}

# 3. Создаём публичные подсети (count = количество)
resource "aws_subnet" "public" {
  count = 3  # Создаст 3 подсети
  
  vpc_id                  = aws_vpc.this.id
  cidr_block              = "10.0.${count.index + 1}.0/24"  # 10.0.1.0/24, 10.0.2.0/24, ...
  availability_zone       = ["us-east-1a", "us-east-1b", "us-east-1c"][count.index]
  map_public_ip_on_launch = true  # Автоматически назначать публичные IP
}

# 4. Создаём приватные подсети
resource "aws_subnet" "private" {
  count = 3
  
  vpc_id            = aws_vpc.this.id
  cidr_block        = "10.0.${count.index + 10}.0/24"  # 10.0.10.0/24, 10.0.11.0/24, ...
  availability_zone = ["us-east-1a", "us-east-1b", "us-east-1c"][count.index]
}
```

**Что происходит при `terraform apply`:**

```
1. Terraform анализирует зависимости:
   - Internet Gateway зависит от VPC
   - Subnets зависят от VPC
   
2. Создаёт в правильном порядке:
   ① aws_vpc.this
   ② aws_internet_gateway.this (параллельно с подсетями)
   ② aws_subnet.public[0,1,2] (параллельно друг с другом)
   ② aws_subnet.private[0,1,2] (параллельно)
   
3. Сохраняет реальные ID в state:
   {
     "aws_vpc.this.id": "vpc-abc123",
     "aws_subnet.public[0].id": "subnet-pub1",
     "aws_subnet.public[1].id": "subnet-pub2",
     ...
   }
```

### Пример 2: Композиция модулей

```terraform
// live/staging/main.tf

# 1. Модуль сети
module "network" {
  source = "../../modules/network"
  
  vpc_cidr    = "10.0.0.0/16"
  environment = "staging"
}

# 2. Модуль EKS (использует выходы модуля network)
module "eks" {
  source = "../../modules/eks"
  
  subnet_ids = module.network.private_subnet_ids  # ← Выход из network
  
  cluster_version = "1.28"
  environment     = "staging"
}

# 3. Модуль RDS (использует выходы network и eks)
module "rds" {
  source = "../../modules/rds"
  
  vpc_id                     = module.network.vpc_id
  subnet_ids                 = module.network.private_subnet_ids
  allowed_security_group_ids = [module.eks.cluster_security_group_id]  # ← Выход из eks
  
  db_name     = "monitoring"
  db_username = "postgres"
  db_password = var.db_password  # Секреты из переменных
}
```

**Граф зависимостей:**

```
module.network
    ↓
    ├── module.eks (зависит от network.subnet_ids)
    ↓
    └── module.rds (зависит от network.vpc_id и eks.security_group_id)
```

### Пример 3: Count и For Each

**Count** - создаёт несколько копий ресурса по индексу:

```terraform
resource "aws_subnet" "public" {
  count = 3  # Создаст [0], [1], [2]
  
  cidr_block = "10.0.${count.index + 1}.0/24"  # count.index = 0, 1, 2
}

# Обращение: aws_subnet.public[0].id, aws_subnet.public[1].id
```

**For Each** - создаёт ресурсы на основе map или set:

```terraform
variable "availability_zones" {
  type = set(string)
  default = ["us-east-1a", "us-east-1b", "us-east-1c"]
}

resource "aws_subnet" "public" {
  for_each = var.availability_zones
  
  availability_zone = each.value  # each.value = "us-east-1a", "us-east-1b", ...
  cidr_block        = cidrsubnet("10.0.0.0/16", 8, index(var.availability_zones, each.value))
}

# Обращение: aws_subnet.public["us-east-1a"].id
```

### Пример 4: Conditional Resources (Условное создание)

```terraform
variable "enable_nat_gateway" {
  type    = bool
  default = true
}

# Создаст NAT Gateway только если enable_nat_gateway = true
resource "aws_nat_gateway" "this" {
  count = var.enable_nat_gateway ? 1 : 0  # Тернарный оператор
  
  allocation_id = aws_eip.nat[0].id
  subnet_id     = aws_subnet.public[0].id
}

# В продакшене: enable_nat_gateway = true  → создаст NAT Gateway
# В staging:    enable_nat_gateway = false → не создаст (экономия $$$)
```

---

## Структура проекта

```
infra/terraform/
│
├── bootstrap/              # Шаг 0: Создание S3 + DynamoDB для state
│   ├── main.tf            # S3 bucket, DynamoDB table
│   ├── variables.tf       # Параметры (bucket name, region)
│   └── outputs.tf         # Вывод созданных ресурсов
│
├── modules/               # Переиспользуемые модули
│   ├── network/          # VPC, Subnets, Internet Gateway, NAT
│   │   ├── main.tf
│   │   ├── variables.tf
│   │   └── outputs.tf
│   │
│   ├── eks/              # EKS Cluster, Node Groups
│   │   ├── main.tf
│   │   ├── variables.tf
│   │   └── outputs.tf
│   │
│   ├── rds/              # PostgreSQL RDS
│   │   ├── main.tf
│   │   ├── variables.tf
│   │   └── outputs.tf
│   │
│   └── ecr/              # Docker Registry
│       ├── main.tf
│       ├── variables.tf
│       └── outputs.tf
│
└── live/                 # Окружения (используют модули)
    ├── staging/          # Staging окружение
    │   ├── main.tf              # Вызовы модулей: network, eks, rds, ecr
    │   ├── variables.tf         # Параметры для staging
    │   ├── terraform.tfvars     # Значения переменных
    │   ├── backend.tf           # Конфигурация S3 backend
    │   ├── backend.hcl          # Параметры backend (не коммитится)
    │   └── providers.tf         # AWS provider config
    │
    └── prod/             # Production окружение
        ├── main.tf              # То же, но для prod
        ├── variables.tf
        ├── terraform.tfvars
        ├── backend.tf
        ├── backend.hcl
        └── providers.tf
```

### Почему такая структура?

1. **DRY принцип**: Модули используются в staging и prod (не дублируем код)
2. **Изоляция окружений**: Staging и prod полностью независимы
3. **Разные параметры**: В staging меньше ресурсов (дешевле), в prod - больше
4. **Безопасность**: Разные AWS аккаунты/регионы для staging и prod

---

## Практическое использование

### Сценарий 1: Первый запуск (Bootstrap)

```bash
# Шаг 1: Создаём backend (S3 + DynamoDB)
cd infra/terraform/bootstrap

# Копируем example и заполняем
cp terraform.tfvars.example terraform.tfvars
nano terraform.tfvars  # Заполняем параметры

# Инициализация и применение
terraform init
terraform plan     # Проверяем что будет создано
terraform apply    # Создаём S3 bucket и DynamoDB table

# Output покажет:
# state_bucket_name = "monitoring-terraform-state"
# lock_table_name   = "terraform-locks"
```

### Сценарий 2: Создание Staging окружения

```bash
cd infra/terraform/live/staging

# Шаг 1: Настройка backend
cp backend.hcl.example backend.hcl
nano backend.hcl

# backend.hcl:
# bucket         = "monitoring-terraform-state"
# key            = "staging/terraform.tfstate"
# region         = "us-east-1"
# dynamodb_table = "terraform-locks"

# Шаг 2: Настройка переменных
cp terraform.tfvars.example terraform.tfvars
nano terraform.tfvars

# terraform.tfvars:
# environment      = "staging"
# vpc_cidr         = "10.0.0.0/16"
# cluster_version  = "1.28"
# db_password      = "strong-password-here"  # НЕ коммитить!

# Шаг 3: Инициализация с backend
terraform init -backend-config=backend.hcl

# Шаг 4: Планирование (проверка)
terraform plan

# Output покажет:
# Plan: 47 to add, 0 to change, 0 to destroy.
#
# module.network.aws_vpc.this will be created
# module.network.aws_subnet.public[0] will be created
# module.eks.aws_eks_cluster.this will be created
# module.rds.aws_db_instance.this will be created
# ...

# Шаг 5: Применение (реально создаёт ресурсы в AWS)
terraform apply

# Terraform спросит: Do you want to perform these actions?
# Type 'yes' и Enter

# Процесс займёт 15-20 минут (EKS кластер создаётся долго)
```

### Сценарий 3: Изменение инфраструктуры

```bash
# Допустим, нужно увеличить размер RDS

# Шаг 1: Редактируем terraform.tfvars
nano terraform.tfvars

# Было:
# db_instance_class = "db.t3.micro"

# Стало:
# db_instance_class = "db.t3.small"

# Шаг 2: Проверяем изменения
terraform plan

# Output покажет:
# module.rds.aws_db_instance.this will be updated in-place
#   ~ instance_class = "db.t3.micro" -> "db.t3.small"
#
# Plan: 0 to add, 1 to change, 0 to destroy.

# Шаг 3: Применяем
terraform apply
```

### Сценарий 4: Просмотр текущего состояния

```bash
# Показать все ресурсы
terraform show

# Показать конкретный ресурс
terraform state show module.network.aws_vpc.this

# Output:
# resource "aws_vpc" "this" {
#     id         = "vpc-abc123"
#     cidr_block = "10.0.0.0/16"
#     ...
# }

# Список всех ресурсов
terraform state list
```

### Сценарий 5: Outputs (получить значения)

```bash
# Показать все outputs
terraform output

# Output:
# eks_cluster_endpoint = "https://ABC123.gr7.us-east-1.eks.amazonaws.com"
# rds_endpoint         = "monitoring-staging.abc123.us-east-1.rds.amazonaws.com:5432"
# vpc_id               = "vpc-abc123"

# Получить конкретный output
terraform output eks_cluster_endpoint

# Использовать в скриптах
EKS_ENDPOINT=$(terraform output -raw eks_cluster_endpoint)
aws eks update-kubeconfig --name $EKS_ENDPOINT
```

---

## Полезные команды

```bash
# Форматирование кода
terraform fmt -recursive

# Валидация синтаксиса
terraform validate

# Просмотр плана без применения
terraform plan -out=tfplan

# Применение сохранённого плана
terraform apply tfplan

# Обновление state без изменения ресурсов
terraform refresh

# Импорт существующего ресурса в state
terraform import aws_vpc.main vpc-abc123

# Удаление конкретного ресурса
terraform destroy -target=module.network.aws_subnet.public[0]

# Разблокировка state (если зависла команда)
terraform force-unlock <LOCK_ID>

# Граф зависимостей (визуализация)
terraform graph | dot -Tsvg > graph.svg
```

---

## Лучшие практики

### 1. **Никогда не коммитить секреты**

```terraform
# ❌ Плохо - секрет в коде
resource "aws_db_instance" "this" {
  password = "hardcoded-password"  # НЕ ДЕЛАЙТЕ ТАК!
}

# ✅ Хорошо - секрет из переменной
variable "db_password" {
  type      = string
  sensitive = true  # Terraform не покажет в логах
}

resource "aws_db_instance" "this" {
  password = var.db_password
}
```

```bash
# Передача через переменную окружения
export TF_VAR_db_password="secret"
terraform apply

# Или через файл (не коммитить!)
echo 'db_password = "secret"' > secrets.auto.tfvars
terraform apply
```

### 2. **Использовать remote state**

```terraform
# backend.tf
terraform {
  backend "s3" {
    bucket         = "my-terraform-state"
    key            = "staging/terraform.tfstate"
    region         = "us-east-1"
    dynamodb_table = "terraform-locks"
    encrypt        = true
  }
}
```

### 3. **Версионировать провайдеры**

```terraform
terraform {
  required_version = ">= 1.6.0"  # Минимальная версия Terraform
  
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"  # Любая 5.x версия
    }
  }
}
```

### 4. **Использовать workspaces для окружений (альтернатива)**

```bash
# Создать workspace
terraform workspace new staging
terraform workspace new prod

# Переключиться
terraform workspace select staging
terraform apply

# Текущий workspace
terraform workspace show
```

### 5. **Документировать код**

```terraform
variable "vpc_cidr" {
  type        = string
  description = "CIDR block for VPC (e.g., 10.0.0.0/16)"
  
  validation {
    condition     = can(cidrhost(var.vpc_cidr, 0))
    error_message = "Must be valid IPv4 CIDR."
  }
}
```

---

## Отличие от других инструментов

| Инструмент | Подход | Когда использовать |
|------------|--------|-------------------|
| **Terraform** | Декларативный, multi-cloud | Управление инфраструктурой (VPC, EC2, RDS) |
| **Ansible** | Императивный, конфигурация | Настройка серверов (установка софта) |
| **CloudFormation** | Декларативный, только AWS | Если используете только AWS |
| **Pulumi** | Императивный, реальные языки (Python, Go) | Если предпочитаете Python/Go вместо HCL |
| **Kubernetes** | Декларативный, только K8s | Управление приложениями в кластере |

**В вашем проекте:**
- **Terraform** создаёт VPC, EKS, RDS (инфраструктура)
- **Helm/Kubernetes** деплоит приложение в EKS (приложения)
- **GitHub Actions** автоматизирует процесс (CI/CD)

---

## Резюме

### Terraform в одном абзаце:

Terraform позволяет описать инфраструктуру как код. Вы пишете `.tf` файлы с описанием желаемого состояния (VPC, подсети, кластеры), Terraform сравнивает это с текущим состоянием (state), строит граф зависимостей и выполняет необходимые API вызовы к AWS для создания/изменения/удаления ресурсов в правильном порядке.

### Ключевые преимущества:

✅ **Версионируемая инфраструктура** - в Git  
✅ **Повторяемость** - staging = prod (с разными параметрами)  
✅ **Автоматизация** - нет ручных кликов в консоли  
✅ **Предсказуемость** - `plan` показывает изменения до применения  
✅ **Командная работа** - remote state + блокировки  
✅ **Multi-cloud** - AWS, GCP, Azure одним инструментом  

### Следующие шаги:

1. Попробуйте запустить bootstrap
2. Создайте staging окружение
3. Измените параметры и посмотрите на `plan`
4. Почитайте [официальную документацию](https://developer.hashicorp.com/terraform/docs)

Удачи! 🚀

# 📝 Terraform - Краткая шпаргалка

## Основные команды

```bash
# Инициализация (скачивает провайдеры)
terraform init

# Планирование (показывает изменения)
terraform plan

# Применение (создаёт ресурсы)
terraform apply

# Применение без подтверждения
terraform apply -auto-approve

# Удаление всех ресурсов
terraform destroy

# Форматирование кода
terraform fmt -recursive

# Валидация синтаксиса
terraform validate

# Показать текущее состояние
terraform show

# Показать outputs
terraform output

# Список ресурсов в state
terraform state list

# Показать конкретный ресурс
terraform state show aws_vpc.main
```

---

## Синтаксис

### Resource (Создание ресурса)

```terraform
resource "тип_ресурса" "имя_в_коде" {
  параметр1 = "значение"
  параметр2 = 123
}

# Пример
resource "aws_s3_bucket" "my_bucket" {
  bucket = "my-unique-bucket-name"
}
```

### Data Source (Чтение существующего)

```terraform
data "тип_источника" "имя" {
  фильтр = "значение"
}

# Пример
data "aws_vpc" "existing" {
  id = "vpc-12345"
}
```

### Variable (Переменная)

```terraform
variable "имя" {
  type        = string
  description = "Описание"
  default     = "значение по умолчанию"
}

# Использование
resource "aws_instance" "app" {
  instance_type = var.имя
}
```

### Output (Вывод значения)

```terraform
output "имя" {
  value       = resource.type.name.attribute
  description = "Описание"
}
```

### Module (Модуль)

```terraform
module "имя_модуля" {
  source = "./путь/к/модулю"
  
  параметр1 = "значение"
  параметр2 = var.переменная
}

# Использование output модуля
resource "другой_ресурс" "пример" {
  param = module.имя_модуля.output_имя
}
```

---

## Ссылки между ресурсами

```terraform
# Создать VPC
resource "aws_vpc" "main" {
  cidr_block = "10.0.0.0/16"
}

# Создать подсеть (зависит от VPC)
resource "aws_subnet" "public" {
  vpc_id     = aws_vpc.main.id  # ← Ссылка на VPC
  cidr_block = "10.0.1.0/24"
}

# Terraform автоматически поймёт порядок:
# 1. Сначала создаст VPC
# 2. Потом создаст Subnet
```

---

## Типы данных

```terraform
# String (строка)
variable "name" {
  type    = string
  default = "hello"
}

# Number (число)
variable "count" {
  type    = number
  default = 5
}

# Bool (булево)
variable "enabled" {
  type    = bool
  default = true
}

# List (список)
variable "zones" {
  type    = list(string)
  default = ["us-east-1a", "us-east-1b"]
}

# Map (словарь)
variable "tags" {
  type = map(string)
  default = {
    Environment = "staging"
    Project     = "monitoring"
  }
}

# Object (объект)
variable "config" {
  type = object({
    name    = string
    port    = number
    enabled = bool
  })
}
```

---

## Условия и циклы

### Count (создать N копий)

```terraform
resource "aws_subnet" "public" {
  count = 3  # Создаст [0], [1], [2]
  
  cidr_block = "10.0.${count.index + 1}.0/24"
}

# Обращение:
# aws_subnet.public[0].id
# aws_subnet.public[1].id
# aws_subnet.public[2].id
```

### For Each (для map/set)

```terraform
variable "users" {
  type    = set(string)
  default = ["alice", "bob", "charlie"]
}

resource "aws_iam_user" "users" {
  for_each = var.users
  
  name = each.value  # each.value = "alice", "bob", ...
}

# Обращение:
# aws_iam_user.users["alice"].arn
```

### Условный ресурс (if/else)

```terraform
variable "create_backup" {
  type    = bool
  default = true
}

# Создать только если create_backup = true
resource "aws_db_snapshot" "backup" {
  count = var.create_backup ? 1 : 0  # Тернарный оператор
  
  db_instance_identifier = aws_db_instance.main.id
}
```

### For Expression (преобразование)

```terraform
variable "names" {
  default = ["alice", "bob", "charlie"]
}

# Преобразовать в uppercase
locals {
  uppercase_names = [for name in var.names : upper(name)]
  # Результат: ["ALICE", "BOB", "CHARLIE"]
}

# Фильтрация
locals {
  long_names = [for name in var.names : name if length(name) > 3]
  # Результат: ["alice", "charlie"]
}
```

---

## Locals (локальные переменные)

```terraform
locals {
  # Вычисляемые значения
  environment = "staging"
  name_prefix = "${var.project_name}-${local.environment}"
  
  # Общие теги
  common_tags = {
    Project     = var.project_name
    Environment = local.environment
    ManagedBy   = "terraform"
  }
}

# Использование
resource "aws_vpc" "main" {
  tags = local.common_tags
}
```

---

## Функции (часто используемые)

```terraform
# Строки
upper("hello")           # "HELLO"
lower("WORLD")           # "world"
replace("hello", "l", "L") # "heLLo"
format("Hello %s", "World") # "Hello World"

# Числа
max(1, 5, 3)            # 5
min(1, 5, 3)            # 1
abs(-5)                 # 5

# Списки
length([1, 2, 3])       # 3
concat([1, 2], [3, 4])  # [1, 2, 3, 4]
element([1, 2, 3], 1)   # 2 (индекс 1)

# Map
merge({a = 1}, {b = 2}) # {a = 1, b = 2}
keys({a = 1, b = 2})    # ["a", "b"]
values({a = 1, b = 2})  # [1, 2]

# CIDR
cidrsubnet("10.0.0.0/16", 8, 1) # "10.0.1.0/24"

# Файлы
file("path/to/file.txt")        # Прочитать файл
filebase64("image.png")         # Base64

# JSON/YAML
jsondecode('{"key": "value"}')  # Парсинг JSON
yamldecode("key: value")        # Парсинг YAML
```

---

## Backend (Remote State)

```terraform
terraform {
  backend "s3" {
    bucket         = "my-terraform-state"
    key            = "env/terraform.tfstate"
    region         = "us-east-1"
    dynamodb_table = "terraform-locks"
    encrypt        = true
  }
}
```

```bash
# Инициализация с backend
terraform init -backend-config=backend.hcl

# Переконфигурация backend
terraform init -reconfigure

# Миграция state
terraform init -migrate-state
```

---

## Depends On (явная зависимость)

```terraform
resource "aws_instance" "app" {
  ami           = "ami-123"
  instance_type = "t2.micro"
  
  # Явная зависимость (хотя Terraform обычно сам понимает)
  depends_on = [aws_security_group.app]
}
```

---

## Lifecycle (управление жизненным циклом)

```terraform
resource "aws_instance" "app" {
  ami = "ami-123"
  
  lifecycle {
    # Создать новый ресурс перед удалением старого
    create_before_destroy = true
    
    # Не удалять ресурс при terraform destroy
    prevent_destroy = true
    
    # Игнорировать изменения в этих атрибутах
    ignore_changes = [
      tags,
      user_data
    ]
  }
}
```

---

## Provisioners (выполнение команд) - Редко используются

```terraform
resource "aws_instance" "app" {
  ami = "ami-123"
  
  # Выполнить локально после создания
  provisioner "local-exec" {
    command = "echo ${self.private_ip} >> private_ips.txt"
  }
  
  # Выполнить на удалённой машине
  provisioner "remote-exec" {
    inline = [
      "sudo apt update",
      "sudo apt install -y nginx"
    ]
  }
}
```

---

## Импорт существующих ресурсов

```bash
# Импортировать существующий ресурс в state
terraform import aws_vpc.main vpc-abc123

# После импорта нужно написать код для этого ресурса
```

---

## Debugging

```bash
# Включить подробные логи
export TF_LOG=DEBUG
terraform apply

# Логи в файл
export TF_LOG=DEBUG
export TF_LOG_PATH=terraform.log
terraform apply

# Уровни логирования: TRACE, DEBUG, INFO, WARN, ERROR
```

---

## Переменные окружения

```bash
# Установить переменную Terraform
export TF_VAR_имя_переменной="значение"

# Пример
export TF_VAR_db_password="secret123"
terraform apply  # Использует эту переменную
```

---

## Workspace (изоляция окружений)

```bash
# Список workspaces
terraform workspace list

# Создать workspace
terraform workspace new staging
terraform workspace new prod

# Переключиться
terraform workspace select staging

# Текущий
terraform workspace show

# Удалить
terraform workspace delete staging
```

---

## Полезные паттерны

### Тэгирование всех ресурсов

```terraform
locals {
  common_tags = {
    Project     = "monitoring"
    Environment = var.environment
    ManagedBy   = "terraform"
    CreatedAt   = timestamp()
  }
}

resource "aws_vpc" "main" {
  tags = merge(local.common_tags, {
    Name = "main-vpc"
  })
}
```

### Naming Convention

```terraform
locals {
  name_prefix = "${var.project_name}-${var.environment}"
}

resource "aws_vpc" "main" {
  tags = {
    Name = "${local.name_prefix}-vpc"
  }
}

resource "aws_subnet" "public" {
  tags = {
    Name = "${local.name_prefix}-public-subnet"
  }
}
```

---

## Ошибки и решения

### "Error locking state"

```bash
# Снять блокировку (если процесс упал)
terraform force-unlock <LOCK_ID>
```

### "No valid credential sources found"

```bash
# Настроить AWS credentials
aws configure

# Или экспортировать переменные
export AWS_ACCESS_KEY_ID="..."
export AWS_SECRET_ACCESS_KEY="..."
```

### "Error: Cycle"

```
Циклическая зависимость между ресурсами.
Проверьте ссылки: resource A → resource B → resource A
```

---

## Лучшие практики

✅ **Версионируйте провайдеры** (`required_providers`)  
✅ **Используйте remote state** (S3 + DynamoDB)  
✅ **Не коммитьте секреты** (используйте variables)  
✅ **Форматируйте код** (`terraform fmt`)  
✅ **Делайте plan перед apply**  
✅ **Используйте modules** для переиспользования  
✅ **Документируйте переменные** (description)  
✅ **Тегируйте все ресурсы**  
✅ **Используйте `.gitignore`**:

```gitignore
# .gitignore
**/.terraform/*
*.tfstate
*.tfstate.*
terraform.tfvars
*.auto.tfvars
backend.hcl
```

---

## Ресурсы для изучения

- [Официальная документация](https://developer.hashicorp.com/terraform/docs)
- [AWS Provider](https://registry.terraform.io/providers/hashicorp/aws/latest/docs)
- [Terraform Registry](https://registry.terraform.io/) - готовые модули
- [Learn Terraform](https://developer.hashicorp.com/terraform/tutorials)

---

Удачи! 🚀

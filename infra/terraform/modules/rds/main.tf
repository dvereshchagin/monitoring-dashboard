locals {
  name_prefix = "${var.project_name}-${var.environment}"
}

resource "aws_db_subnet_group" "this" {
  name       = "${locals.name_prefix}-db-subnet-group"
  subnet_ids = var.subnet_ids

  tags = merge(var.tags, {
    Name = "${locals.name_prefix}-db-subnet-group"
  })
}

resource "aws_security_group" "this" {
  name        = "${locals.name_prefix}-rds-sg"
  description = "Security group for ${locals.name_prefix} RDS"
  vpc_id      = var.vpc_id

  tags = merge(var.tags, {
    Name = "${locals.name_prefix}-rds-sg"
  })
}

resource "aws_vpc_security_group_ingress_rule" "from_eks" {
  for_each = toset(var.allowed_security_group_ids)

  security_group_id            = aws_security_group.this.id
  referenced_security_group_id = each.value
  from_port                    = 5432
  to_port                      = 5432
  ip_protocol                  = "tcp"
  description                  = "PostgreSQL access from trusted security groups"
}

resource "aws_vpc_security_group_egress_rule" "all" {
  security_group_id = aws_security_group.this.id
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
  description       = "Allow all outbound traffic"
}

resource "aws_db_instance" "this" {
  identifier                 = "${locals.name_prefix}-postgres"
  db_name                    = var.db_name
  username                   = var.db_username
  password                   = var.db_password
  engine                     = "postgres"
  engine_version             = "16.4"
  instance_class             = var.instance_class
  allocated_storage          = var.allocated_storage
  max_allocated_storage      = var.max_allocated_storage
  storage_encrypted          = true
  multi_az                   = var.multi_az
  backup_retention_period    = var.backup_retention_period
  db_subnet_group_name       = aws_db_subnet_group.this.name
  vpc_security_group_ids     = [aws_security_group.this.id]
  skip_final_snapshot        = false
  final_snapshot_identifier  = "${locals.name_prefix}-postgres-final"
  deletion_protection        = var.deletion_protection
  publicly_accessible        = false
  auto_minor_version_upgrade = true

  tags = merge(var.tags, {
    Name = "${locals.name_prefix}-postgres"
  })
}

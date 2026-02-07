variable "repository_name" {
  type = string
}

variable "image_tag_mutability" {
  type    = string
  default = "MUTABLE"
}

variable "scan_on_push" {
  type    = bool
  default = true
}

variable "lifecycle_max_images" {
  type    = number
  default = 30
}

variable "tags" {
  type    = map(string)
  default = {}
}

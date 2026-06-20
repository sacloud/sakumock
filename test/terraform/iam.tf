resource "sakura_iam_folder" "test" {
  name        = "sakumock-tf-folder"
  description = "description"
}

resource "sakura_iam_project" "test" {
  name             = "sakumock-tf-project"
  code             = "sakumock-tf-project"
  description      = "description"
  parent_folder_id = sakura_iam_folder.test.id
}

resource "sakura_iam_user" "test" {
  name                = "sakumock-tf-user"
  code                = "sakumock-tf-user"
  password_wo         = "Password12345!"
  password_wo_version = 1
  description         = "description"
}

resource "sakura_iam_group" "test" {
  name        = "sakumock-tf-group"
  description = "description"
}

resource "sakura_iam_service_principal" "test" {
  name        = "sakumock-tf-sp"
  project_id  = sakura_iam_project.test.id
  description = "description"
}

resource "sakura_iam_project_apikey" "test" {
  name        = "sakumock-tf-apikey"
  project_id  = sakura_iam_project.test.id
  iam_roles   = ["resource-creator"]
  description = "description"
}

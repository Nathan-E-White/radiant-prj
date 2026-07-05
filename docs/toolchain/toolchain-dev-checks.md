

## 1. Declare the toolchain

* Add a repo-local tool manifest, `.mise.toml`, for:
  * bun, 
  * node, 
  * go, 
  * rust, 
  * terraform

* For Ansible, either uv/pipx with a pinned requirements-dev.txt

## 2. Doctor check

* Add `scripts/doctor.mjs` script that calls helpers:
  * `scripts/doctor.bun.mjs`
  * `scripts/doctor.node.mjs`
  * `scripts/doctor.go.mjs`
  * `scripts/doctor.rust.mjs`
  * `scripts/doctor.terraform.mjs`
  * `scripts/doctor.ansible.mjs`
  * `scripts/doctor.docker.mjs`

## 3. Split verification levels

Right now `infra:check` quietly says “terraform not available” and still passes. That is okay for static checks, but release language needs two modes:
1. `infra:check`: static artifact validation, passes without Terraform/Ansible
2. `infra:native-check`: requires Terraform/Ansible and fails if missing

And two integration paths:
1. `ci`: normal dev/CI path
2. `ci:full`: includes native infra and compose smoke tests

## Package manager:

Cleanup the package managers. We have `bun.lock` and `package-lock.json`, but the README/CI say Bun. Pick Bun as primary, keep `package-lock.json` out for now.
#!/usr/bin/env bash

inp="${1}";
if [[ ! ${inp} ]]; then
  exit
fi

# 1. Create the OpenTofu plugin registry layout
mkdir -p ~/.opentofu/plugins/registry.opentofu.org/local/customdocker/1.0.0/darwin_arm64/;

# 2. Compile and move your binary directly into it
go build -o terraform-provider-customdocker_v1.0.0
mv terraform-provider-customdocker_v1.0.0 ~/.opentofu/plugins/registry.opentofu.org/local/customdocker/1.0.0/darwin_arm64/;

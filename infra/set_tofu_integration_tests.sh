#!/usr/bin/env bash

# Force the framework to drive tests using OpenTofu
export TF_ACC=1;
export TF_ACC_PROVIDER_BINARY_PROGRAM=tofu;

go test -v -run=TestAccContainerResource

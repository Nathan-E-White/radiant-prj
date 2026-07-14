# Docker test-dependency compatibility module

Apache Iceberg Go v0.6.0 requires the vulnerable legacy
`github.com/docker/docker` module only from dependency-test helpers. Radiant does
not compile or run those helpers. This local replacement supplies their package
paths to Go's module resolver without selecting or shipping the vulnerable
Docker implementation.

Radiant runtime code uses the supported split `github.com/moby/moby/api` and
`github.com/moby/moby/client` modules. Remove this replacement when Iceberg Go
stops requiring the legacy module.

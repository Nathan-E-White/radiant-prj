# CodeQL Alert Remediation Research

## Scope

This note supports the remediation plan for GitHub code-scanning alerts 1 and 2:

- `actions/missing-workflow-permissions` in `.github/workflows/ci.yml`
- `go/disabled-certificate-check` in `backend/slurm-gateway/cmd/simops-webtransport-probe/main.go`

## Change Control

Affected configuration items:

- CI-CI: `.github/workflows/ci.yml` now declares read-only workflow token permissions.
- CI-SRC: backend WebTransport TLS loading and probe dialing move into verified Go code.
- CI-INF: `deploy/slurm-gateway.Dockerfile` and `deploy/slurm-gateway.compose.yml` carry the TLS and dataplane wiring needed by local Simulation Ops smoke tests.
- CI-SCR: local certificate generation plus Compose and Simulation Ops smoke scripts now refresh ignored `.local/compose-secrets` material before validation.
- CI-DOC: this research note, `docs/design/interface-control.md`, and `docs/toolchain/toolchain-dev-checks.md` record the trust-material and verification behavior.

Verification activities for this change are `bun run backend:test`, `bun run infra:check`, `bun run quality:check`, `bun run compose:smoke`, and `bun run simops:smoke:local --timeout 300`.

The `.local/certs` and `.local/compose-secrets` files are local ignored trust material. They are generated inputs for smoke validation, not controlled release records.

## Findings

GitHub Actions workflows should declare the least `GITHUB_TOKEN` permissions they need. GitHub documents that an action can access `github.token` even if the workflow does not explicitly pass the token, and recommends limiting the token to the minimum required access. CodeQL's missing-workflow-permissions query warns that workflows without explicit permissions inherit repository or organization defaults, and recommends a workflow or job `permissions` block with least privilege.

The Radiant CI workflow checks out code and runs Bun/Go validation. It does not publish packages, write issues, create releases, upload security events, or update pull requests. The minimal permission needed for checkout and read-only validation is `contents: read`.

Primary sources:

- GitHub `GITHUB_TOKEN` docs: https://docs.github.com/en/actions/tutorials/authenticate-with-github_token
- GitHub workflow `permissions` syntax: https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#permissions
- CodeQL Actions query help: https://codeql.github.com/codeql-query-help/actions/actions-missing-workflow-permissions/

Go's `tls.Config.InsecureSkipVerify` disables normal server certificate chain and hostname verification. CodeQL's disabled-certificate-check query recommends not setting it to true outside tests. The Go TLS package documents `RootCAs` as the client root set used for server verification and `ServerName` as the hostname used for certificate verification when the client needs an override.

The existing WebTransport smoke path used an ephemeral self-signed certificate and dialed `https://simops-moq-gateway:9443/moq/simops` from inside Compose. Proper verification therefore requires two repo-specific pieces: the local server certificate must include `simops-moq-gateway` in its SANs, and the probe must trust the local CA while preserving HTTP/3 ALPN.

Primary sources:

- CodeQL Go query help: https://codeql.github.com/codeql-query-help/go/go-disabled-certificate-check/
- Go `crypto/tls.Config`: https://pkg.go.dev/crypto/tls#Config
- `webtransport-go` Dialer docs: https://pkg.go.dev/github.com/quic-go/webtransport-go#Dialer

Docker Compose secrets are file-mounted under `/run/secrets/<name>` and granted per service. Docker's current Compose reference notes that uid/gid/mode remapping is not implemented for file-backed secrets because Compose uses bind mounts under the hood. Radiant should therefore keep source certificate material ignored under `.local/certs` and create ignored, smoke-readable copies under `.local/compose-secrets` for the non-root MoQ gateway container.

Primary sources:

- Docker Compose secrets guide: https://docs.docker.com/compose/how-tos/use-secrets/
- Docker Compose service secrets reference: https://docs.docker.com/reference/compose-file/services/#secrets

## Implementation Implications

- Add root-level workflow `permissions: contents: read`.
- Remove `InsecureSkipVerify` entirely from production code.
- Add CA-backed WebTransport client TLS configuration and configured server certificate loading behind small internal gateway functions.
- Extend local certificate generation with the `simops-moq-gateway` DNS name.
- Refresh ignored `.local/compose-secrets` copies whenever local certs already exist.
- Keep browser-facing surfaces free of certificate, broker, database, object-storage, Docker, or Iceberg credentials.

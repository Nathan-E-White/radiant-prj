# Docker to Podman and Buildah Migration Research

| Field | Value |
| --- | --- |
| Document ID | CONTAINER-TOOLCHAIN-MIGRATION-RESEARCH-001 |
| Status | Research note; no implementation change |
| Scope | This repository's Docker/Buildx local development, packaging, Compose, CI, and Docker-SDK runtime path |
| Last updated | 2026-07-30 |

## Conclusion

Podman and Buildah can replace most of this repository's *image build, image
run, registry, and Compose-validation* work, while preserving the existing
Dockerfiles. They are not a safe transparent replacement for the repository's
runtime control plane: the gateway mounts `/var/run/docker.sock` and calls the
Moby Docker API from Go. Podman's service provides a Docker v1.40 compatibility
layer, but that is an API subset and has a different socket, identity, storage,
and security model. Treat that bridge as a short compatibility experiment, not
as the migration's final architecture. [Podman service](https://docs.podman.io/en/latest/markdown/podman-system-service.1.html)

The defensible target is therefore: use Buildah or `podman build` for the five
repository Dockerfiles, publish OCI-compatible images to the existing registry,
use a deliberately selected external Compose provider for local stacks, and
replace the gateway's Docker-SDK implementation with a runtime-neutral worker
boundary (preferably the existing Kubernetes path where a scheduler is needed).
This removes the privileged engine socket from the application architecture;
renaming the socket is otherwise a rather expensive way to retain the same
security problem.

This note relies on local repository sources for its inventory and primary
upstream Podman, Buildah, and OCI specifications for platform claims. It does
not certify a specific Podman, Buildah, Compose-provider, GitHub runner, or
registry version. Pin and test those versions before a production cutover.

## Repository inventory and migration classification

| Area | Current repository evidence | Migration consequence |
| --- | --- | --- |
| Image definitions | Five Dockerfiles: `Dockerfile`, `worker.Dockerfile`, `deploy/slurm-gateway.Dockerfile`, `deploy/simops-generator.Dockerfile`, and `deploy/scada-standins.Dockerfile`; all are multi-stage builds. | Retain initially; both Podman and Buildah accept a file named `Dockerfile` and the Containerfile/Dockerfile instruction syntax. [Podman build](https://docs.podman.io/en/stable/markdown/podman-build.1.html), [Buildah build](https://github.com/containers/buildah/blob/main/docs/buildah-build.1.md) |
| Packaging | `docker-bake.hcl` defines 12 packaging targets; `scripts/verify-docker-packaging.mjs` runs `docker buildx bake --load`, uses BuildKit cache exporters, parses Buildx output, queries `docker system df`, then runs and inspects images. | This is the highest build-tool rewrite: Buildah has no Buildx Bake interpreter, so replace the Bake plan with explicit per-target Buildah/Podman commands or retain Bake only during transition. Do not silently downgrade cache, target, platform, evidence, or size-budget checks. |
| Local Compose | `docker-compose.yml` and `deploy/slurm-gateway.compose.yml` define a small console stack and a larger 15-service SimOps stack, including profiles, health checks, secrets, named volumes, UDP/TCP ports, read-only roots, tmpfs, and `depends_on`. | `podman compose` delegates to an **external** Compose provider, rather than implementing Compose itself. Select and version the provider, then validate the two files feature-by-feature. [Podman compose](https://docs.podman.io/en/v5.6.2/markdown/podman-compose.1.html) |
| Runtime socket | `deploy/slurm-gateway.compose.yml` runs the gateway as `0:0` and mounts `/var/run/docker.sock`; `backend/slurm-gateway/internal/simopsdocker/` uses `github.com/moby/moby/client` to inspect images and create/start/list/inspect/stop/remove workers. | This cannot become rootless merely by changing `DOCKER_HOST`. A process that can use an engine API can run arbitrary code as that engine's user; remove the socket for the steady state. [Podman service security](https://docs.podman.io/en/latest/markdown/podman-system-service.1.html) |
| Runtime choices | The gateway already has separate Docker and Kubernetes spoolers, selected by `SIMOPS_WORKER_RUNTIME`; the Kubernetes adapter is instantiated in `cmd/server/main.go`. | The Kubernetes adapter is the natural replacement for scheduler-managed worker jobs. A Podman adapter is reasonable only for bounded local development, behind the existing runtime seam. |
| Tooling and CI | `scripts/docker-up.sh`, `scripts/docker-down.sh`, Compose/smoke scripts, packaging verification, and Docker hygiene/storage checks are Docker-named; `.github/workflows/docker-packaging-publish.yml` installs Docker Buildx and uses Docker actions plus `type=gha` cache settings. | Separate the local-engine migration from CI image publication. A Buildah pipeline needs its own registry auth, cache strategy, artifact/evidence format, and runner capability test. |

The inventory is intentionally conservative: a text replacement of `docker` with
`podman` would miss the embedded client, Buildx-specific parser and cache
policy, Docker-context selection, and the mounted control socket. These are
different businesses wearing the same trench coat.

## Compatibility boundaries

### CLI and image format

For basic lifecycle operations (`pull`, `build`, `run`, `exec`, `ps`, `logs`,
`inspect`, `tag`, and `push`), Podman's CLI is deliberately familiar. Its build
command accepts Dockerfiles, recognizes `.dockerignore` (or `.containerignore`),
and documents `buildx build` only as a scripting-compatibility option. That is
useful for simple scripts, but does **not** make `docker buildx bake` or the
repository's BuildKit cache/export log format a supported Podman contract.
[Podman build](https://docs.podman.io/en/stable/markdown/podman-build.1.html)

Buildah's `build`/`bud` commands build from a Dockerfile or Containerfile, with
OCI as the default manifest/configuration format and an option to request Docker
schema-2 output. OCI images are the preferred common denominator: the OCI
Image Manifest defines a content-addressable image configuration and layers,
and an OCI image index represents platform-specific manifests. [Buildah
build](https://github.com/containers/buildah/blob/main/docs/buildah-build.1.md),
[OCI image manifest](https://github.com/opencontainers/image-spec/blob/main/manifest.md)

Initial build equivalence should look like this, with targets enumerated by the
repository's replacement packaging script rather than inferred from Bake:

```sh
# normal development build; Dockerfile names need not change
podman build -f deploy/simops-generator.Dockerfile \
  -t radiant-simops-generator:dev .

# CI-oriented explicit builder; OCI is Buildah's default format
buildah bud -f deploy/simops-generator.Dockerfile \
  -t registry.example/radiant/simops-generator:sha .
buildah push --digestfile image.digest \
  registry.example/radiant/simops-generator:sha
```

The exact target name above is illustrative; the migration script must read the
actual stages currently used by `docker-bake.hcl`. Buildah can push to a
registry and write the resulting digest, and Podman can push either OCI or
Docker v2 schema formats. Preserve digest evidence, image size inspection, and
post-build behavioural probes before retiring the current packaging gate.
[Podman push](https://docs.podman.io/en/stable/markdown/podman-push.1.html)

### Rootless operation, networking, and volumes

On Linux, rootless Podman creates a user namespace and requires subordinate UID
and GID mappings for normal multi-ID operation; rootless and rootful stores are
separate, so images, containers, and volumes are not automatically shared.
Rootless networking uses `pasta` by default (or `slirp4netns` when configured),
and rootless storage has filesystem constraints such as older OverlayFS kernels
and distributed filesystems. This is an improvement in host privilege exposure,
but it is a behavioural change that needs an explicit local-development test
matrix. [Podman rootless mode](https://docs.podman.io/en/latest/markdown/podman.1.html),
[Podman networking](https://docs.podman.io/en/v5.4.2/markdown/podman-network.1.html)

The repository's named data volumes (`postgres-data`, `redpanda-data`,
`minio-data`) should be treated as disposable local fixtures unless a migration
explicitly exports and restores them. Do not point rootless Podman at an
existing Docker volume directory or rely on a matching name. For bind mounts on
SELinux hosts, use `:z` for shared content or `:Z` for private content only
where relabelling is intended; Podman documents that relabelling system paths is
unsafe and can be slow for large trees. [Podman volume mounts](https://docs.podman.io/en/latest/markdown/podman-run.1.html)

The existing `no-new-privileges`, `read_only`, and `tmpfs` declarations should
remain acceptance tests, not be dropped to make a provider start. Podman warns
that `--privileged` disables several isolation mechanisms, and rootless
containers cannot gain more privilege than their launching user. The gateway's
current root user plus engine socket is therefore the exceptional risk to
remove, not a template for the target stack. [Podman run security](https://docs.podman.io/en/latest/markdown/podman-run.1.html)

### Socket and API compatibility

Podman is daemonless for normal CLI use, but `podman system service` exposes a
REST service containing a Docker v1.40 compatibility layer and Podman's own
Libpod API. The rootless default Unix socket is
`$XDG_RUNTIME_DIR/podman/podman.sock`; the rootful default is
`/run/podman/podman.sock`, not `/var/run/docker.sock`. The service documents
that socket access gives complete Podman control as the service user and
recommends Unix permissions, or mutual TLS for remote TCP access. [Podman
service](https://docs.podman.io/en/latest/markdown/podman-system-service.1.html)

This permits a contained compatibility pilot: run the gateway on the host (not
inside a socket-mounting container), point `DOCKER_HOST` at a rootless Podman
socket, and exercise the exact Moby methods in `simopsdocker`. The pilot must
cover image inspection; create with labels, network, read-only root, tmpfs, and
auto-remove; list filters; inspect state; stop; remove; and the current
integration smoke. A passing happy-path create is insufficient evidence that
the Moby SDK/API version negotiation works for this code path.

Do **not** mount the rootless socket into `slurm-gateway` as the final design.
The Podman documentation specifically calls out mounting the socket into a
container and disabling SELinux labelling to access it; this is an explicit
privilege boundary, not an ordinary data volume. Prefer the existing
Kubernetes-runtime branch for managed worker execution, or implement a narrow
host-local Podman adapter that is available only in local developer mode.
[Podman service: container socket access](https://docs.podman.io/en/latest/markdown/podman-system-service.1.html)

### Compose migration choices

`podman compose` is a thin wrapper: it discovers an external provider, normally
`docker-compose` or `podman-compose`, passes the command through, and configures
that provider to use the Podman socket. `docker-compose` takes precedence when
both are installed. Consequently, `podman compose` alone does not give this
repository a single implementation or a portable support promise. Pin one
provider in developer setup/CI and record its version and known Compose-spec
coverage. [Podman compose](https://docs.podman.io/en/v5.6.2/markdown/podman-compose.1.html)

Recommended decision sequence:

1. During discovery, run the current `config --quiet` checks through the chosen
   provider against both compose files and retain the rendered configuration as
   evidence.
2. Pilot `podman compose` with a pinned `docker-compose` provider against the
   Podman API first, because the repository already relies on Compose syntax and
   this is the provider Podman documents as original/widely used. This is a
   compatibility bridge, not an assertion that every Docker Desktop feature is
   identical. [Podman compose](https://docs.podman.io/en/v5.6.2/markdown/podman-compose.1.html)
3. Separately assess a pinned `podman-compose` provider only if removing the
   Docker Compose dependency is a stated objective. Re-run profiles, secrets,
   conditional `depends_on`, TCP/UDP port exposure, health checks, named
   volumes, and cleanup as acceptance cases.
4. Preserve Compose solely for the local multi-service environment. It is not
   a deployment abstraction for the Kubernetes/OKD direction already present in
   this repository.

### macOS and local development

Podman on macOS runs containers in a Linux VM managed with `podman machine`;
all `podman machine` commands are rootless. The local client is remote to that
VM, so some local paths, IP/port behaviour, file ownership, performance, and
socket consumers differ from a Linux host. The Mac start documentation shows
that Docker API clients may need `DOCKER_HOST` pointed at the machine API
socket; rootful versus rootless VM connections also see separate image/container
stores. [Podman machine](https://docs.podman.io/en/latest/markdown/podman-machine.1.html),
[Podman machine start](https://docs.podman.io/en/stable/markdown/podman-machine-start.1.html),
[Podman machine rootful switch](https://docs.podman.io/en/stable/markdown/podman-machine-set.1.html)

Therefore replace `scripts/docker-up.sh`/`docker-down.sh` with engine-neutral
helpers only after defining their macOS semantics: machine initialize/start,
connection selection, available CPU/memory/disk, expected rootless mode,
`DOCKER_HOST` bridge policy, and a clear failure message for unsupported
Docker-socket tests. Do not reuse the current script's Docker Desktop GUI
launch/quit behaviour; it is not an abstraction, merely a very specific
arrangement with a whale logo.

## CI/CD and image delivery

The current publishing workflow has a Docker-specific Buildx setup, uses
Docker login actions, and supplies five BuildKit `type=gha` cache input/output
families to the Node packaging verifier. Buildah can build and push registry
images, and can produce OCI or Docker schema output, but this does not imply
BuildKit cache-exporter or Bake parity. Make the cache choice explicit:

| Choice | Benefit | Cost / condition |
| --- | --- | --- |
| Retain Docker Buildx for CI temporarily; migrate developers first | Preserves existing 12-target Bake cache and evidence gate. | The repository remains partly Docker-dependent; it is a bridge, not completion. |
| Replace CI packaging with explicit Buildah jobs | Aligns build mechanism with the target, records registry digests directly, and avoids a Docker daemon. [Buildah build](https://github.com/containers/buildah/blob/main/docs/buildah-build.1.md) | Rewrite `docker-bake.hcl` target expansion, cache policy, log parser, `docker system df` measurement, and image inspection probes. |
| Run Podman CLI in CI | Reuses familiar lifecycle commands and can push OCI/Docker formats. [Podman push](https://docs.podman.io/en/stable/markdown/podman-push.1.html) | Still requires a runner-specific rootless/storage/networking validation; it is not a substitute for a cache design. |

For either target, authenticate using an explicit CI-owned auth file/secret and
avoid writing credentials to a shared developer config. Podman documents
`REGISTRY_AUTH_FILE`, its platform-specific default auth paths, and registry
push destinations; capture the emitted digest and test pull-by-digest before
promotion. [Podman push](https://docs.podman.io/en/stable/markdown/podman-push.1.html)

Build multi-platform images only after the single `linux/amd64` path remains
byte/accounting and behaviour-compatible. Buildah supports building manifest
lists for multiple platforms, with emulation required where the build does not
run natively; the OCI image index is the standard representation for the
platform variants. [Buildah build](https://github.com/containers/buildah/blob/main/docs/buildah-build.1.md),
[OCI image manifest](https://github.com/opencontainers/image-spec/blob/main/manifest.md)

## Staged migration plan

1. **Freeze the baseline.** Record tool versions, `docker compose config`, the
   12 target image digests/sizes, package evidence, and current smoke results.
   Identify the owner and retention policy for the three local named volumes.
   No behaviour changes yet.

2. **Establish a rootless Podman developer pilot.** On Linux, verify subuid/
   subgid, storage driver, `pasta`/`slirp4netns`, rootless network DNS, ports,
   volume permissions, Compose provider version, and both rendered Compose
   configurations. On macOS, provision and start a Podman machine and execute
   the same short stack tests through the remote connection. [Podman rootless
   mode](https://docs.podman.io/en/latest/markdown/podman.1.html), [Podman
   machine](https://docs.podman.io/en/latest/markdown/podman-machine.1.html)

3. **Port image production without changing the runtime.** Add an engine-neutral
   packaging script that maps every Bake target to an explicit Dockerfile,
   context, target, build args, tags, and verification probe. First compare
   Buildah/Podman-produced images against the Docker baseline: architecture,
   digest, image configuration, runtime command, exposed ports, health check,
   file-content assertions, and size budget. Digest equality is not mandatory
   across builders; behavioural and provenance equivalence is.

4. **Migrate CI deliberately.** Run Docker and Buildah packaging jobs in
   parallel until each target can build, run its existing probes, push, pull by
   digest, and publish equivalent evidence. Decide/cache-test the new caching
   mechanism before deleting Buildx configuration. Preserve the current
   `linux/amd64` release requirement first.

5. **Run the API compatibility experiment.** With a rootless Podman API socket,
   execute the actual `simopsdocker` integration/smoke suite using a host-local
   gateway. Record unsupported Moby calls or semantic differences. This phase
   can justify a local-only Podman adapter; it must not normalize a container
   socket mount.

6. **Remove Docker as an application dependency.** Make the Kubernetes runtime
   the scheduler-backed worker mechanism and use a local Podman adapter only
   where developer experience requires it. Delete `/var/run/docker.sock` from
   the compose gateway, run the gateway non-root where possible, rename the
   Docker-specific tests/scripts/policy keys, and update the operational
   evidence. Keep an explicit rollback release rather than keeping two
   untested engines indefinitely.

7. **Cut over and prove recovery.** Require a clean-machine developer setup,
   a macOS machine setup, a Linux rootless setup, both Compose stacks, all
   packaging probes, CI publish/pull-by-digest, and runtime cleanup tests.
   Roll back by selecting the prior engine in local tooling or the previous
   signed/pinned image digest—not by copying opaque container-store directories.

## Validation and risk register

| Risk | Why it matters here | Required evidence / mitigation | Residual risk |
| --- | --- | --- | --- |
| Moby API mismatch | `simopsdocker` uses create/list/inspect/stop/remove semantics, labels, and API negotiation—not shell commands. | Dedicated rootless Podman-service integration suite covering every used API method and error path; redesign behind runtime seam if any semantics differ. [Podman service](https://docs.podman.io/en/latest/markdown/podman-system-service.1.html) | Medium until the socket path is removed. |
| Socket escalation | The gateway container currently receives an engine socket and runs root. | Remove socket mount; do not expose Podman TCP API without mutual TLS; run workers through Kubernetes or a tightly scoped host-local adapter. [Podman service security](https://docs.podman.io/en/latest/markdown/podman-system-service.1.html) | Low after removal; high if retained. |
| Compose-provider drift | Podman delegates Compose; feature coverage follows the chosen provider, not Podman alone. | Pin provider and run configuration plus stack tests for profiles, secrets, health/depends-on, TCP/UDP, volumes, and teardown. [Podman compose](https://docs.podman.io/en/v5.6.2/markdown/podman-compose.1.html) | Medium; upgrades require regression tests. |
| Buildx/Bake/cache regression | Packaging gate depends on Bake, BuildKit log parsing, cache exporters, and `docker system df`. | Explicit target manifest, parallel CI comparison, rewrite evidence parser/metrics, and a documented cache policy. | Medium during CI cutover. |
| Rootless storage/networking surprise | Separate stores, user mappings, VM remoting, and rootless networking differ by OS and host configuration. | Linux and macOS clean-machine acceptance matrix; record machine/rootless mode and storage/network backend. [Podman rootless mode](https://docs.podman.io/en/latest/markdown/podman.1.html), [Podman machine](https://docs.podman.io/en/latest/markdown/podman-machine.1.html) | Medium for developer experience. |
| Image interoperability | A registry or consumer may expect a particular manifest type or platform index. | Push OCI by default, pull and run by digest in target environment; use Docker v2 schema only for a measured compatibility need. [Buildah build](https://github.com/containers/buildah/blob/main/docs/buildah-build.1.md), [Podman push](https://docs.podman.io/en/stable/markdown/podman-push.1.html) | Low with registry acceptance tests. |
| Local state loss | Docker and Podman volume stores are distinct. | Treat current named volumes as non-portable; backup/restore at application level when data must survive. | Low if local data is disposable; otherwise needs an explicit migration runbook. |

## Acceptance criteria for completion

The migration is complete only when the following are demonstrated, not merely
when the word “Docker” is absent from a script name:

- All five Dockerfiles build reproducibly through the selected Podman/Buildah
  path; all 12 former Bake targets retain their arguments, target stages, tags,
  size checks, and behavioural probes.
- Published images are pulled and run by digest in the target environment, with
  the required `linux/amd64` variant and an explicit manifest-format policy.
- Both Compose files validate and their required profiles/services pass the
  defined local smoke suite using a pinned provider.
- Linux rootless and macOS Podman-machine developer flows are documented and
  tested from a clean state.
- The gateway has no container-engine socket mount in its normal deployment;
  managed workers use the Kubernetes runtime or an equally narrow, tested
  alternative.
- CI has a documented build/cache/auth/evidence strategy that does not depend
  on accidental Docker Buildx compatibility, and the old Docker-specific path
  has a scheduled removal decision.

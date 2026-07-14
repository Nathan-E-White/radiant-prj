import assert from "node:assert/strict";
import test from "node:test";

import {
  assertLocalModuleReplacement,
  assertMinimumModuleVersion,
  assertModuleNotRequired,
  parseGoModuleVersion,
} from "./go-module-version.mjs";

test("parses a direct module version from a go.mod require block", () => {
  const goMod = `module example.test/service

require (
	github.com/docker/docker v29.3.1+incompatible
)
`;

  assert.equal(
    parseGoModuleVersion(goMod, "github.com/docker/docker"),
    "v29.3.1+incompatible",
  );
});

test("accepts versions at or above the security floor", () => {
  const atFloor = "require github.com/docker/docker v29.3.1+incompatible\n";
  const aboveFloor = "require github.com/docker/docker v30.0.0+incompatible\n";

  assert.doesNotThrow(() =>
    assertMinimumModuleVersion(atFloor, "github.com/docker/docker", "29.3.1"),
  );
  assert.doesNotThrow(() =>
    assertMinimumModuleVersion(aboveFloor, "github.com/docker/docker", "29.3.1"),
  );
});

test("rejects a prerelease at a stable security floor", () => {
  const prerelease =
    "require github.com/moby/moby/api v1.55.0-beta.1\n";

  assert.throws(
    () =>
      assertMinimumModuleVersion(
        prerelease,
        "github.com/moby/moby/api",
        "1.55.0",
      ),
    /requires github\.com\/moby\/moby\/api >= v1\.55\.0/,
  );
});

test("rejects a version below the security floor", () => {
  const vulnerable = "require github.com/docker/docker v28.5.2+incompatible\n";

  assert.throws(
    () =>
      assertMinimumModuleVersion(
        vulnerable,
        "github.com/docker/docker",
        "29.3.1",
      ),
    /requires github\.com\/docker\/docker >= v29\.3\.1; found v28\.5\.2\+incompatible/,
  );
});

test("rejects a missing direct module declaration", () => {
  assert.throws(
    () =>
      assertMinimumModuleVersion(
        "module example.test/service\n",
        "github.com/docker/docker",
        "29.3.1",
      ),
    /does not directly require github\.com\/docker\/docker/,
  );
});

test("rejects a forbidden legacy module declaration", () => {
  const legacy = "require github.com/docker/docker v28.5.2+incompatible\n";
  const indirectLegacy =
    "require github.com/docker/docker v28.5.2+incompatible // indirect\n";

  assert.throws(
    () => assertModuleNotRequired(legacy, "github.com/docker/docker"),
    /must not require legacy module github\.com\/docker\/docker/,
  );
  assert.throws(
    () => assertModuleNotRequired(indirectLegacy, "github.com/docker/docker"),
    /must not require legacy module github\.com\/docker\/docker/,
  );
  assert.doesNotThrow(() =>
    assertModuleNotRequired(
      "require github.com/moby/moby/client v0.5.0\n",
      "github.com/docker/docker",
    ),
  );
});

test("requires the legacy module graph edge to resolve to the local shim", () => {
  const replaced = {
    Path: "github.com/docker/docker",
    Version: "v28.5.2+incompatible",
    Replace: { Path: "./third_party/docker-compat" },
  };

  assert.doesNotThrow(() =>
    assertLocalModuleReplacement(
      replaced,
      "github.com/docker/docker",
      "./third_party/docker-compat",
    ),
  );
  assert.throws(
    () =>
      assertLocalModuleReplacement(
        { ...replaced, Replace: undefined },
        "github.com/docker/docker",
        "./third_party/docker-compat",
      ),
    /must resolve to local replacement/,
  );
  assert.throws(
    () =>
      assertLocalModuleReplacement(
        { ...replaced, Replace: { Path: "github.com/moby/moby/v2" } },
        "github.com/docker/docker",
        "./third_party/docker-compat",
      ),
    /must resolve to local replacement/,
  );
  assert.throws(
    () =>
      assertLocalModuleReplacement(
        { ...replaced, Path: "github.com/docker/not-docker" },
        "github.com/docker/docker",
        "./third_party/docker-compat",
      ),
    /must resolve to local replacement/,
  );
});

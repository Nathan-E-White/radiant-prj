function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function parseComparableVersion(version) {
  const match =
    /^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?(?:\+.*)?$/.exec(
      version,
    );
  if (!match) {
    throw new Error(`unsupported Go module version: ${version}`);
  }

  return {
    parts: match.slice(1, 4).map(Number),
    prerelease: match[4] ?? null,
  };
}

export function parseGoModuleVersion(goMod, modulePath) {
  const directRequirement = new RegExp(
    `^[\\t ]*(?:require[\\t ]+)?${escapeRegExp(modulePath)}[\\t ]+(v\\S+?)[\\t ]*$`,
    "m",
  );
  return directRequirement.exec(goMod)?.[1] ?? null;
}

function parseAnyGoModuleVersion(goMod, modulePath) {
  const requirement = new RegExp(
    `^[\\t ]*(?:require[\\t ]+)?${escapeRegExp(modulePath)}[\\t ]+(v\\S+?)(?:[\\t ]+//[\\t ]+indirect)?[\\t ]*$`,
    "m",
  );
  return requirement.exec(goMod)?.[1] ?? null;
}

export function assertMinimumModuleVersion(goMod, modulePath, minimumVersion) {
  const currentVersion = parseGoModuleVersion(goMod, modulePath);
  if (!currentVersion) {
    throw new Error(`go.mod does not directly require ${modulePath}`);
  }

  const current = parseComparableVersion(currentVersion);
  const minimum = parseComparableVersion(minimumVersion);
  const differingPart = current.parts.findIndex(
    (part, index) => part !== minimum.parts[index],
  );
  const belowMinimum =
    (differingPart >= 0 &&
      current.parts[differingPart] < minimum.parts[differingPart]) ||
    (differingPart < 0 && current.prerelease && !minimum.prerelease);

  if (belowMinimum) {
    throw new Error(
      `go.mod requires ${modulePath} >= v${minimumVersion}; found ${currentVersion}`,
    );
  }

  return currentVersion;
}

export function assertModuleNotRequired(goMod, modulePath) {
  const currentVersion = parseAnyGoModuleVersion(goMod, modulePath);
  if (currentVersion) {
    throw new Error(
      `go.mod must not require legacy module ${modulePath}; found ${currentVersion}`,
    );
  }
}

export function assertLocalModuleReplacement(
  moduleInfo,
  modulePath,
  expectedReplacement,
) {
  const actualReplacement = moduleInfo?.Replace?.Path;
  if (
    moduleInfo?.Path !== modulePath ||
    actualReplacement !== expectedReplacement ||
    !actualReplacement.startsWith("./")
  ) {
    throw new Error(
      `${modulePath} must resolve to local replacement ${expectedReplacement}`,
    );
  }
}

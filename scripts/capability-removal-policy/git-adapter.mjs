import { execFileSync } from "node:child_process";

export function loadChangedPaths({ base, root = process.cwd(), run = execFileSync } = {}) {
  if (!base) throw new Error("a Git base revision is required");
  const output = run("git", ["diff", "--name-status", "--find-renames", base], { cwd: root, encoding: "utf8" });
  return output.trim().split("\n").filter(Boolean).map((line) => {
    const [status, first, second] = line.split("\t");
    if (status.startsWith("R")) return { status: "R", oldPath: first, path: second };
    return { status: status[0], path: first };
  });
}

#!/usr/bin/env bun
import {
  applyFleetBoardAction,
  createInitialFleetBoardState,
  summarizeFleetBoard,
  type FleetBoardFacilityKind,
  type FleetBoardState
} from "../src/domain/fleet-board";

const bold = "\x1b[1m";
const dim = "\x1b[2m";
const reset = "\x1b[0m";

let state = createInitialFleetBoardState({ seed: "prototype" });

const placements: Array<{ key: string; kind: FleetBoardFacilityKind; x: number; y: number }> = [
  { key: "f", kind: "trisoFactory", x: 2, y: 2 },
  { key: "r", kind: "reactor", x: 5, y: 2 },
  { key: "d", kind: "desalPlant", x: 8, y: 2 },
  { key: "b", kind: "armyBase", x: 5, y: 5 },
  { key: "k", kind: "battery", x: 8, y: 5 }
];

render(state);

process.stdin.setRawMode(true);
process.stdin.resume();
process.stdin.setEncoding("utf8");
process.stdin.on("data", (key) => {
  if (key === "q" || key === "\u0003") {
    process.stdout.write("\n");
    process.exit(0);
  }
  if (key === "t") {
    state = applyFleetBoardAction(state, { type: "tickDay" });
  }
  if (key === "u") {
    const reactor = Object.values(state.facilities).find((facility) => facility.kind === "reactor");
    if (reactor) {
      state = applyFleetBoardAction(state, { type: "refuelFacility", facilityId: reactor.id });
    }
  }
  const placement = placements.find((candidate) => candidate.key === key);
  if (placement) {
    state = applyFleetBoardAction(state, {
      type: "placeFacility",
      facilityId: `${placement.kind}-${Object.keys(state.facilities).length + 1}`,
      facilityKind: placement.kind,
      position: { x: placement.x, y: placement.y }
    });
  }
  render(state);
});

function render(next: FleetBoardState) {
  const summary = summarizeFleetBoard(next);
  process.stdout.write("\x1b[2J\x1b[H");
  process.stdout.write(`${bold}Fleet Board Logic Prototype${reset}\n`);
  process.stdout.write(`${dim}Question: does the contract sprint economy feel coherent before Phaser gets involved?${reset}\n\n`);
  process.stdout.write(`${bold}State${reset}\n`);
  process.stdout.write(`day: ${summary.day}/30\n`);
  process.stdout.write(`cash: $${summary.cash}\n`);
  process.stdout.write(`fuel blocks: ${summary.fuelBlocks}\n`);
  process.stdout.write(`electric: ${summary.electricMwe} MWe\n`);
  process.stdout.write(`thermal: ${summary.thermalMwt} MWt\n`);
  process.stdout.write(`water credits: ${summary.waterCredits}\n`);
  process.stdout.write(`resilience credits: ${summary.resilienceCredits}\n`);
  process.stdout.write(`score: ${summary.score}\n`);
  process.stdout.write(`removed: ${summary.removed}\n`);
  process.stdout.write(`complete: ${summary.complete}\n\n`);

  process.stdout.write(`${bold}Facilities${reset}\n`);
  for (const facility of Object.values(next.facilities)) {
    process.stdout.write(
      `${facility.id}: ${facility.label} @ ${facility.position.x},${facility.position.y} ${facility.status}`
    );
    if (facility.outageDaysRemaining > 0) {
      process.stdout.write(` (${facility.outageDaysRemaining}d)`);
    }
    process.stdout.write("\n");
  }
  if (Object.keys(next.facilities).length === 0) {
    process.stdout.write(`${dim}none yet${reset}\n`);
  }

  process.stdout.write(`\n${bold}Pawns${reset}\n`);
  for (const pawn of Object.values(next.pawns)) {
    process.stdout.write(`${pawn.kind}: ${pawn.position.x},${pawn.position.y}\n`);
  }

  process.stdout.write(`\n${bold}Recent Events${reset}\n`);
  for (const event of next.events.slice(-7)) {
    process.stdout.write(`${event.day}: ${event.kind} - ${event.detail}\n`);
  }
  if (next.events.length === 0) {
    process.stdout.write(`${dim}none yet${reset}\n`);
  }

  process.stdout.write(
    `\n${bold}Keys${reset} ${bold}f${reset}${dim} fuel${reset}  ${bold}r${reset}${dim} reactor${reset}  ${bold}d${reset}${dim} desal${reset}  ${bold}b${reset}${dim} base${reset}  ${bold}k${reset}${dim} battery${reset}  ${bold}t${reset}${dim} tick day${reset}  ${bold}u${reset}${dim} refuel first reactor${reset}  ${bold}q${reset}${dim} quit${reset}\n`
  );
}

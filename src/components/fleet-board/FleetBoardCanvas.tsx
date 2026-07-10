import { useEffect, useMemo, useRef } from "react";
import type { FleetBoardFacilityKind, FleetBoardSceneModel } from "../../domain/fleet-board";

const spriteSheetUrl = new URL("../../assets/fleet-board/fleet-board-placeholder-sprites.png", import.meta.url).href;

const frameBySpriteKey: Record<string, number> = {
  reactor: 0,
  trisoFactory: 1,
  desalPlant: 2,
  armyBase: 3,
  battery: 4,
  inspector: 5,
  trouble: 6,
  routePulse: 7
};

export function FleetBoardCanvas({
  scene,
  onPlaceFacility
}: {
  scene: FleetBoardSceneModel;
  onPlaceFacility: (facilityKind: FleetBoardFacilityKind, x: number, y: number) => void;
}) {
  const hostRef = useRef<HTMLDivElement | null>(null);
  const sceneKey = useMemo(() => JSON.stringify(scene), [scene]);

  useEffect(() => {
    let game: { destroy: (removeCanvas: boolean) => void } | null = null;
    let cancelled = false;

    void import("phaser").then((module) => {
      if (cancelled || !hostRef.current) {
        return;
      }
      const Phaser = (module.default ?? module) as typeof import("phaser");
      const host = hostRef.current;
      const tileSize = scene.grid.tileSize;
      const offset = { x: 64, y: 52 };
      const worldWidth = offset.x * 2 + scene.grid.columns * tileSize;
      const worldHeight = offset.y * 2 + scene.grid.rows * tileSize;

      class FleetBoardScene extends Phaser.Scene {
        preload() {
          this.load.spritesheet("fleet-board-placeholder", spriteSheetUrl, {
            frameWidth: 448,
            frameHeight: 512
          });
        }

        create() {
          this.cameras.main.setBackgroundColor("#101922");
          this.cameras.main.setBounds(0, 0, worldWidth, worldHeight);
          this.input.setTopOnly(true);

          const graphics = this.add.graphics();
          graphics.lineStyle(1, 0x274554, 0.8);
          for (let column = 0; column <= scene.grid.columns; column += 1) {
            const x = offset.x + column * tileSize;
            graphics.lineBetween(x, offset.y, x, offset.y + scene.grid.rows * tileSize);
          }
          for (let row = 0; row <= scene.grid.rows; row += 1) {
            const y = offset.y + row * tileSize;
            graphics.lineBetween(offset.x, y, offset.x + scene.grid.columns * tileSize, y);
          }

          graphics.lineStyle(3, 0x2ad6c7, 0.62);
          for (const route of buildRouteSegments(scene)) {
            graphics.lineBetween(
              offset.x + route.from.gridX * tileSize,
              offset.y + route.from.gridY * tileSize,
              offset.x + route.to.gridX * tileSize,
              offset.y + route.to.gridY * tileSize
            );
          }

          for (const facility of scene.facilities) {
            const x = offset.x + facility.gridX * tileSize;
            const y = offset.y + facility.gridY * tileSize;
            const sprite = this.add
              .image(x, y, "fleet-board-placeholder", frameBySpriteKey[facility.spriteKey])
              .setDisplaySize(58, 66)
              .setAlpha(facility.status === "active" ? 1 : 0.58);
            this.add
              .text(x, y + 42, facility.label, {
                fontFamily: "Inter, system-ui, sans-serif",
                fontSize: "10px",
                color: facility.status === "active" ? "#d7fff5" : "#ffcc80"
              })
              .setOrigin(0.5);
            sprite.setData("facilityId", facility.id);
          }

          for (const pawn of scene.pawns) {
            const x = offset.x + pawn.gridX * tileSize;
            const y = offset.y + pawn.gridY * tileSize;
            this.add
              .image(x, y, "fleet-board-placeholder", frameBySpriteKey[pawn.spriteKey])
              .setDisplaySize(44, 52)
              .setDepth(5);
          }

          const routePulse = this.add
            .image(offset.x + 10.5 * tileSize, offset.y + 0.85 * tileSize, "fleet-board-placeholder", frameBySpriteKey.routePulse)
            .setDisplaySize(50, 36)
            .setAlpha(0.75);
          this.tweens.add({
            targets: routePulse,
            alpha: 0.25,
            x: routePulse.x - 80,
            duration: 1100,
            yoyo: true,
            repeat: -1
          });

          const dragCard = this.add
            .image(offset.x + 0.7 * tileSize, offset.y + 6.9 * tileSize, "fleet-board-placeholder", frameBySpriteKey.reactor)
            .setDisplaySize(54, 62)
            .setInteractive({ draggable: true })
            .setDepth(10);
          this.add
            .text(dragCard.x, dragCard.y + 42, "Reactor", {
              fontFamily: "Inter, system-ui, sans-serif",
              fontSize: "11px",
              color: "#b9f8f0"
            })
            .setOrigin(0.5);

          this.input.setDraggable(dragCard);
          this.input.on("drag", (_pointer: unknown, gameObject: Phaser.GameObjects.Image, dragX: number, dragY: number) => {
            gameObject.setPosition(dragX, dragY);
          });
          this.input.on("dragend", (_pointer: unknown, gameObject: Phaser.GameObjects.Image) => {
            const gridX = Math.round((gameObject.x - offset.x) / tileSize);
            const gridY = Math.round((gameObject.y - offset.y) / tileSize);
            onPlaceFacility(
              "reactor",
              Math.max(0, Math.min(scene.grid.columns - 1, gridX)),
              Math.max(0, Math.min(scene.grid.rows - 1, gridY))
            );
          });

          let panStart:
            | {
                pointerX: number;
                pointerY: number;
                scrollX: number;
                scrollY: number;
              }
            | null = null;

          this.input.on("pointerdown", (pointer: Phaser.Input.Pointer, gameObjects: Phaser.GameObjects.GameObject[]) => {
            if (gameObjects.length > 0) {
              return;
            }
            panStart = {
              pointerX: pointer.x,
              pointerY: pointer.y,
              scrollX: this.cameras.main.scrollX,
              scrollY: this.cameras.main.scrollY
            };
          });

          this.input.on("pointermove", (pointer: Phaser.Input.Pointer) => {
            if (!panStart || !pointer.isDown) {
              return;
            }
            const camera = this.cameras.main;
            camera.setScroll(
              panStart.scrollX - (pointer.x - panStart.pointerX) / camera.zoom,
              panStart.scrollY - (pointer.y - panStart.pointerY) / camera.zoom
            );
          });

          this.input.on("pointerup", () => {
            panStart = null;
          });

          this.input.on("wheel", (_pointer: unknown, _gameObjects: unknown, _deltaX: number, deltaY: number) => {
            const camera = this.cameras.main;
            camera.setZoom(Math.max(0.82, Math.min(1.25, camera.zoom + (deltaY > 0 ? -0.08 : 0.08))));
          });
        }
      }

      game = new Phaser.Game({
        type: Phaser.CANVAS,
        parent: host,
        width: 980,
        height: 640,
        transparent: false,
        scene: FleetBoardScene,
        input: {
          mouse: true,
          touch: true
        }
      });
    });

    return () => {
      cancelled = true;
      game?.destroy(true);
    };
  }, [sceneKey, onPlaceFacility, scene]);

  return <div className="fleet-board-canvas" data-testid="fleet-board-canvas" ref={hostRef} />;
}

function buildRouteSegments(scene: FleetBoardSceneModel) {
  const reactors = scene.facilities.filter((facility) => facility.kind === "reactor" && facility.status === "active");
  const routeTargets = scene.facilities.filter(
    (facility) =>
      facility.status === "active" &&
      (facility.kind === "trisoFactory" ||
        facility.kind === "desalPlant" ||
        facility.kind === "armyBase" ||
        facility.kind === "battery")
  );

  return reactors.flatMap((reactor) =>
    routeTargets
      .filter((facility) => manhattanDistance(reactor, facility) <= 4)
      .map((facility) => ({ from: reactor, to: facility }))
  );
}

function manhattanDistance(
  left: { gridX: number; gridY: number },
  right: { gridX: number; gridY: number }
): number {
  return Math.abs(left.gridX - right.gridX) + Math.abs(left.gridY - right.gridY);
}

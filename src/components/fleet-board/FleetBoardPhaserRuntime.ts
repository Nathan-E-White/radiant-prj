import type {
  FleetBoardFacilityKind,
  FleetBoardSceneModel,
  FleetBoardSpriteKey
} from "../../domain/fleet-board";

export type FleetBoardPhaserUpdate = {
  scene: FleetBoardSceneModel;
  onPlaceFacility: (facilityKind: FleetBoardFacilityKind, x: number, y: number) => void;
  onSelectReactor: (facilityId: string) => void;
};

export type FleetBoardPhaserMount = FleetBoardPhaserUpdate & {
  host: HTMLDivElement;
};

type MountedFleetBoardPhaserGame = {
  update: (next: FleetBoardPhaserUpdate) => void;
  destroy: () => void;
};

type StartFleetBoardPhaserGame = (
  mount: FleetBoardPhaserMount
) => Promise<MountedFleetBoardPhaserGame>;

const spriteSheetUrl = new URL("../../assets/fleet-board/fleet-board-placeholder-sprites.png", import.meta.url).href;

const frameBySpriteKey: Record<FleetBoardSpriteKey, number> = {
  reactor: 0,
  trisoFactory: 1,
  desalPlant: 2,
  armyBase: 3,
  battery: 4,
  inspector: 5,
  trouble: 6,
  routePulse: 7
};

export function createFleetBoardPhaserRuntime(startGame: StartFleetBoardPhaserGame = startFleetBoardPhaserGame) {
  let mountedGame: MountedFleetBoardPhaserGame | null = null;
  let startPromise: Promise<void> | null = null;
  let pendingUpdate: FleetBoardPhaserUpdate | null = null;
  let destroyed = false;

  return {
    mount(mount: FleetBoardPhaserMount) {
      startPromise ??= startGame(mount).then((game) => {
        if (destroyed) {
          game.destroy();
          return;
        }
        mountedGame = game;
        if (pendingUpdate) {
          game.update(pendingUpdate);
          pendingUpdate = null;
        }
      });
    },
    update(next: FleetBoardPhaserUpdate) {
      if (destroyed) {
        return;
      }
      if (mountedGame) {
        mountedGame.update(next);
        return;
      }
      pendingUpdate = next;
    },
    destroy() {
      if (destroyed) {
        return;
      }
      destroyed = true;
      mountedGame?.destroy();
      mountedGame = null;
      pendingUpdate = null;
    },
    ready() {
      return startPromise ?? Promise.resolve();
    }
  };
}

async function startFleetBoardPhaserGame(mount: FleetBoardPhaserMount): Promise<MountedFleetBoardPhaserGame> {
  const module = await import("phaser");
  const Phaser = (module.default ?? module) as typeof import("phaser");
  let latest = mount;
  let mountedScene: FleetBoardScene | null = null;

  class FleetBoardScene extends Phaser.Scene {
    private dynamicLayer: Phaser.GameObjects.Container | null = null;

    preload() {
      this.load.spritesheet("fleet-board-placeholder", spriteSheetUrl, {
        frameWidth: 448,
        frameHeight: 512
      });
    }

    create() {
      mountedScene = this;
      const { scene } = latest;
      const { offset, worldHeight, worldWidth } = measureWorld(scene);
      const tileSize = scene.grid.tileSize;

      this.cameras.main.setBackgroundColor("#101922");
      this.cameras.main.setBounds(0, 0, worldWidth, worldHeight);
      this.input.setTopOnly(true);

      const grid = this.add.graphics();
      grid.lineStyle(1, 0x274554, 0.8);
      for (let column = 0; column <= scene.grid.columns; column += 1) {
        const x = offset.x + column * tileSize;
        grid.lineBetween(x, offset.y, x, offset.y + scene.grid.rows * tileSize);
      }
      for (let row = 0; row <= scene.grid.rows; row += 1) {
        const y = offset.y + row * tileSize;
        grid.lineBetween(offset.x, y, offset.x + scene.grid.columns * tileSize, y);
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
        const current = latest;
        const currentMeasurements = measureWorld(current.scene);
        const currentTileSize = current.scene.grid.tileSize;
        const gridX = Math.round((gameObject.x - currentMeasurements.offset.x) / currentTileSize);
        const gridY = Math.round((gameObject.y - currentMeasurements.offset.y) / currentTileSize);
        current.onPlaceFacility(
          "reactor",
          Math.max(0, Math.min(current.scene.grid.columns - 1, gridX)),
          Math.max(0, Math.min(current.scene.grid.rows - 1, gridY))
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

      this.dynamicLayer = this.add.container();
      this.render(latest.scene);
    }

    render(scene: FleetBoardSceneModel) {
      if (!this.dynamicLayer) {
        return;
      }
      this.dynamicLayer.removeAll(true);
      const { offset } = measureWorld(scene);
      const tileSize = scene.grid.tileSize;
      const graphics = this.add.graphics();
      this.dynamicLayer.add(graphics);

      graphics.lineStyle(3, 0x2ad6c7, 0.62);
      for (const route of scene.routes) {
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
        if (facility.id === scene.selectedReactorId) {
          graphics.lineStyle(3, 0xf8d66d, 0.92);
          graphics.strokeCircle(x, y, 42);
        }
        const sprite = this.add
          .image(x, y, "fleet-board-placeholder", frameBySpriteKey[facility.spriteKey])
          .setDisplaySize(58, 66)
          .setAlpha(facility.status === "active" ? 1 : 0.58);
        const label = this.add
          .text(x, y + 42, facility.label, {
            fontFamily: "Inter, system-ui, sans-serif",
            fontSize: "10px",
            color: facility.status === "active" ? "#d7fff5" : "#ffcc80"
          })
          .setOrigin(0.5);
        sprite.setData("facilityId", facility.id);
        if (facility.kind === "reactor") {
          sprite.setInteractive();
          sprite.on("pointerdown", () => latest.onSelectReactor(facility.id));
        }
        this.dynamicLayer.add([sprite, label]);
      }

      for (const rail of scene.reactorSlotRails) {
        const centerX = offset.x + rail.gridX * tileSize;
        const centerY = offset.y + rail.gridY * tileSize;
        const railWidth = Math.max(52, rail.slots.length * 28 + 12);
        graphics.fillStyle(0x0b1218, 0.92);
        graphics.fillRoundedRect(centerX - railWidth / 2, centerY - 13, railWidth, 26, 9);
        graphics.lineStyle(1, 0x7d8b99, 0.9);
        graphics.strokeRoundedRect(centerX - railWidth / 2, centerY - 13, railWidth, 26, 9);
        for (const slot of rail.slots) {
          const slotX = centerX + (slot.slotIndex - (rail.slots.length - 1) / 2) * 28;
          const slotFill = {
            empty: 0x223641,
            idle: 0x30d9ff,
            queued: 0xf8d66d,
            running: 0x55e69a
          }[slot.status];
          graphics.fillStyle(slotFill, 0.96);
          graphics.fillCircle(slotX, centerY, 9);
          graphics.lineStyle(1, slot.status === "empty" ? 0x6d7f89 : 0xe8ffff, 0.95);
          graphics.strokeCircle(slotX, centerY, 9);
          if (slot.status !== "empty") {
            const statusLabel = this.add
              .text(
                slotX,
                centerY,
                slot.status === "idle" ? "I" : slot.status === "queued" ? "Q" : `${slot.advancesRemaining ?? 0}`,
                {
                  fontFamily: "Inter, system-ui, sans-serif",
                  fontSize: "9px",
                  color: "#081116"
                }
              )
              .setOrigin(0.5);
            this.dynamicLayer.add(statusLabel);
          }
        }
        const railLabel = this.add
          .text(centerX, centerY - 20, rail.label, {
            fontFamily: "Inter, system-ui, sans-serif",
            fontSize: "8px",
            color: "#b9f8f0"
          })
          .setOrigin(0.5);
        this.dynamicLayer.add(railLabel);
      }

      for (const badge of scene.insightTokenBadges) {
        const x = offset.x + badge.gridX * tileSize;
        const y = offset.y + badge.gridY * tileSize;
        graphics.fillStyle(0xc98cff, 0.96);
        graphics.fillCircle(x, y, 13);
        graphics.lineStyle(2, 0xf3dcff, 0.96);
        graphics.strokeCircle(x, y, 13);
        const label = this.add
          .text(x, y, `${badge.count}`, {
            fontFamily: "Inter, system-ui, sans-serif",
            fontSize: "10px",
            color: "#13091b"
          })
          .setOrigin(0.5);
        this.dynamicLayer.add(label);
      }

      for (const pawn of scene.pawns) {
        const pawnSprite = this.add
          .image(
            offset.x + pawn.gridX * tileSize,
            offset.y + pawn.gridY * tileSize,
            "fleet-board-placeholder",
            frameBySpriteKey[pawn.spriteKey]
          )
          .setDisplaySize(44, 52)
          .setDepth(5);
        this.dynamicLayer.add(pawnSprite);
      }
    }
  }

  const game = new Phaser.Game({
    type: Phaser.CANVAS,
    parent: mount.host,
    width: 980,
    height: 640,
    transparent: false,
    scene: FleetBoardScene,
    input: {
      mouse: true,
      touch: true
    }
  });
  const ownedCanvas = game.canvas;

  return {
    update(next) {
      latest = { ...next, host: mount.host };
      mountedScene?.render(next.scene);
    },
    destroy() {
      mountedScene = null;
      game.destroy(true);
      ownedCanvas.remove();
    }
  };
}

function measureWorld(scene: FleetBoardSceneModel) {
  const offset = { x: 64, y: 52 };
  return {
    offset,
    worldWidth: offset.x * 2 + scene.grid.columns * scene.grid.tileSize,
    worldHeight: offset.y * 2 + scene.grid.rows * scene.grid.tileSize
  };
}

import { useEffect, useRef } from "react";
import type { FleetBoardFacilityKind, FleetBoardSceneModel } from "../../domain/fleet-board";
import { createFleetBoardPhaserRuntime } from "./FleetBoardPhaserRuntime";

export function FleetBoardCanvas({
  scene,
  onPlaceFacility,
  onSelectReactor
}: {
  scene: FleetBoardSceneModel;
  onPlaceFacility: (facilityKind: FleetBoardFacilityKind, x: number, y: number) => void;
  onSelectReactor: (facilityId: string) => void;
}) {
  const hostRef = useRef<HTMLDivElement | null>(null);
  const runtimeRef = useRef<ReturnType<typeof createFleetBoardPhaserRuntime> | null>(null);
  const initialPropsRef = useRef({ scene, onPlaceFacility, onSelectReactor });
  const skippedInitialUpdateRef = useRef(false);

  useEffect(() => {
    const host = hostRef.current;
    if (!host) {
      return;
    }
    const runtime = createFleetBoardPhaserRuntime();
    runtimeRef.current = runtime;
    runtime.mount({ host, ...initialPropsRef.current });

    return () => {
      runtime.destroy();
      runtimeRef.current = null;
    };
  }, []);

  useEffect(() => {
    if (!skippedInitialUpdateRef.current) {
      skippedInitialUpdateRef.current = true;
      return;
    }
    runtimeRef.current?.update({ scene, onPlaceFacility, onSelectReactor });
  }, [scene, onPlaceFacility, onSelectReactor]);

  return <div className="fleet-board-canvas" data-testid="fleet-board-canvas" ref={hostRef} />;
}

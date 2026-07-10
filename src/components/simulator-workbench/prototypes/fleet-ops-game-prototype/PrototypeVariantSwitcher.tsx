import { ChevronLeft, ChevronRight } from "lucide-react";
import { useEffect } from "react";
import type { FleetOpsPrototypeVariant } from "./fleetOpsPrototypeModel";
import { fleetOpsVariantMeta } from "./fleetOpsPrototypeModel";

const variants: Array<FleetOpsPrototypeVariant> = ["A", "B", "C", "D"];

export function PrototypeVariantSwitcher({
  current,
  onChange
}: {
  current: FleetOpsPrototypeVariant;
  onChange: (variant: FleetOpsPrototypeVariant) => void;
}) {
  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      const target = event.target as HTMLElement | null;
      const editable = target?.closest("input, textarea, select, [contenteditable='true']");
      if (editable) return;
      if (event.key === "ArrowLeft") {
        event.preventDefault();
        onChange(nextVariant(current, -1));
      }
      if (event.key === "ArrowRight") {
        event.preventDefault();
        onChange(nextVariant(current, 1));
      }
    }

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [current, onChange]);

  if (import.meta.env.PROD) {
    return null;
  }

  return (
    <div className="fleet-proto-switcher" aria-label="Fleet ops prototype variant switcher">
      <button type="button" onClick={() => onChange(nextVariant(current, -1))} aria-label="Previous prototype variant">
        <ChevronLeft size={18} />
      </button>
      <span>
        {current} - {fleetOpsVariantMeta[current].name}
      </span>
      <button type="button" onClick={() => onChange(nextVariant(current, 1))} aria-label="Next prototype variant">
        <ChevronRight size={18} />
      </button>
    </div>
  );
}

function nextVariant(current: FleetOpsPrototypeVariant, delta: -1 | 1): FleetOpsPrototypeVariant {
  const currentIndex = variants.indexOf(current);
  const nextIndex = (currentIndex + delta + variants.length) % variants.length;
  return variants[nextIndex];
}

export function readPrototypeVariant(): FleetOpsPrototypeVariant {
  const search = new URLSearchParams(window.location.search);
  const requested = search.get("variant");
  return isPrototypeVariant(requested) ? requested : "A";
}

export function writePrototypeVariant(variant: FleetOpsPrototypeVariant) {
  const url = new URL(window.location.href);
  url.searchParams.set("prototype", "fleet-ops");
  url.searchParams.set("variant", variant);
  window.history.replaceState(null, "", url);
}

function isPrototypeVariant(value: string | null): value is FleetOpsPrototypeVariant {
  return value === "A" || value === "B" || value === "C" || value === "D";
}

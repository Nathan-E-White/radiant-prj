import type { WorkbenchValueBasis } from "../api/simulatorWorkbench";

export type InteractionStateId = "hover" | "focus" | "selected" | "disabled" | "warning" | "current";

export type GrammarToken = {
  id: string;
  label: string;
  usage: string;
  className: string;
};

export type InteractionStateSpec = GrammarToken & {
  id: InteractionStateId;
};

export type ValueBasisSpec = GrammarToken & {
  id: WorkbenchValueBasis;
  iconLabel: string;
  ruleStyle: "solid" | "double" | "dashed";
  texture: string;
};

export const semanticColorTokens: GrammarToken[] = [
  {
    id: "surface",
    label: "Surface",
    usage: "Ordinary reading canvas and grouped content.",
    className: "grammar-swatch-surface"
  },
  {
    id: "ink",
    label: "Ink",
    usage: "Primary review text on light surfaces.",
    className: "grammar-swatch-ink"
  },
  {
    id: "measured",
    label: "Measured",
    usage: "Observed state from resident stand-ins or measured tags.",
    className: "grammar-swatch-measured"
  },
  {
    id: "imputed",
    label: "Imputed",
    usage: "Twin-derived estimates and model interpolation.",
    className: "grammar-swatch-imputed"
  },
  {
    id: "simulated",
    label: "Simulated",
    usage: "Run-scoped scientific result state.",
    className: "grammar-swatch-simulated"
  },
  {
    id: "warning",
    label: "Warning",
    usage: "Exceptions, holds, degraded reads, and retry-needed states only.",
    className: "grammar-swatch-warning"
  },
  {
    id: "success",
    label: "Success",
    usage: "Completed controls, clean traceability, and healthy checks.",
    className: "grammar-swatch-success"
  },
  {
    id: "dark",
    label: "Dark Spatial",
    usage: "Earned spatial canvases, terminals, and dense status bays.",
    className: "grammar-swatch-dark"
  }
];

export const typeScaleTokens: GrammarToken[] = [
  {
    id: "display",
    label: "Display",
    usage: "One page-level product title only.",
    className: "grammar-type-display"
  },
  {
    id: "section",
    label: "Section",
    usage: "Panel and path headings.",
    className: "grammar-type-section"
  },
  {
    id: "body",
    label: "Body",
    usage: "Screen-share-readable review text.",
    className: "grammar-type-body"
  },
  {
    id: "caption",
    label: "Caption",
    usage: "IDs, metadata, and compact table labels.",
    className: "grammar-type-caption"
  }
];

export const ruleStyleTokens: GrammarToken[] = [
  {
    id: "whitespace",
    label: "Whitespace",
    usage: "Default grouping and visual rhythm.",
    className: "grammar-rule-whitespace"
  },
  {
    id: "containment",
    label: "Containment",
    usage: "Panels, tables, and repeated evidence records.",
    className: "grammar-rule-containment"
  },
  {
    id: "selection",
    label: "Selection",
    usage: "Current tab, selected unit, selected row, or selected value.",
    className: "grammar-rule-selection"
  },
  {
    id: "exception",
    label: "Exception",
    usage: "Warnings, degraded reads, and evidence holds.",
    className: "grammar-rule-exception"
  }
];

export const targetSizeTokens: GrammarToken[] = [
  {
    id: "frequent-command",
    label: "44 px command",
    usage: "Frequent actions and path navigation.",
    className: "grammar-target-command"
  },
  {
    id: "compact-control",
    label: "36 px compact",
    usage: "Dense filters, status controls, and secondary actions.",
    className: "grammar-target-compact"
  }
];

export const interactionStateSpecs: InteractionStateSpec[] = [
  {
    id: "hover",
    label: "Hover",
    usage: "Raised contrast and border affordance before action.",
    className: "grammar-state-hover"
  },
  {
    id: "focus",
    label: "Focus",
    usage: "Keyboard-visible outline outside the component bounds.",
    className: "grammar-state-focus"
  },
  {
    id: "selected",
    label: "Selected",
    usage: "Persistent fill plus strong rule on the active choice.",
    className: "grammar-state-selected"
  },
  {
    id: "disabled",
    label: "Disabled",
    usage: "Muted text and blocked action without hiding context.",
    className: "grammar-state-disabled"
  },
  {
    id: "warning",
    label: "Warning",
    usage: "Amber rule and fill reserved for holds and degraded evidence.",
    className: "grammar-state-warning"
  },
  {
    id: "current",
    label: "Current",
    usage: "Present read position in a sequence or process.",
    className: "grammar-state-current"
  }
];

export const valueBasisSpecs: Record<WorkbenchValueBasis, ValueBasisSpec> = {
  measured: {
    id: "measured",
    label: "Measured State",
    usage: "Direct observed state from resident sources.",
    className: "basis-measured",
    iconLabel: "sensor wave",
    ruleStyle: "solid",
    texture: "plain measured stripe"
  },
  imputed: {
    id: "imputed",
    label: "Imputed State",
    usage: "Twin-derived estimate from measured or model inputs.",
    className: "basis-imputed",
    iconLabel: "model branch",
    ruleStyle: "double",
    texture: "crosshatch estimate field"
  },
  simulated: {
    id: "simulated",
    label: "Simulated Result State",
    usage: "Run-scoped scientific output and scenario result.",
    className: "basis-simulated",
    iconLabel: "result box",
    ruleStyle: "dashed",
    texture: "dashed run track"
  }
};

export const visualGrammarSpec = {
  density: "review",
  radiusPx: 8,
  panelGapPx: 14,
  darkSpatialRule: "Use dark spatial treatment for topology, status bays, and terminals only.",
  semanticColorTokens,
  typeScaleTokens,
  ruleStyleTokens,
  targetSizeTokens,
  interactionStateSpecs,
  valueBasisSpecs
} as const;

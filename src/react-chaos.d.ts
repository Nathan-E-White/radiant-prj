declare module "react-chaos" {
  import type { ComponentType } from "react";

  export default function withChaos<Props extends object>(
    component: ComponentType<Props>,
    level?: number,
    errorMessage?: string,
    runInProduction?: boolean
  ): ComponentType<Props>;
}

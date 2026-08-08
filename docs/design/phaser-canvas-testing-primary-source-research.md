# Phaser Canvas Testing: Primary-Source Research

## Scope and status

This note answers whether there is a more sophisticated Phaser testing strategy than brittle Playwright canvas screenshots. It is research for engineering decisions, not implementation evidence. Sources are primary: Phaser documentation/source, Playwright documentation, HTML/Canvas standards, MDN API documentation, and Vitest documentation where relevant because Phaser's own package scripts use Vitest.

Primary sources used:

- Phaser, [`HEADLESS` constant](https://docs.phaser.io/api-documentation/constant/phaser), [`GameConfig`](https://docs.phaser.io/api-documentation/typedef/types-core), [Game concepts](https://docs.phaser.io/phaser/concepts/game), [`Game#headlessStep`](https://docs.phaser.io/api-documentation/class/game), [Scenes concepts](https://docs.phaser.io/phaser/concepts/scenes), [Data Manager](https://docs.phaser.io/phaser/concepts/data-manager), and [RandomDataGenerator](https://docs.phaser.io/phaser/concepts/math).
- Phaser source repository, [`package.json`](https://github.com/phaserjs/phaser/blob/master/package.json), [v4.0.0 changelog](https://github.com/phaserjs/phaser/blob/master/changelog/v4/4.0/CHANGELOG-v4.0.0.md), and [Phaser examples README](https://github.com/phaserjs/examples).
- Playwright, [locators](https://playwright.dev/docs/locators), [visual comparisons](https://playwright.dev/docs/next/test-snapshots), [`PageAssertions#toHaveScreenshot`](https://playwright.dev/docs/api/class-pageassertions), [screenshots](https://playwright.dev/docs/screenshots), [ARIA snapshots](https://playwright.dev/docs/api/class-pageassertions), and [test CLI snapshot update flags](https://playwright.dev/docs/test-cli).
- WHATWG HTML, [`canvas`](https://html.spec.whatwg.org/multipage/canvas.html) and [focusable areas](https://html.spec.whatwg.org/multipage/interaction.html).
- MDN, [basic canvas usage and fallback content](https://developer.mozilla.org/en-US/docs/Web/API/Canvas_API/Tutorial/Basic_usage), [`CanvasRenderingContext2D#getImageData`](https://developer.mozilla.org/en-US/docs/Web/API/CanvasRenderingContext2D/getImageData), [cross-origin images in canvas](https://developer.mozilla.org/en-US/docs/Web/HTML/How_to/CORS_enabled_image), and [`OffscreenCanvasRenderingContext2D`](https://developer.mozilla.org/en-US/docs/Web/API/OffscreenCanvasRenderingContext2D).
- Vitest, [test environments](https://main.vitest.dev/guide/environment), [Browser Mode](https://main.vitest.dev/guide/browser/), [component testing](https://main.vitest.dev/guide/browser/component-testing), and [fake timers](https://vitest.dev/guide/mocking/timers).

## Bottom line

There is no single first-party Phaser replacement for Playwright screenshots that makes canvas-rendered game objects queryable like DOM nodes. Phaser provides lower-level hooks that support more deterministic testing: `Phaser.HEADLESS`, explicit game configuration, boot callbacks, the game registry, scene lifecycle controls, seeded randomness, and `headlessStep`. Those APIs are enough to build a local test harness for scene/system behavior, but the harness is application-owned. Phaser does not document a first-party scene assertion DSL, golden-image manager, Playwright adapter, or DOM-style locator system for rendered game objects. [Phaser HEADLESS](https://docs.phaser.io/api-documentation/constant/phaser) [GameConfig](https://docs.phaser.io/api-documentation/typedef/types-core) [Game#headlessStep](https://docs.phaser.io/api-documentation/class/game) [Phaser package scripts](https://github.com/phaserjs/phaser/blob/master/package.json)

The practical professional pattern is therefore layered:

| Layer | Best evidence | Why |
| --- | --- | --- |
| Domain rules and projections | Unit tests over pure TypeScript models | Fast, deterministic, no renderer. |
| React controls and navigator | Role/label/text/test-id assertions | Playwright recommends user-facing locators for resilient DOM tests. [Playwright locators](https://playwright.dev/docs/locators) |
| Phaser scene behavior | App-owned instrumentation plus headless or real-browser runtime tests | Phaser exposes scene/game lifecycle APIs, but the test contract must be designed locally. [Scenes](https://docs.phaser.io/phaser/concepts/scenes) [Game concepts](https://docs.phaser.io/phaser/concepts/game) |
| Render smoke | Pixel sampling or scoped element screenshot | Canvas drawing is bitmap output; pixel APIs and screenshots are the browser-owned proof. [MDN getImageData](https://developer.mozilla.org/en-US/docs/Web/API/CanvasRenderingContext2D/getImageData) [Playwright visual comparisons](https://playwright.dev/docs/next/test-snapshots) |
| Visual regression | One or a few constrained screenshots with pinned environment and tolerance | Playwright warns rendering varies by OS, browser, hardware, settings, and headless mode. [Playwright visual comparisons](https://playwright.dev/docs/next/test-snapshots) |

## Phaser unit, headless, and runtime options

Phaser's `HEADLESS` renderer creates neither a Canvas nor WebGL renderer, still requires a DOM, and is explicitly described as meant for unit testing rather than server-side Phaser execution. That makes it useful for exercising scene lifecycle, update logic, registry/data changes, timers, input-adjacent code that can be invoked directly, and app instrumentation, but not for proving that visible canvas pixels are correct. [Phaser HEADLESS](https://docs.phaser.io/api-documentation/constant/phaser)

`GameConfig.type` accepts `AUTO`, `CANVAS`, `HEADLESS`, or `WEBGL`; `GameConfig.scene` can install scenes; `GameConfig.seed` seeds Phaser's predefined RNG; `GameConfig.input` can disable game input; and `GameConfig.callbacks.preBoot/postBoot` can run setup once the boot sequence reaches the documented points. These are the configuration levers for a deterministic Phaser test harness. [GameConfig](https://docs.phaser.io/api-documentation/typedef/types-core) [Game concepts](https://docs.phaser.io/phaser/concepts/game)

For `HEADLESS`, Phaser exposes `Game#headlessStep(time, delta)`, described as a special version of the game step for the headless renderer. It updates global managers and each Scene and emits prerender/postrender events even though nothing displays. That is the most direct first-party hook for manual stepping in a non-visual scene/system test. [Game#headlessStep](https://docs.phaser.io/api-documentation/class/game)

Phaser's own source package currently defines `test` as `vitest run`, `test:watch` as `vitest`, includes `jsdom` and `vitest` in dev dependencies, and does not advertise a separate official visual-regression or E2E scene-testing command in `package.json`. That is evidence of Phaser using ordinary JS test infrastructure for its package, not evidence of a first-party application-level test harness for Phaser games. [Phaser package scripts](https://github.com/phaserjs/phaser/blob/master/package.json)

Phaser's examples repository is positioned as source examples and a local examples browser, not as a test harness. Its README says the examples can be browsed on the labs site or cloned locally for testing while developing with Phaser, but it does not define a formal assertion framework for game scenes. [Phaser examples README](https://github.com/phaserjs/examples)

## Scene and system testing strategies

Scene tests should treat Phaser as a runtime coordinator and keep the domain state outside the Scene when possible. Phaser's Scene docs distinguish Run, Pause, Sleep, and Stop states, and document operations such as `start`, `launch`, `pause`, `resume`, `sleep`, `wake`, `stop`, and `restart`; this gives tests stable lifecycle transitions to drive rather than relying on opaque frame timing. [Scenes](https://docs.phaser.io/phaser/concepts/scenes)

Phaser's Game docs say the game instance is available to scenes as `this.game`; the game and scenes expose event emitters; and the game registry is a global data store. A harness can therefore install test-only event listeners or registry values in `preBoot/postBoot`, observe scene events, and assert registry snapshots without reading pixels. [Game concepts](https://docs.phaser.io/phaser/concepts/game)

Phaser's Data Manager exists on the Game registry, every Scene, and Game Objects once enabled; it supports `set`, `get`, `getAll`, events for data changes, and custom Data Manager instances. This is a first-party place to expose test state, provided the app treats it as an explicit test/diagnostic contract rather than letting tests inspect private scene internals freely. [Data Manager](https://docs.phaser.io/phaser/concepts/data-manager)

Seeded scenarios should use Phaser's documented RNG controls or an app-owned deterministic model. Phaser documents `GameConfig.seed`, `Phaser.Math.RND`, runtime `rnd.init(seed)`, and `new Phaser.Math.RandomDataGenerator(seed)`. For Radiant, the better first layer remains the existing deterministic Fleet Board domain session; Phaser seeding is useful only for randomness that actually occurs inside Phaser. [RandomDataGenerator](https://docs.phaser.io/phaser/concepts/math)

Vitest is a reasonable first-party-adjacent option because Phaser itself uses it. Vitest's default environment is Node; `jsdom` and `happy-dom` emulate browser APIs; Browser Mode runs tests in real browsers through providers including Playwright; and fake timers can advance timeout/interval-driven code without waiting wall-clock time. This supports three harness tiers: pure Node domain tests, jsdom/light DOM component tests, and real-browser component/runtime tests when layout, canvas, focus, or browser APIs matter. [Phaser package scripts](https://github.com/phaserjs/phaser/blob/master/package.json) [Vitest environments](https://main.vitest.dev/guide/environment) [Vitest Browser Mode](https://main.vitest.dev/guide/browser/) [Vitest fake timers](https://vitest.dev/guide/mocking/timers)

## Canvas pixel and screenshot testing

Canvas is not DOM. MDN describes `<canvas>` as a fixed-size drawing surface that exposes rendering contexts; fallback content inside the element is for users and tools that cannot experience the canvas, while the browser normally renders the bitmap surface. Playwright locators can prove the `<canvas>` element exists, but they cannot locate Phaser's drawn reactor, route, or slot rail as DOM elements unless the app also exposes those as DOM or accessibility structures. [MDN canvas basics](https://developer.mozilla.org/en-US/docs/Web/API/Canvas_API/Tutorial/Basic_usage) [Playwright locators](https://playwright.dev/docs/locators)

For pixel smoke tests, `CanvasRenderingContext2D#getImageData()` returns an `ImageData` object for a rectangle of the canvas's underlying pixel data. That supports checks like "nonblank canvas" or "this sampled rail region changed," but it is still a low-level bitmap assertion, not a semantic scene query. [MDN getImageData](https://developer.mozilla.org/en-US/docs/Web/API/CanvasRenderingContext2D/getImageData)

Pixel reads can fail when a canvas is tainted by cross-origin image content: MDN documents that `getImageData`, `toBlob`, `toDataURL`, and `captureStream` throw `SecurityError` on a tainted canvas. Radiant's current local generated assets avoid this class of failure; external art/CDN images would need CORS discipline before tests could read pixels reliably. [MDN cross-origin canvas](https://developer.mozilla.org/en-US/docs/Web/HTML/How_to/CORS_enabled_image)

Playwright screenshots remain the first-party browser-test tool for visual regression. Playwright says `toHaveScreenshot()` repeatedly captures until two consecutive screenshots match, stores PNG baselines, and compares subsequent runs to those baselines. It also warns that rendering can vary by OS, browser version, settings, hardware, power source, and headless mode, and recommends running screenshots in the same environment as baseline generation. [Playwright visual comparisons](https://playwright.dev/docs/next/test-snapshots)

Playwright supports scoped element screenshots, screenshot buffers for custom processing, `maxDiffPixels`, `maxDiffPixelRatio`, `threshold`, `animations`, `caret`, masks, and screenshot-only stylesheets. These controls are useful for reducing brittleness, but they do not make a screenshot semantic; they only make bitmap comparison more repeatable and better scoped. [Playwright screenshots](https://playwright.dev/docs/screenshots) [PageAssertions#toHaveScreenshot](https://playwright.dev/docs/api/class-pageassertions)

Playwright's snapshot naming includes browser/project and platform such as `chromium-darwin`; the docs state that screenshots differ between browsers and platforms and may need separate snapshots. That is why a Linux CI baseline can be necessary even when a Darwin baseline exists. [Playwright visual comparisons](https://playwright.dev/docs/next/test-snapshots)

## Deterministic render controls

Deterministic Phaser tests should lock scenario data, clock/frame stepping, viewport, camera, motion, and renderer choice. Phaser provides seed configuration, explicit game dimensions, renderer type, `fps` configuration, and `headlessStep`; Playwright provides viewport/device configuration through the test project and screenshot options for animations/caret/masks/style injection. [GameConfig](https://docs.phaser.io/api-documentation/typedef/types-core) [Game#headlessStep](https://docs.phaser.io/api-documentation/class/game) [PageAssertions#toHaveScreenshot](https://playwright.dev/docs/api/class-pageassertions)

Phaser's TimeStep docs expose configuration such as `fps.limit`, `forceSetTimeOut`, and delta smoothing history. These controls can reduce runtime variability in low-intensity scenes, but they do not replace manual stepping or semantic assertions when a test needs deterministic state transitions. [TimeStep](https://docs.phaser.io/api-documentation/class/core-timestep)

Phaser v4 replaced the v3 WebGL pipeline system with render nodes and deprecates the Canvas renderer in the v4 changelog. Tests that rely on Canvas 2D internals should therefore be kept small and app-owned; Playwright screenshots remain renderer-agnostic because they capture what the browser presents. [Phaser v4 changelog](https://github.com/phaserjs/phaser/blob/master/changelog/v4/4.0/CHANGELOG-v4.0.0.md) [Playwright screenshots](https://playwright.dev/docs/screenshots)

OffscreenCanvas is a browser API for drawing to a bitmap off the main DOM canvas, including in workers, and can be useful for custom rendering tests. It is not a Phaser testing feature in the cited Phaser docs, and MDN notes differences from the normal 2D context, including no support for UI features such as `drawFocusIfNeeded`. Treat it as a specialized browser primitive, not an immediate replacement for current Phaser runtime tests. [MDN OffscreenCanvasRenderingContext2D](https://developer.mozilla.org/en-US/docs/Web/API/OffscreenCanvasRenderingContext2D)

## Test hooks and instrumentation

The strongest alternative to brittle screenshots is an explicit app-owned scene contract. Phaser's documented registry, Scene data manager, Game and Scene event emitters, boot callbacks, and Scene lifecycle controls give Radiant enough surface to expose "rendered scene ready," selected reactor id, camera state, route count, slot rail state, and sprite/object counts as diagnostics. Tests can then assert that the Phaser runtime consumed the domain model without overfitting to pixels. [Game concepts](https://docs.phaser.io/phaser/concepts/game) [Data Manager](https://docs.phaser.io/phaser/concepts/data-manager) [Scenes](https://docs.phaser.io/phaser/concepts/scenes)

The HTML Standard separately recommends that interactive canvas regions have focusable fallback content with a one-to-one mapping to focusable canvas regions. For Radiant, the Board Navigator is not only a test hook; it is also the standards-aligned non-canvas representation of interactive board state. It should carry the semantic assertions for facilities, routes, pawns, reactor slot rails, selected reactor, and available commands. [WHATWG canvas](https://html.spec.whatwg.org/multipage/canvas.html) [WHATWG focusable areas](https://html.spec.whatwg.org/multipage/interaction.html)

Playwright recommends role, text, label, and explicit test-id locators before CSS/XPath because those locators align with user-facing attributes or explicit contracts and are less coupled to DOM structure. Therefore the Board Scene Workbench should keep DOM assertions on controls and navigator semantics, not XPath chains into markup shape. [Playwright locators](https://playwright.dev/docs/locators)

Playwright ARIA snapshots can assert accessibility-tree structure. They are useful for the non-canvas workbench controls and Board Navigator, but they cannot inspect Phaser game objects unless those objects are represented in fallback/DOM/accessibility content. [PageAssertions#toMatchAriaSnapshot](https://playwright.dev/docs/api/class-pageassertions) [WHATWG canvas](https://html.spec.whatwg.org/multipage/canvas.html)

## What Phaser does not provide first-party

The cited Phaser documentation and source metadata show these gaps:

- No documented first-party DOM-style locator API for Phaser game objects. Phaser game objects are rendered through Phaser's renderer, not exposed as DOM nodes. [MDN canvas basics](https://developer.mozilla.org/en-US/docs/Web/API/Canvas_API/Tutorial/Basic_usage) [Phaser v4 changelog](https://github.com/phaserjs/phaser/blob/master/changelog/v4/4.0/CHANGELOG-v4.0.0.md)
- No documented first-party visual-regression system comparable to Playwright `toHaveScreenshot`; Phaser's source package uses Vitest scripts, and its examples repository is examples-oriented. [Phaser package scripts](https://github.com/phaserjs/phaser/blob/master/package.json) [Phaser examples README](https://github.com/phaserjs/examples)
- No documented official Scene assertion DSL. Phaser exposes Scenes, lifecycle operations, events, registry/data, and `headlessStep`; composing those into assertions is the application's responsibility. [Scenes](https://docs.phaser.io/phaser/concepts/scenes) [Game#headlessStep](https://docs.phaser.io/api-documentation/class/game) [Data Manager](https://docs.phaser.io/phaser/concepts/data-manager)
- No first-party guarantee that headless mode proves rendering. Phaser explicitly says `HEADLESS` creates neither Canvas nor WebGL renderer. [Phaser HEADLESS](https://docs.phaser.io/api-documentation/constant/phaser)

## Recommendations for Radiant's Board Scene Workbench

1. Keep domain tests as the main truth for board state. `buildFleetBoardSceneModel`, `buildBoardNavigator`, and `resolveBoardSceneWorkbenchView` should prove scenario seed/day selection, route display labels, reactor slot rails, selected reactor, reduced-motion flag, and camera settings without Phaser. This matches the fact that Phaser's headless/renderer APIs do not make rendered game objects semantically queryable. [Phaser HEADLESS](https://docs.phaser.io/api-documentation/constant/phaser)

2. Keep DOM/component tests for the review surface. Controls and the Board Navigator should be asserted with role/label/text/test-id queries: scenario buttons, seed/day inputs, selected reactor select, density mode, reduced-motion toggle, and visible route rows. This follows Playwright locator guidance and the HTML canvas fallback principle. [Playwright locators](https://playwright.dev/docs/locators) [WHATWG canvas](https://html.spec.whatwg.org/multipage/canvas.html)

3. Add a small Phaser runtime diagnostic contract before adding more screenshots. A test-only or diagnostic callback can expose "scene ready," selected reactor, camera zoom/pan, reduced-motion tween state, facility count, route count, and rail count through `data-*` attributes, a registry snapshot, or a window-scoped harness in e2e fixtures. Phaser's registry/events/data APIs support this; the contract should be narrow and documented so tests do not rummage through private scene implementation. [Game concepts](https://docs.phaser.io/phaser/concepts/game) [Data Manager](https://docs.phaser.io/phaser/concepts/data-manager)

4. Use `HEADLESS` only for non-rendering scene/system tests. It can manually step scene logic with `headlessStep`, but it cannot prove sprite placement, route drawing, scaling, textures, or broken WebGL/Canvas presentation because no renderer is created. [Phaser HEADLESS](https://docs.phaser.io/api-documentation/constant/phaser) [Game#headlessStep](https://docs.phaser.io/api-documentation/class/game)

5. Keep one scoped visual regression for the workbench canvas. The screenshot should stay element-scoped, use reduced motion, disable animations/caret through Playwright, pin CI to the baseline environment, and use one small threshold style. That preserves coverage for blank canvas, missing layers, broken camera scale, or asset failures without asking pixels to prove every board rule. [PageAssertions#toHaveScreenshot](https://playwright.dev/docs/api/class-pageassertions) [Playwright visual comparisons](https://playwright.dev/docs/next/test-snapshots)

6. Prefer pixel sampling over full-image diff for specific runtime behaviors. Current tests that sample a known rail/camera region are a legitimate middle ground: they verify the canvas changed in a domain-relevant area while avoiding a full layout/image baseline. Use `getImageData` for same-origin/local assets only, and fall back to screenshots if WebGL or tainting prevents 2D pixel reads. [MDN getImageData](https://developer.mozilla.org/en-US/docs/Web/API/CanvasRenderingContext2D/getImageData) [MDN cross-origin canvas](https://developer.mozilla.org/en-US/docs/Web/HTML/How_to/CORS_enabled_image)

7. Consider Vitest Browser Mode only if component-level Phaser harnesses become too slow under Playwright e2e. Vitest Browser Mode can run tests in real browsers through Playwright/WebdriverIO providers and is positioned for accurate component testing, but adopting it would add a new lane. It is an alternative for browser-native component/runtime tests, not a reason to remove Playwright acceptance coverage. [Vitest Browser Mode](https://main.vitest.dev/guide/browser/) [Vitest component testing](https://main.vitest.dev/guide/browser/component-testing)

## Proposed Board Scene testing split

| Concern | Recommended lane |
| --- | --- |
| Scenario catalog, seed/day clamp, selected reactor normalization | Domain unit tests |
| Route labels and navigator grouping | Domain and React static/component tests |
| Controls are real controls | React component tests plus Playwright role/label assertions |
| Runtime consumes camera/reduced-motion controls | Phaser runtime diagnostic assertion in Playwright |
| Canvas is nonblank after mount/update | Pixel smoke with `getImageData` or screenshot fallback |
| Overall board scene remains visually recognizable | One scoped Playwright canvas screenshot with CI baseline |
| Accessibility and inspectability | Board Navigator DOM assertions and optional ARIA snapshot |

## Decision implication

The sophisticated solution is not "replace screenshots with one better Phaser tool." The source-backed solution is a test pyramid around the canvas: pure deterministic models, accessible DOM projections, app-owned Phaser diagnostics, targeted pixel probes, and a small number of visual snapshots. Screenshots remain necessary for renderer failures, but they should be the last layer, not the primary proof that the game state is correct.

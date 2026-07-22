# Product-experience primary-source research

## Purpose and limits

This note grounds a product-design sweep of Radiant in primary standards and first-party tool guidance. It covers graphic design, user experience (UX), developer experience (DX) for extending and validating the frontend, and interaction design (IxD). It does **not** assess system, infrastructure, or code architecture, and it does not claim WCAG conformance.

The observations are based on the current `AppShell`, Simulator Workbench, Twin Viewport, Fleet Board, global stylesheet, Storybook configuration, and Playwright tests. “Sourced principle” below means the source states the principle. “Radiant interpretation” is an application of that principle to this repository and should be validated with rendered inspection, keyboard/screen-reader testing, and representative users.

## Current product surface

Radiant is not one dashboard. It currently puts several different modes of thought into one page:

- a three-section shell (`Welcome`, `Status Workbench`, `Evidence`);
- a provenance-sensitive engineering review surface that keeps Measured State, Imputed State, Simulated Result State, and Lineage distinct;
- a selectable Kaleidos Fleet strip and SVG Twin Viewport;
- a playable, canvas-rendered Fleet Board with build, time-advance, refueling, simulation-capacity, and event-log interactions;
- container-orchestration and synthetic HPC status panels;
- one Storybook story family (Simulation Health Summary) and seven Playwright flows, currently configured around a single 1440 × 980 viewport.

That breadth is the central design constraint. The interface has to preserve provenance and public-safe boundaries without making every fact look equally urgent. More panels are not, by themselves, a navigation system.

## Primary-source findings and Radiant implications

### 1. Meaning needs more than color

**Sourced principle.** WCAG 2.2 Success Criterion 1.4.1 says color must not be the only visual means of conveying information, action, or state. Its explanatory guidance recommends adding shape or text. WCAG 1.4.11 separately requires sufficient contrast for visual information needed to identify controls, states, and meaningful graphics. [W3C: Use of Color](https://www.w3.org/WAI/WCAG22/Understanding/use-of-color.html) · [W3C: Non-text Contrast](https://www.w3.org/WAI/WCAG22/Understanding/non-text-contrast)

**Radiant interpretation.** The Workbench does include text labels for the three Value Bases, which is a sound start. However, blue/green/orange borders and fills carry much of the at-a-glance grammar across values, viewport overlays, lineage steps, job bars, and status badges. A graphic-design sweep should define a redundant visual language: stable icon or texture, explicit short label, and position in addition to hue. Measured State might use a sensor glyph and solid rule; Imputed State a model glyph and dotted rule; Simulated Result State a run/artifact glyph and segmented rule. This also prevents the same warm color from ambiguously meaning simulation, warning, commercial context, and game pressure.

### 2. Hierarchy should express task priority, not panel count

**Sourced principle.** Apple’s layout guidance recommends placing items according to relative importance, following reading order, aligning related elements for scanability, differentiating controls from content, and adapting gracefully across window, text-size, and locale changes. Its color guidance recommends using color consistently and avoiding one color for multiple meanings. [Apple: Layout](https://developer.apple.com/design/human-interface-guidelines/layout) · [Apple: Color](https://developer.apple.com/design/human-interface-guidelines/color)

**Radiant interpretation.** The Status Workbench presents the read status, selected unit context, Simulation Health Summary, fleet selector, Fleet Board, three Value Basis regions, Lineage, two terminals, orchestration, queue, and four HPC panels in a long sequence. The screen has local hierarchy within cards, but weak hierarchy between user goals. A design sweep should establish three levels:

1. **Decision level:** what changed, whether the Workbench Snapshot is trustworthy, and what needs review.
2. **Explanation level:** selected value, Value Basis, freshness, confidence, and Lineage.
3. **Exploration level:** Fleet Board play, orchestration detail, synthetic HPC diagnostics, and event history.

This is a visual and product-ordering recommendation, not a proposal to rearrange source modules.

### 3. Reflow must be treated as an information-design requirement

**Sourced principle.** WCAG 1.4.10 requires content to remain available and functional at a width equivalent to 320 CSS pixels without two-dimensional scrolling, except where a two-dimensional layout is essential. Even where the exception applies, W3C recommends reducing the scrolling burden within that content. [W3C: Reflow](https://www.w3.org/WAI/WCAG21/Understanding/reflow.html)

**Radiant interpretation.** The stylesheet deliberately creates horizontal rails for tabs, fleet cards, queue rows, requirements, Lineage, and explanation cards. The Fleet Board keeps a three-column layout with a `minmax(660px, 1fr)` canvas column, even below the current responsive breakpoints. Some horizontal presentation may be essential for the board or a timeline, but the surrounding reading experience is not. At narrow widths or 400% zoom, users should receive a coherent single-column review path, an explicit “open spatial view” action for genuinely two-dimensional content, and compact summaries that do not require hunting across multiple independent horizontal scrollers.

### 4. Focus must be unmistakable and selection must remain stable

**Sourced principle.** WCAG requires visible keyboard focus; its Focus Appearance guidance uses a minimum 3:1 change-of-contrast measure and encourages authors to exceed the minimum. The ARIA keyboard-interface guidance recommends following familiar platform key conventions within composite widgets and maintaining a visually persistent focus indicator. [W3C: Focus Appearance](https://www.w3.org/WAI/WCAG22/Understanding/focus-appearance.html) · [W3C APG: Developing a Keyboard Interface](https://www.w3.org/WAI/ARIA/apg/practices/keyboard-interface/)

**Radiant interpretation.** The SVG Twin Viewport has a specific `:focus-visible` treatment, but the global button system mostly defines hover and selected states, not a consistent focus treatment. On the Fleet Strip and value cards, hover and selection also share visual styling. The sweep should make **hover**, **keyboard focus**, **selected**, **warning**, and **disabled** five deliberately distinct states across light and dark surfaces. Focus should never disappear when the selected value updates another part of the page.

### 5. The shell looks like tabs and should behave like tabs—or stop looking like them

**Sourced principle.** The ARIA Authoring Practices tabs pattern specifies a `tablist`, `tab`, and `tabpanel` relationship; an active tab exposes `aria-selected`; each tab identifies its panel; and Left/Right Arrow navigate a horizontal tab list. [W3C APG: Tabs Pattern](https://www.w3.org/WAI/ARIA/apg/patterns/tabs/)

**Radiant interpretation.** The three shell sections are visually called tabs, but are currently ordinary buttons inside a `nav`, with no selected state exposed to assistive technology and no tab keyboard model. There are two legitimate designs: implement the complete tabs pattern, or present them as three page-level navigation commands with an explicit current-page indication. The half-tab/half-navigation state is the awkward one.

### 6. Canvas play requires equivalent operability and understandable state

**Sourced principle.** The HTML Standard says interactive canvas regions should have a one-to-one mapping to focusable fallback content so the interaction can be keyboard-accessible. Microsoft’s Xbox Accessibility Guideline 107 calls for game interfaces to support the user’s input mechanism of choice and remain operable with digital input. XAG 112 calls for consistent, intuitive UI navigation and consistent relative order of repeated controls. [WHATWG: Canvas](https://html.spec.whatwg.org/multipage/canvas.html) · [Microsoft XAG 107: Input](https://learn.microsoft.com/en-us/xbox/accessibility/xbox-accessibility-guidelines/107) · [Microsoft XAG 112: UI Navigation](https://learn.microsoft.com/en-us/xbox/accessibility/xbox-accessibility-guidelines/112)

**Radiant interpretation.** Fleet Board has HTML controls beside the canvas for fixed starter placements and simulation actions, but facilities drawn in Phaser are not represented by fallback content in `FleetBoardCanvas`. A player who cannot point into the canvas does not have an equivalent way to inspect every tile, select every reactor, or understand board occupancy. The product needs a synchronized non-canvas board navigator—grid/list, keyboard cursor, or both—that exposes tile coordinates, facility identity, selectable reactors, available actions, and current focus. This should be considered a core IxD feature rather than accessibility garnish.

### 7. Dynamic state should announce the useful delta, not the whole dashboard

**Sourced principle.** WCAG 4.1.3 requires status messages to be programmatically determinable without moving focus. W3C’s failure guidance notes that a dynamic status which must be manually rediscovered fails this requirement. [W3C: Status-message failure F103](https://www.w3.org/WAI/WCAG22/Techniques/failures/F103)

**Radiant interpretation.** Radiant uses `role="status"` for Workbench read state and several `aria-live="polite"` resource summaries. That is directionally correct, but one Tick Day can change multiple live regions plus the event log and canvas. The interaction design should announce one concise transaction result—for example, “Day advanced to 8: Reactor K-02 entered cooldown; score +4; 1 new event”—and let users inspect a persistent change log. Live regions should report deltas; visible panels should preserve the full state.

### 8. Pointer targets and dense text need a deliberate operating profile

**Sourced principle.** WCAG 2.5.8 sets a 24 × 24 CSS-pixel minimum target or equivalent spacing at Level AA; its guidance recommends larger targets for important controls. Microsoft’s game text guidance treats readable default sizing and configurable display options as prerequisites for people with low vision and for situational use at distance. [W3C: Target Size (Minimum)](https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum) · [Microsoft XAG 101: Text Display](https://learn.microsoft.com/gaming/accessibility/xbox-accessibility-guidelines/101)

**Radiant interpretation.** Many controls meet 34–38px minimum height, but the UI also relies heavily on 0.66–0.78rem text and dense metadata chips. Those values may technically render, but a screen-share workbench and board game are both distance-viewing scenarios. The sweep should define at least two density modes—**review** and **compact**—with the review mode using larger text, fewer simultaneous facts, and at least 44px targets for high-frequency game and navigation actions. This is particularly important for Tick Day, reactor selection, and value inspection.

### 9. Motion and time progression need user control

**Sourced principle.** The `prefers-reduced-motion` media feature reports a user preference to reduce or replace non-essential motion. Microsoft’s accessibility guidelines separately include visual-distraction and motion settings. [MDN: `prefers-reduced-motion`](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/At-rules/%40media/prefers-reduced-motion) · [Microsoft: Xbox Accessibility Guidelines](https://learn.microsoft.com/en-us/xbox/accessibility/guidelines)

**Radiant interpretation.** Current CSS has little overt animation, but Fleet Board and future live Snapshot transitions invite motion. Any board movement, pulsing freshness warning, topology animation, or “live” sweep should have a reduced-motion equivalent and a pause mechanism. Time advance should remain an explicit transaction with a reviewable outcome, not become an ambient animation that can outrun comprehension.

## Frontend DX findings

### 10. The state catalogue is much smaller than the product state space

**Sourced principle.** Storybook treats stories as UI test cases for distinct states. Its play functions can simulate user behavior and assert the resulting state, and its interaction panel can pause, rewind, and step through those interactions. [Storybook: UI testing](https://storybook.js.org/docs/writing-tests/index) · [Storybook: Interaction tests](https://storybook.js.org/docs/9/writing-tests/interaction-testing)

**Radiant interpretation.** The repository has Storybook configured but only four stories, all for Simulation Health Summary. Extending the frontend requires developers to reconstruct complex states by running the full app and knowing which fixture or sequence reveals them. The missing DX surface is a product-state catalogue: Workbench Snapshot phases; all Value Bases; fresh/stale/missing Lineage; fleet phase combinations; empty/full Reactor Slot Rail; job queued/running/completed; event pressure; narrow/zoomed layouts; and keyboard focus states. Stories should be named in domain language, not implementation language.

### 11. Visual and accessibility regressions are not yet first-class review artifacts

**Sourced principle.** Storybook’s official guidance says its accessibility addon audits rendered DOM and can run with component tests, while noting that automated checks are only a first line of QA. Its visual-testing guidance describes baseline screenshots for detecting changes in layout, color, size, and contrast. Playwright supports screenshot comparison, accessibility-tree snapshots, and axe integration, while explicitly recommending automated, manual, and inclusive-user testing together. [Storybook: Accessibility tests](https://storybook.js.org/docs/writing-tests/accessibility-testing) · [Storybook: Visual tests](https://storybook.js.org/docs/8/writing-tests/visual-testing) · [Playwright: Visual comparisons](https://playwright.dev/docs/test-snapshots) · [Playwright: ARIA snapshots](https://playwright.dev/docs/aria-snapshots) · [Playwright: Accessibility testing](https://playwright.dev/docs/accessibility-testing)

**Radiant interpretation.** Current Playwright tests verify behavior and canvas non-blankness at one desktop viewport; Storybook has no listed a11y addon or interaction tests. A frontend change can therefore preserve clicks while degrading hierarchy, focus, reflow, semantics, or color meaning. The DX should turn each product state into a three-part review artifact: rendered image, accessible structure, and interaction trace. Automated results should flag regressions, while manual keyboard, zoom, contrast, and screen-reader checks remain explicit release work.

## Source-backed feature directions

These are design hypotheses derived from the standards above, not features required by those standards.

### UX candidates

1. **Review Lens.** A task-level mode that reduces the Status Workbench to “what changed,” “can I trust it,” and “why,” with expandable Snapshot, Value Basis, and Lineage evidence. This applies visual-hierarchy and status-delta guidance while preserving Radiant’s provenance model.
2. **Snapshot Compare.** A two-generation review that highlights changed values, changed Value Basis, freshness movement, and Lineage additions/removals. It would turn “Refresh live Snapshot” from a blind replacement into an intelligible review transaction.
3. **Public-safe Boundary Inspector.** A persistent explainer that answers “what this value is,” “what it is not,” and “which source/run/model produced it.” This converts the glossary’s careful exclusions into usable product comprehension rather than scattered caveats.

### DX candidates

1. **Experience Contract Gallery.** A Storybook catalogue generated around domain states and interaction sequences, with density, viewport, contrast, reduced-motion, and Value Basis variants. Each story becomes a shared design/test fixture rather than a decorative component demo.
2. **Three-snapshot review harness.** For each critical flow, capture the visual screenshot, ARIA snapshot, and domain-relevant result text after the same Playwright interaction. Reviewers then see what changed visually, semantically, and behaviorally in one place.
3. **Design-token specimen and contrast ledger.** A living story that lists each semantic color, text size, target size, focus state, icon/texture, and permitted meaning, with automated contrast checks. This would make the redundant Value Basis grammar straightforward to extend.

### IxD candidates

1. **Linked inspection cursor.** Selecting or focusing a unit, viewport entity, value, or Lineage step highlights the same concept everywhere, preserves focus, and provides Next/Previous commands. The current panels become coordinated views of one selection rather than separate click targets.
2. **Board navigator with canvas parity.** A synchronized grid/list representation of every Fleet Board tile and facility, with arrow-key movement, named actions, current tile context, and canvas focus indication. It supplies the one-to-one operability the canvas standard anticipates.
3. **Transactional day stepper.** Preview the consequences of advancing a day, apply once, announce a concise delta, and offer review/undo where game rules permit. This reduces accidental repeated actions and makes simulation time legible.
4. **Change ribbon.** After Snapshot refresh, unit change, job transition, or Tick Day, show a persistent ordered ribbon of additions, removals, and state changes. Selecting an item moves the linked inspection cursor to the affected unit/value/facility without discarding the user’s place.
5. **Command and shortcut layer.** A discoverable command palette for switch section, choose unit, choose Value Basis, inspect Lineage, advance day, and open board navigator, with user-remappable single-key alternatives where appropriate. The visible shortcut labels must update when mappings change, following XAG input guidance.

## Practical acceptance checks for the design sweep

The following checks are interpretations, not a conformance claim:

- Every status and Value Basis remains identifiable in grayscale and at increased contrast.
- Every interactive state has distinct hover, focus, selected, disabled, and warning treatments.
- At 400% zoom, all non-spatial reading and command tasks work without two-dimensional scrolling; spatial views have a coherent equivalent navigator.
- All shell-section navigation works with the chosen, standards-consistent keyboard model.
- Every canvas action and selection is available with keyboard/digital input and exposed in an accessible structure.
- A state change produces one concise announcement and a persistent, inspectable delta.
- Critical targets are at least 24px with adequate spacing, with frequent review/game commands targeting 44px.
- Non-essential motion respects reduced-motion preference and live progression can be paused.
- Each critical domain state has a story; each critical flow has visual, ARIA, and interaction evidence.
- Automated accessibility results are treated as partial evidence and supplemented by manual keyboard, zoom, contrast, screen-reader, and inclusive-user evaluation.

## Limitations

This was a source-and-code inspection, not a rendered ergonomic study. It did not measure actual contrast ratios, target bounding boxes, zoom behavior, screen-reader output, canvas keyboard behavior at runtime, or user task performance. Those tests are necessary before treating any observation as a verified defect or any proposed feature as “killer” in practice.

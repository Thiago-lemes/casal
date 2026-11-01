---
version: 1
slug: "frontend-index-html"
primary_target: "frontend/index.html"
related_targets: []
---

# Surface: app shell and Hoje (frontend/)

Mode: Operate. Mobile first, desktop strong. Dark and light themes, pt-BR and en.

## Job and audience
One person of a couple living together, phone in hand, wanting to know in seconds what the day asks and to leave a note for the other. Success: open the app, read the day, resolve or create a reminder in two taps.

## Direction contract

THESIS: The app speaks like a freezer label: a big date, a short name, a date-stamped state. It refuses the grey app with one accent colour and the SaaS card grid; reminders are labels in a list, not identical cards.

OWN-WORLD: Orange is committed, 30 to 50 percent of the surface on key screens: the brand band, the primary action field, the "agora" marker. Light theme: cool near-white ground #F3F4F6, ink #1B1D22, orange #F26A1B (text on orange is always ink, never white). Dark theme: graphite #15171B, ink #F1F2F4, orange #FF8A3D. Two person markers (blue #2459D9, green #0E8A6B in light; lifted tints in dark) used only where data needs to tell the two people apart. Type: Barlow Condensed (bold, large dates and titles) with Barlow for text; times and dates in tabular numerals. Shapes: labels have a die-cut notch on the leading edge, not a coloured side border; one corner radius family; flat surfaces, divider only where content separates.

STORY: The visitor sees today's date and the next thing due, understands who left what for whom, resolves it with one tap or snoozes it, and leaves with the sense that the day is under control.

FIRST VIEWPORT: Phone: orange band with the full date set large (weekday and day, condensed) and the greeting of the person in one short line; below it the list of pending reminders as labels, the soonest first, each with its due time large at the leading edge, the text, who left it ("Ana deixou"), and two actions (Resolver, Adiar). A bottom bar with four destinations (Hoje, Rotina, Casa, Mais; Rotina disabled with a clear reason until it ships) and a floating primary action "Novo lembrete" in orange. Desktop: left rail replaces the bottom bar; the list gets a readable column (max 72ch) and a side panel with the composer.

FORM: Etiqueta de marmita (the freezer label), position 1 of my grounded list, taken as the Impeccable's pick card. seed: ceb9b49f0dbad3e1

SIGNATURE MOVE: "Dar baixa": resolving a reminder stamps it with the resolve time ("Resolvido 14:32"), the label slides out and the list closes the gap in 200 to 250 ms with an exponential ease-out; reduced-motion users get an instant swap with the stamp text. No other authored motion beyond standard 150 to 250 ms state transitions.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Constraints and open decisions
- App name is undecided; one config constant, never hard-coded in components.
- No eyebrow labels, no section numbers, no gradient text, no cards in cards, no side-borders above 1px, no all-caps tracked labels.
- Contrast AA in both themes; targets 44px; focus ring in orange with offset; reduced motion honored.
- Honest risk: heavy orange can tire on dense screens (finanças, rotina); there the field shrinks to the header band and the action, and lists go neutral.

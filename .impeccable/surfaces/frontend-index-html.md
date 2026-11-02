---
version: 1
slug: "frontend-index-html"
primary_target: "frontend/index.html"
related_targets: []
---

# Surface: app shell and Hoje (frontend/)

Mode: Operate. Mobile first, desktop strong. Dark and light themes, pt-BR and en.

## Job and audience
One person of a couple living together, wanting to know in seconds what the day asks and to leave a note for the other. Success: open the app, read the day, resolve or create a reminder in two taps.

## User-pinned reference (the brief wins)
The user supplied three dashboard references (a dark fintech bento, a hospital dashboard in light and dark, a utility dashboard): a saturated brand-colour sidebar with a rounded inner corner, a page ground slightly different from the cards, large rounded cards in a bento grid, a month calendar widget, a progress ring, a bar chart with the current bar highlighted, pill buttons, circular avatars. Reproduce that structure in orange, in both themes. All numbers shown come from real reminders; no invented data.

## Direction contract

THESIS: A bento dashboard of the couple's day in orange: one committed orange block (the day), surrounded by calm cards that each answer one question (what is due, how far along is the day, which days are busy, how the week went). It refuses the grey single-column list.

OWN-WORLD: Orange is committed in two places: the navigation rail (desktop) and the day block. Light theme: page ground #E9EBEF, cards #FFFFFF, ink #1B1D22, orange #F26A1B. Dark theme: page ground #101216, cards #1B1E24, ink #F1F2F4, orange #FF8A3D. Text on orange is always dark ink. Type: Barlow Condensed bold for dates, times and big figures, Barlow for text, tabular numerals. Radius 24px on cards, 12px on controls, pills for chips and buttons. Flat cards, no shadow, no nested cards. Reminder rows keep the freezer-label notch.

STORY: The visitor opens Hoje, reads today's date and counts in the orange block, sees the reminders due as labels, the day's progress ring, the month with busy days dotted, and the week's resolved bars, and resolves or creates a reminder without leaving the page.

FIRST VIEWPORT: Desktop: orange rail on the left (brand, Hoje, Rotina soon, Casa, Mais) with a rounded inner corner against the page ground; a 12-column bento: row one the orange day block (weekday and date large, greeting, the three counts pending, overdue, done today, and the New reminder button) beside the day progress ring; below, the reminders list on the left column and the month calendar over the week bars on the right. Phone: bottom bar; stack order day block, ring, reminders, calendar, week bars.

FORM: Bento dashboard from the user's references, over the Etiqueta de marmita palette. seed: ceb9b49f0dbad3e1

SIGNATURE MOVE: "Dar baixa": resolving a reminder stamps it with the resolve time, the label closes, and the ring and bar for today animate to their new value in 200 to 250 ms. No other authored motion.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Constraints
- App name in one config constant.
- No eyebrows, no section numbers, no gradient text, no side borders above 1px, no nested cards.
- AA contrast both themes, 44px targets, orange focus ring, reduced motion honored.
- Charts are SVG/CSS drawn from real data, with an accessible text alternative.

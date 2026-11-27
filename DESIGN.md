---
name: adf console
description: A control room for a fleet of agent desktops
colors:
  graphite-rack: "oklch(0.215 0.007 70)"
  graphite-panel: "oklch(0.252 0.008 70)"
  graphite-raised: "oklch(0.305 0.009 70)"
  graphite-line: "oklch(0.355 0.01 70)"
  graphite-line-strong: "oklch(0.47 0.012 70)"
  paper-text: "oklch(0.945 0.006 80)"
  ash-muted: "oklch(0.75 0.012 75)"
  ash-faint: "oklch(0.6 0.01 75)"
  screen-black: "oklch(0.13 0.004 70)"
  tally-amber: "oklch(0.8 0.15 75)"
  tally-red: "oklch(0.66 0.2 25)"
  tally-green: "oklch(0.74 0.15 150)"
typography:
  headline:
    fontFamily: "Barlow Semi Condensed, Barlow, system-ui, sans-serif"
    fontSize: "26px"
    fontWeight: 600
    lineHeight: 1.2
  title:
    fontFamily: "Barlow Semi Condensed, Barlow, system-ui, sans-serif"
    fontSize: "17px"
    fontWeight: 600
    lineHeight: 1.2
  body:
    fontFamily: "Barlow, system-ui, sans-serif"
    fontSize: "15px"
    fontWeight: 400
    lineHeight: 1.45
  label:
    fontFamily: "Barlow, system-ui, sans-serif"
    fontSize: "13.5px"
    fontWeight: 500
    lineHeight: 1.2
    fontFeature: "tnum"
rounded:
  s: "4px"
  m: "6px"
spacing:
  xs: "6px"
  s: "10px"
  m: "16px"
  l: "24px"
components:
  button:
    backgroundColor: "{colors.graphite-raised}"
    textColor: "{colors.paper-text}"
    rounded: "{rounded.s}"
    padding: "7px 13px"
  button-primary:
    backgroundColor: "{colors.paper-text}"
    textColor: "{colors.graphite-rack}"
    rounded: "{rounded.s}"
    padding: "7px 13px"
  button-pressed:
    backgroundColor: "{colors.tally-red}"
    textColor: "{colors.paper-text}"
  button-danger:
    backgroundColor: "{colors.graphite-rack}"
    textColor: "{colors.ash-muted}"
    rounded: "{rounded.s}"
  input:
    backgroundColor: "{colors.graphite-rack}"
    textColor: "{colors.paper-text}"
    rounded: "{rounded.s}"
    padding: "7px 10px"
  segmented:
    backgroundColor: "{colors.graphite-panel}"
    textColor: "{colors.ash-muted}"
    rounded: "{rounded.m}"
---

# Design System: adf console

## 1. Overview

**Creative North Star: "The Broadcast Multiviewer"**

The console borrows from the gallery of a TV control room, where one wall shows every source at once, each with a label strip and a tally light. Here every source is an agent's desktop. The live screens are the brightest thing on the page; a warm graphite surround recedes so a wall of white web pages reads at a glance, and nothing else competes with them until something needs a person. Dark is a consequence of the content, not a default: the screens carry the light.

It is a product surface, so familiarity wins over novelty. One button vocabulary, one segmented control, one lamp. Colour is almost absent and carries meaning only: amber means an agent is acting, red means a person has control or something failed, green means ready. As PRODUCT.md puts it, attention is earned.

The system rejects generic SaaS admin dashboards with identical metric cards and hero numbers, Grafana-style panel soup, neon-on-black hacker styling, glassmorphism, and modal dialogs for routine actions.

**Key Characteristics:**
- Live screens framed as monitors, each with a label strip below.
- Tally borders and lamps as the only saturated colour.
- Warm graphite neutrals tinted toward hue 70, never pure black or white.
- Barlow for text, Barlow Semi Condensed for names and headings, tabular numbers for metrics.
- Inline, reversible actions; no dialogs.

## 2. Colors

A restrained palette: tinted graphite neutrals and three tally colours that each mean exactly one thing.

### Primary
- **Tally Amber** (oklch(0.8 0.15 75)): an agent acted on this desktop in the last few seconds. Monitor border with a soft 6px halo, lamp, state text, and the room meter when space runs low.

### Secondary
- **Tally Red** (oklch(0.66 0.2 25)): a person has control (border, pressed button), a recent action failed (lamp, state text, last-action text), the computer is full, or a destructive action is armed.
- **Tally Green** (oklch(0.74 0.15 150)): ready lamp only.

### Neutral
- **Graphite Rack** (oklch(0.215 0.007 70)): page background and input fields.
- **Graphite Panel** (oklch(0.252 0.008 70)): header, toolbar controls, activity feed, control bar.
- **Graphite Raised** (oklch(0.305 0.009 70)): buttons and the selected segment.
- **Graphite Line** (oklch(0.355 0.01 70)) and **Line Strong** (oklch(0.47 0.012 70)): dividers, monitor edges, hover edges.
- **Paper Text** (oklch(0.945 0.006 80)): primary text and the primary button fill.
- **Ash Muted** (oklch(0.75 0.012 75)) and **Ash Faint** (oklch(0.6 0.01 75)): secondary text, metadata, times.
- **Screen Black** (oklch(0.13 0.004 70)): the inside of a monitor before its first frame, and the scrim behind overlays.

**The Tally Rule.** Saturated colour appears only as a tally: on a monitor border, a lamp, or the text that names the same state. Never as decoration, never on an inactive control.

**The Lamp Plus Words Rule.** A lamp is always paired with text that says the same thing. Colour alone never carries state.

## 3. Typography

**Display Font:** Barlow Semi Condensed (with Barlow, system-ui)
**Body Font:** Barlow (with system-ui)
**Label/Mono Font:** ui-monospace only for literal commands and code

**Character:** Barlow is a slightly rounded grotesque with the plainness of signage; its semi-condensed cut gives desktop names the feel of a broadcast label strip without becoming a display face.

### Hierarchy
- **Headline** (600, 26px, 1.2): the desktop name in the focused view; 30px for the empty-state title.
- **Title** (600, 17px, 1.2): desktop names on tiles, panel headings at 18px.
- **Body** (400, 15px, 1.45): explanations and overlays; empty-state copy capped at 64ch.
- **Label** (500, 13.5 to 14px, tabular numbers): metrics, counts, times, button text.

**The Tabular Rule.** Every number that updates live uses tabular figures so it never jitters.

## 4. Elevation

Flat by default, separated by tone and 1px lines. The only lifted elements are toasts (a soft shadow, `0 10px 30px` at half opacity of the rack colour), because they float over content. Monitors gain a 2px tally border plus a 6px translucent halo when an agent acts or a person takes control; that is state, not elevation.

**The State Is The Shadow Rule.** If something glows, it is because its state changed. Nothing glows at rest.

## 5. Components

### Buttons
- **Shape:** gently squared corners (4px).
- **Default:** Graphite Raised fill, Paper Text, 500 weight, 7px by 13px padding; hover lightens one step, active darkens.
- **Primary:** Paper Text fill with Graphite Rack text. Used once per view: New desktop, or the Resume and Start buttons inside an overlay.
- **Pressed toggle:** Take control turns Tally Red and reads Release control.
- **Danger:** transparent with a line border and muted text; hover turns the border and text red; the first click arms it (red fill, "Confirm delete") for 4 seconds instead of opening a dialog.
- **Small:** 13px text, 4px by 10px padding, for quick keys and tile actions.

### Segmented controls
- **Style:** a Graphite Panel track with a 1px line and 6px radius; segments are transparent with muted text; the selected segment is Graphite Raised with Paper Text. Counts sit after the label in Ash Faint and disappear when zero.

### Monitors (signature component)
- 16:10, Screen Black inside, 4px radius (6px in the focused view), a 1px line edge that strengthens on hover.
- Tally border: 2px amber or red plus a 6px soft halo.
- Paused and stopped screens turn grayscale and dim behind a scrim overlay that names the state and what happens next.
- Tile actions (Pause or Resume, Delete) appear on the screen's top-right corner on hover or focus, and stay visible for paused and stopped desktops.

### Label strip
- Lamp, name, state text, then metrics right-aligned (CPU and RAM, value in Paper Text, label muted, amber when hot). Below it, the latest action: who, what, and when, with the time right-aligned.

### Inputs
- Graphite Rack fill, 1px line, 4px radius; hover strengthens the line; focus turns it Paper Text; an invalid name turns it red.

### Room meter
- An 8px bar of memory in use with a hatched segment at the right for the memory kept free for the computer. It turns amber with room for two or fewer desktops and red when full, with the words beside it saying the same.

## 6. Do's and Don'ts

### Do:
- **Do** keep the live screens the brightest element; chrome stays in the graphite range.
- **Do** pair every lamp with words, and every colour change with a state change.
- **Do** offer the next action where the state is shown: Resume on a paused screen, Start on a stopped one.
- **Do** stop pulling frames for hidden tabs, off-screen tiles, and paused or stopped desktops, and slow thumbnails as more are visible.
- **Do** write in the user's terms: desktops, agents, memory. Never machines, branches or balloons.

### Don't:
- **Don't** build generic SaaS admin dashboards: identical metric cards, hero numbers, gradient accents.
- **Don't** build Grafana-style panel soup: dozens of charts competing for attention.
- **Don't** use neon-on-black "hacker" or cyberpunk styling.
- **Don't** use glassmorphism, decorative blur or glow.
- **Don't** open modal dialogs for routine actions such as confirming a delete.
- **Don't** use pure black or pure white, or neutrals without the hue 70 tint.
- **Don't** use a side-stripe border to mark state; use the full tally border.
- **Don't** join metadata with middle dots or use all-caps labels.

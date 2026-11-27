---
name: desktops
description: Use when a task needs a real browser or computer rather than a plain fetch or API, such as sites that need clicking, forms, logins or JavaScript (flight and hotel search, web apps), comparing many websites, testing a web or Linux desktop app, or using a GUI app; and whenever the user mentions adf, desktops or computer use. Covers sizing the work to this computer's limits, running one subagent per desktop in parallel, handing control to a human, and cleaning up.
---

# Running adf desktops

adf gives you sandboxed Linux desktops (1280x800, Firefox, Debian) through its MCP tools. Each desktop is an isolated microVM that starts in about a second, so use as many as the task needs and the computer allows. The user watches them all live in the adf console.

## 1. Plan the split

Break the task into independent pieces that can run at the same time, one desktop per piece. Examples:
- comparing 3 destinations on flights, hotels and things to do: 9 desktops (destination x aspect)
- testing a web app: one desktop per test account, role or feature area
- researching 6 products or vendors: one desktop each

Use one desktop for a single linear task. Don't create desktops you will not use. Give each a short, descriptive lowercase name (`seattle-flights`, `qa-checkout`) that is not already taken (`list_desktops`).

## 2. Check the room

Call `fleet_status` before creating several desktops. It reports how many more fit right now ("room for N more") and the limits. Plan for no more than that number. If the task has more pieces than room, run them in waves: start what fits, and when one finishes, delete its desktop and start the next. Don't try to raise the limits yourself.

## 3. Run them in parallel

Start one subagent per desktop, all in the same message, so they run at the same time. A fast model such as sonnet is usually enough for driving a desktop. Give each subagent a complete brief, since it sees nothing else:
- its desktop name, and to create it with `create_desktop` if it does not exist
- to use only that desktop, and never create, delete or touch other desktops
- the goal and exactly which facts to bring back, in a few short lines
- a firm budget of about 25 actions: when it runs out, stop and report what it has, marking anything unconfirmed
- the currency, language or region the answer should use, if it matters
- not to log in, buy, send, post or delete anything unless the user asked for it, and to stop and report if it hits a login, captcha or payment
- to leave its desktop running when it finishes (you clean up)

Driving tips worth passing on:
- every action returns a screenshot, so there is no need for a separate one; use `screenshot` with `wait_seconds` when a page is still loading
- `ctrl+l` then typing a URL is faster than clicking around; zooming out with `ctrl+minus` shows more of a long page
- websites see this computer's location, so they may show another currency or language: set the right one first (for Google Flights and Hotels, add `&curr=USD&hl=en&gl=us` or the right codes to the URL) instead of converting later
- `run_command` runs shell commands in the desktop: it can install Linux apps with `apt-get install -y`, then start them with `DISPLAY=:1 app &`

## 4. Hand over to a human when needed

For a login, captcha, two-factor code, payment or anything irreversible, don't guess. Tell the user which desktop needs them, and to open the console (`adf console`, at http://127.0.0.1:7300 by default), click that desktop and press Take control. Then wait for them by calling `screenshot` with `wait_seconds` 20 again and again (for up to about 5 minutes) until the screen shows they are done.

## 5. Report

When the subagents are back, combine their findings into one answer, usually a table plus a recommendation. Say which pieces failed or were blocked and why, instead of filling gaps with guesses.

## 6. Clean up

When the task is done, delete every desktop you created with `delete_desktop`, including any whose subagent failed, then call `fleet_status` to confirm the room is back. Keep them only if the user asked to, or if a human still has to finish something on one (say which). Never delete desktops you did not create unless the user asks.

## When something fails

- "not enough room on this computer": finish or delete one of your desktops first, or run fewer at once
- "no running desktop named ...": create it with `create_desktop` (or check the name with `list_desktops`)
- the very first `create_desktop` on a computer downloads and prepares the desktop image, which takes a minute or two; later ones take about a second

# Product

## Register

product

## Users

Developers who run AI agents that use computers: people building agent products, running evals or RL experiments, or testing their own agents. They start desktops from an agent (Claude Code, Cursor, their own loop over MCP) or from the `adf` CLI, and open the console beside their editor and terminal, often on a second monitor. Most of the time they watch: is every agent making progress, which one is stuck, is the machine running out of room. Occasionally they take over a desktop to unblock an agent or check something by hand.

## Product Purpose

adf runs many lightweight sandboxed Linux desktops on one computer, makes them usable by any agent, and lets a person see every one of them live. The console is the visibility layer: a wall of live screens, what each agent just did, how much room the computer has left, and a way to step in. Success is that a person can glance at twenty desktops and know within seconds which need attention, then fix it in a click or two.

## Brand Personality

Calm, precise, alive. The console is a control room, not a dashboard: the live screens are the content, and everything else recedes until something needs attention. It speaks plainly, in the user's terms (desktops, agents, memory), never in internal terms (machines, branches, balloons).

## Anti-references

- Generic SaaS admin dashboards: identical metric cards, hero numbers, gradient accents.
- Grafana-style panel soup: dozens of charts competing for attention.
- Neon-on-black "hacker" or cyberpunk styling.
- Glassmorphism, decorative blur and glow.
- Modal dialogs for routine actions such as confirming a delete.

## Design Principles

1. The screens are the content. Chrome stays quiet so a wall of bright web pages reads at a glance.
2. Attention is earned. Color and motion appear only when something changed or needs a person: an agent acting, a failure, the computer running out of room.
3. Show the state, then the action. Every desktop shows whether it is running, paused or stopped and offers the next sensible action right there.
4. Lightweight is a feature. The console never costs more than it shows: no polling of hidden tabs or off-screen tiles, slower thumbnails when many are visible.
5. Teach in place. Empty and error states tell the user exactly what to do next, including how to connect an agent.

## Accessibility & Inclusion

WCAG 2.2 AA contrast for text and controls. Full keyboard use with visible focus. State is never conveyed by color alone: lamps always pair with text. Respect prefers-reduced-motion. Takeover makes clear where keystrokes are going and how to stop.

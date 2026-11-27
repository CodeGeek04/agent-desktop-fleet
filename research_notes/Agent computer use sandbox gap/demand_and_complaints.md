# Demand Side and Developer Pain Points: Sandboxed Computers for Computer-Use Agents (as of 2026-11-27)

Notes on method: HN was searched through the Algolia API (stories and comments from 2025-01-01 on) and the key threads were read in full. GitHub issues and star counts were pulled with the GitHub API on 2026-11-27. Reddit could not be reached: the search tool returned "domains are not accessible to our user agent" for reddit.com. X/Twitter post bodies could not be read either, only titles surfaced by search. Em dashes inside original quotes have been replaced with commas to match the house style, and en dashes in HN titles with hyphens. Items marked "(secondhand)" come from a search-engine summary or an aggregator and have not been checked against the primary source.

## 1. What builders are saying about the lack of good platforms for running many sandboxed computers (and where the "no good platforms" claim comes from)

### Takeaway
I could not find the exact sentence "there are no good platforms for running a lot of sandboxed computers for agents" anywhere. The same sentiment does show up again and again in 2026 Hacker News threads, mostly from people who went on to build their own platform. Their reasons are cost markups, session caps, Linux-only offerings, the macOS licensing ceiling, and the lack of any accepted standard. A crowded field of YC launches (several per batch) and skeptics calling sandboxes "commoditized" sit alongside these complaints. That suggests the gap is in fit (desktops, Windows/macOS, scale economics, RL features), not in the number of vendors.

### Cited Findings
- 2026-03-05, Coasty co-founder Nitish (HN): "My co-founder Prateek and I built this after running into the same wall most agent developers hit: you need a real, isolated computer for your agent to actually do things, browse, run code, fill forms, submit jobs, and the existing options are either expensive, capped at 24-hour sessions, or both." He adds that Coasty's own VMs "spin up in 2-5 minutes" with "no session limits." This is the closest match to the target claim that I found. [HN comment 47268583](https://news.ycombinator.com/item?id=47268583)
- 2026-07-14, solo builder AnicetN (HN): "I started shopping for sandboxes and I realized like web hosting primitives in the web era, they were adding like a 50x markup on what qemu/docker + bare metal prices would be if I was doing it myself, even if I wanted full, beefy ubuntu VMs with all access. Also generally their devX wasn't great, stuck between the old world of short-lived code execution for agents and now long-running agents. I struggled with fly a lot, struggled with e2b/daytona too, the daytona team even offered me 10k in credits ... but I knew long term I'd save hundreds of thousands rolling my own." He then built and released his own sandbox ("box"), saying it sees "hundreds of days of VM time consumed by our users per day." [HN comment 48907563](https://news.ycombinator.com/item?id=48907563)
- 2026-06-26, colesantiago (HN): "Most of the startups are just wrappers around AWS and significantly more expensive. Agents need sandboxes that are cheaper so that they can run thousands ... it would be nice to self host. The best part of self hosting is that you own it as well, no rug pulls from the laundry list of reselling providers that could go away at anytime. It would be nice to have a one click sandbox agent on a self hosted instance that is, free, fast ... and that is open source." [HN comment 48688995](https://news.ycombinator.com/item?id=48688995)
- 2026-04-07, etse, in the Freestyle Launch HN thread: "One gap I'm noticing in these comments and in the current sandbox landscape is Windows. Every platform mentioned in these comments like E2B, Daytona, Fly Sprites, Sandflare appears Linux-native ... a real category exists to automate Windows-specific workflows: enterprise software, ERP systems, anything that runs only on Windows." The commenter also promotes his own Windows tool, Dexbox/Nen, so the statement is self-interested. [HN comment 47679269](https://news.ycombinator.com/item?id=47679269)
- 2026-11-17, Cua founder Francesco Bonacci (HN, launching the Hoy VMM): "In order to meet our customer demands for evals, and RL at scale, we decided to build our own vmm on Hypervisor.framework." He reports boot "5.3 s vs 6.6 s, restore 1.2 s vs 3.0 s with Apple's Virtualization framework" and notes limits: "two macOS VMs per Mac, enforced to meet apple's macOS license." Hoy is "closed-source to start with." [HN comment 50534225](https://news.ycombinator.com/item?id=50534225); [Hoy launch page](https://hoy.cua.ai/launch)
- 2026-01-20, "Ask HN: Why are so many rolling out their own AI/LLM agent sandboxing solution?" (32 pts). Replies included: "It simply feels as if there is no de facto standard yet (there surely will be)" (kaffekaka, 2026-01-22), and "Same reason everyone rolled their own auth in 2010, the problem is simple enough to DIY badly, complex enough that no standard fits everyone" (varshith17, 2026-01-22). [HN thread 46699324](https://news.ycombinator.com/item?id=46699324)
- 2026-09-23, dbmikus (HN, "Cloud Agents Are Inevitable AI Prisons" thread, 74 pts): "It's still not user friendly to do all this ... There's a lot of glue and parts to stitch together to work smoothly. Don't think we've had the 'Docker moment' for this." [HN comment 49822295](https://news.ycombinator.com/item?id=49822295)
- 2026-02-12, toliveistobuild, commenting on an 8-provider remote-browser benchmark: "40% to 95% success rate across providers doing essentially the same thing (serve a browser, let an agent drive it). That's a massive gap for infrastructure that's supposed to be commoditized ... 250 concurrent sessions and most providers weren't even tested because they couldn't handle it ... What's missing from this benchmark is the thing that actually kills you in production: anti-bot detection ... none of these benchmarks test authenticated sessions." [HN comment 46984155](https://news.ycombinator.com/item?id=46984155)
- 2025-04-23, Launch HN: Cua (YC X25), 172 pts / 73 comments. Users asked for things Cua lacked at launch:
  - "Would love to use this for TestDriver, but needs to support Windows" (tomatohs)
  - "will this also be available as a hosted service? Or do you have instructions on how to manage a fleet of these manually" (brene)
  - "One-shot VM would be nice. ephemeral VM spins up, agent runs task, VM is deleted, perfect for CI pipelines" (dhruv3006)
  - The founder replied that Kubernetes-style orchestration "didn't make much sense for macOS since you can only spin up two macOS VMs at a time due to Apple's licensing restrictions."
  - Caveat: another commenter accused the launch of astroturfing with fresh accounts, so some of the praise in the thread is low-signal.
  [HN thread 43773563](https://news.ycombinator.com/item?id=43773563)
- 2026-10-12, Launch HN: Agent37 (YC F26), "Low-cost agent sandboxes," 163 pts / 136 comments. The thread shows both demand and commoditization:
  - "Got a bunch of Daytona credits which are close to useless given their restrictions until you hit tier 3" (alasano)
  - "Sandboxes look a bit commoditized. You cannot 10x on basically what's just a VM with a few agent-friendly features in 2026" (MikhailTal)
  - "feels like there could be a sandbox router (like openrouter) company soon" (xinweihe)
  - "I had ran into another yc startup from the same batch doing the same thing recently ... YC batch are becoming more like hunger games" (bitlad)
  [HN thread 50059096](https://news.ycombinator.com/item?id=50059096)
- 2026-07-15, Launch HN: Coasty (YC S26), "An API for computer-use agents." Commenters asked "How are you all different than the other few CUA APIs in this batch and previous batches?" and said "It's so funny to see YC back tens of generic similar low quality projects." [HN thread 48922706](https://news.ycombinator.com/item?id=48922706)
- 2026-04-27, Tensorlake engineering blog, "Building Sandboxes for Computer Use": "The hard part is building the boring, reproducible desktop around it," and "A previous session can influence what is visible on the screen, which changes what the model sees." This is a vendor post. [Tensorlake blog](https://www.tensorlake.ai/blog/building-sandboxes-for-computer-use)
- 2026-06-24, HN thread "Computer use in Gemini 3.5 Flash" (242 pts / 167 comments):
  - Skeptic: "Computer use is such a terrible idea. It's slow, insecure, error prone, expensive." (airstrike)
  - Practitioner: "Computer use is very useful for developing GUI applications since claude code can build and test the entire app end-to-end ... I run it in a VM using a headless wayland compositor, I'd never trust even fable with access to my real system." (antiloper)
  - Another commenter: "I want a lightweight sandbox with all my dev stuff that LLMs can't escape before I turn on computer use."
  [HN thread 48662999](https://news.ycombinator.com/item?id=48662999)
- 2026-07-10, HN comment relaying a SemiAnalysis argument (secondhand): "the SemiAnalysis guy argues that there is a market for macOS servers - running macOS VMs that agents need for computer use. some software is mac only but Apple needs to change the licensing model, currently you are allowed to run only 2 macOS VMs for every physical one you buy." [HN comment 48858156](https://news.ycombinator.com/item?id=48858156)
- X/Twitter posts surfaced by search but not readable (titles only): Harrison Chase, "The two patterns by which agents connect sandboxes" [x.com/hwchase17](https://x.com/hwchase17/status/2021261552222158955); Cua, "Computer-Use 2.0 at AI Engineer World's Fair" [x.com/trycua article](https://x.com/trycua/article/2077440069460181426?lang=en); Fabrizio Serafini, "Can Agents Use a Computer Yet? We've Got the Data" [x.com/fabserafini](https://x.com/fabserafini/status/2086831520824934589).

### Inferences
- The "no good platform" view is held most strongly by people whose workloads fall outside what the coding-sandbox vendors are built for: full desktops (not shells), Windows or macOS, long-running sessions, and hundreds to thousands of parallel instances. Several of these people then became vendors themselves (Coasty, box, Dexbox, Agent37, Cua Hoy), which makes it a classic "build the tool you wanted" market.
- Over the same period the general Linux sandbox layer is commoditizing (crowded YC batches, price-war launches, "sandbox router" talk). The open space for a new entrant is probably computer-use-specific: desktop fidelity, non-Linux OSes, RL-grade reset/fork, and fleet operations. It is probably not another Linux microVM API.

### Gaps
- I found no original source for the exact claim "there are no good platforms for running a lot of sandboxed computers for agents." It may be an X post, a podcast line, or an internal paraphrase. X post bodies could not be read and Reddit was blocked.
- Reddit (r/LocalLLaMA, r/AI_Agents, r/selfhosted, r/MachineLearning) could not be searched. The only Reddit evidence is indirect: an HN commenter (2025-05-04) said someone "spent all day spamming r/LocalLLaMA and other LLM-related subs" promoting Cua ([HN thread 43773563](https://news.ycombinator.com/item?id=43773563)). That shows those subreddits are where CUA tooling gets pushed, but tells us nothing about what users there complain about.

## 2. Common complaints, mapped to evidence

### Takeaway
The complaints with the most evidence are:
1. Price markups at scale.
2. Concurrency and session caps on hosted sandboxes (E2B: 10 or 100 concurrent sandboxes and 1h or 24h sessions by default).
3. No Windows support, and macOS held to 2 VMs per Mac by Apple's license.
4. Flaky results (browser providers range from 40% to 95% task success).
5. Anti-bot and identity walls for browser agents.
6. The cost of starting a fresh environment per rollout, with no cheap snapshot or fork, for RL.

Observability (replay, screenshots, logs) and data residency come up mainly from regulated buyers. GPU support and MCP integration are newer and less often mentioned.

### Cited Findings
**Cost at scale**
- "50x markup" over qemu/docker on bare metal. [HN 48907563](https://news.ycombinator.com/item?id=48907563) (2026-07-14)
- "wrappers around AWS and significantly more expensive. Agents need sandboxes that are cheaper so that they can run thousands." [HN 48688995](https://news.ycombinator.com/item?id=48688995) (2026-06-26)
- Agent37 pitches "under 10% of the cost of most sandbox providers" and an "average of $0.37/month per sandbox." Its founder says "AWS has 80-90% margins, so you gotta rent the bare metal directly." These are vendor claims. [HN 50059096](https://news.ycombinator.com/item?id=50059096) (2026-10-12)
- E2B published pricing, read 2026-11-27: $0.000014 per vCPU-second (about $0.05 per vCPU-hour by my calculation) and $0.0000045 per GiB-second. Sandboxes are 1 to 8 vCPU and 1 to 8 GiB, CPU-only. [E2B pricing](https://e2b.dev/pricing)
- OSGym (arXiv, v1 2025-11-11, v5 2026-04-02) says full GUI sandboxes "consume substantial hardware resources as the number of sandboxes scales." It claims to cut per-replica cost to "0.2-0.3 USD per day," a 90% reduction from standard deployment, while managing "over a thousand OS replicas." [arXiv 2511.11672](https://arxiv.org/abs/2511.11672)
- Token cost on top of infra cost: The Register (2026-05-06) reports Reflex research finding that a browser-use vision agent needed "half a million tokens" to click a dropdown. [The Register](https://www.theregister.com/2026/05/06/aws_workspaces_agent_access/)
- Idle hardware cost: a16z's Martin Casado on the Phaze seed (2026-11-04): "High-end compute is too expensive to sit idle on individual desks." [Utah Business press release](https://www.utahbusiness.com/press-releases/2026/11/04/phaze-raises-seed-led-a16z-give-ai-agents-access-computers-indie-nextview/)

**Cold start and boot latency**
- Coasty's own VMs "spin up in 2-5 minutes" (2026-03-05). That is slow next to what coding-sandbox vendors advertise, and suggests full desktops are still slow to bring up. [HN 47268583](https://news.ycombinator.com/item?id=47268583)
- Daytona claims "60 milliseconds" to configure a sandbox, against 30 seconds to many minutes for traditional cloud VMs (Fortune, 2026-10-20). [Fortune](https://fortune.com/2026/10/20/daytona-startup-building-the-agentic-cloud-valued-at-1-billion-in-latest-funding-round/)
- macOS VMs: boot 5.3 s and restore 1.2 s with Cua's Hoy, against 6.6 s and 3.0 s with Apple's Virtualization framework (2026-11-17). [HN 50534225](https://news.ycombinator.com/item?id=50534225)
- Agent37 user: "cold start time was ever so slightly annoying but nevertheless, cost is attractive." Another user: "the setup and teardown cycle that eats the time." [HN 50059096](https://news.ycombinator.com/item?id=50059096) (2026-10-13)
- (Secondhand) A 2026 cloud-computer comparison reports provisioning in "one to three minutes, not milliseconds" and stopped VMs still billed at full rate. [Pinggy blog](https://pinggy.io/blog/best_cloud_computers_for_ai_agents/)
- (Secondhand) The cua-speedrun benchmark (arXiv 2609.40284) reports roughly 2-3 seconds from action to observation in standard CUA runtimes. [arXiv 2609.40284](https://arxiv.org/pdf/2609.40284)

**Concurrency caps and session duration limits**
- E2B tiers, read 2026-11-27:
  - Hobby: 10 concurrent sandboxes, 1-hour sessions.
  - Pro ($150/mo plus usage): 100 concurrent sandboxes (600 or 1,100 with paid add-ons), 24-hour sessions.
  - Enterprise: from $3,000/month, with BYOC available on request.
  [E2B pricing](https://e2b.dev/pricing)
- Coasty: existing options are "capped at 24-hour sessions." [HN 47268583](https://news.ycombinator.com/item?id=47268583)
- Daytona credits are "close to useless given their restrictions until you hit tier 3" (2026-10-12). [HN 50059096](https://news.ycombinator.com/item?id=50059096)
- OSWorld on AWS: a researcher hit `VcpuLimitExceeded` running only 5 parallel envs on a t3.large (2025-10-27). The maintainer advised a bigger host "If you want to use more than 15 VM environments in parallel." [OSWorld issue 366](https://github.com/xlang-ai/OSWorld/issues/366)
- OSWorld maintainer on how to get the eval under an hour: "set the parallelel [sic] to ~300" (2025-11-13). [OSWorld issue 374](https://github.com/xlang-ai/OSWorld/issues/374)
- E2B SDK bug at high concurrency: "Python SDK HTTP 2 transport fails under high async sandbox concurrency" (2026-06-10). [E2B issue 1418](https://github.com/e2b-dev/E2B/issues/1418)

**Lack of Windows / macOS desktops**
- E2B maintainer, asked whether E2B Desktop supports Windows (2025-06-26): "Hey bro, no way, because we roll em' Firecrackers, and they don't mess with Windows." [e2b-dev/desktop issue 112](https://github.com/e2b-dev/desktop/issues/112)
- "Every platform mentioned ... E2B, Daytona, Fly Sprites, Sandflare appears Linux-native" (2026-04-07). [HN 47679269](https://news.ycombinator.com/item?id=47679269)
- Apple's licensing caps macOS at 2 VMs per physical Mac. Cua cited this in 2025-04 ([HN 43773563](https://news.ycombinator.com/item?id=43773563)) and still enforces it in Hoy in 2026-11 ([HN 50534225](https://news.ycombinator.com/item?id=50534225)).
- Google said Gemini 2.5 Computer Use (2025-10-07) "is not yet optimized for desktop OS-level control," as quoted in the HN thread. A commenter asked: "Does it work with ~legacy~ software? Eg, early 2000's Windows ... There's a goldmine to be had in automating ancient workflows that keep large corps alive." [HN 45507936](https://news.ycombinator.com/item?id=45507936)
- AWS spokesperson, on letting agents drive WorkSpaces virtual desktops: enterprise software "simply doesn't have API access" (2026-05-06). [The Register](https://www.theregister.com/2026/05/06/aws_workspaces_agent_access/)
- Cua's Windows support still has rough edges: "Help wanted: validate multi-monitor (negative-X origin) fix on real Windows hardware, we have no local rig" (2026-06-23, [cua issue 1981](https://github.com/trycua/cua/issues/1981)), and "Windows foreground type_text corrupts multiline text in Notepad" (2026-10-02, [cua issue 4477](https://github.com/trycua/cua/issues/4477)).

**Flakiness / reliability**
- AIMultiple benchmark of 8 remote-browser providers (updated 2026-01-30). Single-task success rates:

  | Provider | Success |
  |---|---|
  | Bright Data | 95% |
  | BrowserAI | 85% |
  | Steel | 70% |
  | Anchor | 70% |
  | Hyperbrowser | 60% |
  | ZenRows | 55% |
  | Browserbase | 50% |
  | Airtop | 40% |

  Only 3 of the 8 had results in the 250-concurrent-agent load test: BrowserAI 86.4%, Bright Data 81.2%, ZenRows 51.2%. [AIMultiple](https://aimultiple.com/remote-browsers)
- A competitor-authored post, "Browserbase has a 29% failure rate on basic page loads" (2025-08-28), comes from Anchor Browser, so treat it as biased. [HN 45052825](https://news.ycombinator.com/item?id=45052825)
- Cua at launch: a user "tried this three times," and the agent "got ahead of itself and starting typing things in the wrong place," then later runs "failed to do anything" (2025-04-23). [HN 43773563](https://news.ycombinator.com/item?id=43773563)
- Cua's local Linux sandbox was "unusable on Apple Silicon (no arm64 image; amd64 host bridge resets)" (2026-08-19, later fixed). [cua issue 3254](https://github.com/trycua/cua/issues/3254) A separate Cua issue reports "~50% CPU usage when idle" (2026-06-02). [cua issue 1808](https://github.com/trycua/cua/issues/1808)
- OSGym cites "stochastic errors arising from diverse software execution within these sandboxes" and "CPU-bounded scaling bottlenecks." [arXiv 2511.11672](https://arxiv.org/abs/2511.11672)
- Modal's guide (2026), summarizing ComputerRL: "desktop environments are slow and unstable over long runs." [Modal resource](https://modal.com/resources/best-sandboxes-computer-use-agent-rl)

**Observability (live view, replay, per-step screenshots/logs)**
- Coasty Launch HN (2026-07-15):
  - A prospect asked: "Do the screenshots and playback and logs get stored on our systems as well? ... Sure but is that stored on our infra or yours? (Work in HIPAA regulated industry)"
  - Another commenter: "Having approval gates plus a replayable event log makes the agent's decisions much easier to audit."
  - The founder: "'did the task finish?' isn't enough. You need to know what the agent saw, why it acted, what changed, and where a human approved something consequential."
  [HN 48922706](https://news.ycombinator.com/item?id=48922706)
- (Secondhand) A 2026 sandboxing guide says standard observability tools "often fail to penetrate the VM boundary." [manveerc substack](https://manveerc.substack.com/p/ai-agent-sandboxing-guide)

**Self-hosting / data residency / compliance**
- HIPAA concern about pig.dev "storing data on their servers" (2025-04-23, [HN 43773563](https://news.ycombinator.com/item?id=43773563)), repeated for Coasty in 2026-07 ([HN 48922706](https://news.ycombinator.com/item?id=48922706)).
- Anthropic added self-hosted sandboxes to Claude Managed Agents (2026-05-19): "Code execution, sensitive files, packages, services, and data stay within your enterprise perimeter." Supported options are Cloudflare, Daytona, Modal, Vercel, or the customer's own infrastructure. [Claude blog](https://claude.com/blog/claude-managed-agents-updates)
- OpenAI's Agents API also offers self-hosted environments. An HN commenter on 2026-09-10: "Buried in there, note you can opt to self-host your sandbox." [HN 49651647](https://news.ycombinator.com/item?id=49651647)
- Phaze seed press release (2026-11-04): customers "want that work performed on machines they control." [Utah Business](https://www.utahbusiness.com/press-releases/2026/11/04/phaze-raises-seed-led-a16z-give-ai-agents-access-computers-indie-nextview/)

**Vendor lock-in**
- Daytona went closed source on 2026-06-11, citing security only: "There is one reason behind this ... security." The public repo is "not going anywhere" but will no longer be maintained. [Daytona post](https://www.daytona.io/dotfiles/updates/daytona-is-going-closed-source) The GitHub API shows daytonaio/daytona archived with 71,267 stars as of 2026-11-27. [GitHub](https://github.com/daytonaio/daytona) The move drew at least four separate HN submissions in June 2026. [HN 48509317](https://news.ycombinator.com/item?id=48509317)
- Lock-in comments:
  - "no rug pulls from the laundry list of reselling providers." [HN 48688995](https://news.ycombinator.com/item?id=48688995)
  - "I'm looking to recreate this setup as a backup so that I don't get locked in." [HN 50059096](https://news.ycombinator.com/item?id=50059096)
  - "I build a OSS lightweight, portable VM for those that don't want lock ins" (Docker Sandboxes thread, 2026-08-10). [HN 49239751](https://news.ycombinator.com/item?id=49239751)
- Agent37 founder on switching costs: "The switching cost is not trivial unlike LLMs." [HN 50059096](https://news.ycombinator.com/item?id=50059096)

**Fleet management / dashboard**
- "do you have instructions on how to manage a fleet of these manually while you're building the orchestration workflows?" (2025-04-23). [HN 43773563](https://news.ycombinator.com/item?id=43773563)
- Modal's guide says self-managed options leave "capacity planning, image distribution, isolation, monitoring, and recovery at scale" to the team (on SCALE-CUA), and "autoscaling, upgrades, capacity, telemetry, and incident response" (on OpenSandbox). [Modal resource](https://modal.com/resources/best-sandboxes-computer-use-agent-rl)

**MCP integration**
- AWS WorkSpaces for agents uses a managed MCP endpoint for "governed access to desktop tools such as screenshots, mouse control, and text input" (2026-05-06). [The Register](https://www.theregister.com/2026/05/06/aws_workspaces_agent_access/)
- Phaze ships remote-computer control for agents as an MCP server (2026-11-04). [Utah Business](https://www.utahbusiness.com/press-releases/2026/11/04/phaze-raises-seed-led-a16z-give-ai-agents-access-computers-indie-nextview/)
- Cua feature request: "configurable post-action observation and text-field settle delays in MCP" (2026-09-18, 10 comments). [cua issue 3971](https://github.com/trycua/cua/issues/3971)

**GPU / graphics rendering**
- E2B sandboxes are CPU-only per its pricing page (2026-11-27). [E2B pricing](https://e2b.dev/pricing)
- Freestyle founder (2026-04-06): "Modal is the only provider with GPU support." [HN 47663147](https://news.ycombinator.com/item?id=47663147)
- Cua built Hoy partly for "better compatibility with graphic design software requiring gpu" (2026-11-17). [HN 50534225](https://news.ycombinator.com/item?id=50534225)
- Show HN: "Agent Sandboxes with GPUs from $0.20/hr" (2026-09-04). [HN 49563492](https://news.ycombinator.com/item?id=49563492)

**Anti-bot / CAPTCHA / identity (browser agents)**
- "What's missing ... anti-bot detection. A remote browser that loads pages fast but gets Cloudflare-blocked on every third request isn't useful." [HN 46984155](https://news.ycombinator.com/item?id=46984155) (2026-02-12)
- "Rendering was almost never the blocker. Identity was." The gate is "a CAPTCHA, an SMS, or a card on file" (2026-08-07, Cloudflare Kitesurf thread). [HN 49214403](https://news.ycombinator.com/item?id=49214403)
- "I have a browser extension running in my user/personal browser instance that my agent use (with rate-limits) in order to avoid all the captchas and blocks basically" (2025-08-20). [HN 44963613](https://news.ycombinator.com/item?id=44963613)
- Almanac uses Browserbase and hands 2FA logins to a human (2026-08-31). [HN 49511458](https://news.ycombinator.com/item?id=49511458)

**Snapshot / fork for RL and parallel exploration**
- CUA-Sandbox (arXiv, 2026-09-26) says conventional setups "replicate an initialized runtime for each independent rollout," which adds memory and startup costs. Sharing runtimes and branching state gives up to 6.20x rollout throughput, 9.2x lower memory per environment, and 504x lower incremental storage. [arXiv 2609.32750](https://arxiv.org/abs/2609.32750)
- CUA-Gym (2026) runs a pool of about 2,000 OSWorld VMs and resets each to a clean snapshot after every episode (secondhand, via search summary). [arXiv 2605.25624](https://arxiv.org/pdf/2605.25624) A separate paper, TClone (2026), targets "Low-Latency Forking of Live GUI Environments for Computer-Use Agents." [arXiv 2605.17320](https://arxiv.org/pdf/2605.17320)
- Freestyle's copy-on-write memory fork drew this from a commenter: "320ms median seems good for the branch-and-explore pattern without reprovisioning every time" (2026-04-07). The founder said snapshotting otherwise means "a 2-4 second interruption in the VM." [HN 47663147](https://news.ycombinator.com/item?id=47663147)
- Related HN interest: "Tilde.run, Agent sandbox with a transactional, versioned filesystem" (205 pts, 2026-05-06, [HN 48037724](https://news.ycombinator.com/item?id=48037724)); "Snapshots, copy-on-write, and the economics of agent sandboxes" (2026-07-23, [HN 49022605](https://news.ycombinator.com/item?id=49022605)).

### Inferences
- For computer-use agents specifically, the top unmet needs are: (a) non-Linux desktops, (b) cheap massive parallelism with fast reset/fork, and (c) audit-grade replay inside the customer's perimeter. The coding-sandbox market has largely solved fast Linux microVMs, but most of those are Firecracker/gVisor based, which rules out Windows and macOS guests.
- Browser-agent pain is moving from "rendering infrastructure" to "identity and anti-bot." A desktop-VM platform does not fix that, but buyers may expect a residential/stealth story anyway.

### Gaps
- I found few direct user quotes about missing live-view or fleet dashboards. Most observability evidence comes from regulated buyers asking where replays are stored, not from people asking for a fleet UI.
- I found no quantified cold-start figures for full Windows desktop sandboxes from any vendor in 2026.

## 3. User segments and what each needs

### Takeaway
Five segments are visible:
1. RL/eval researchers and labs, who need hundreds to tens of thousands of parallel desktops with cheap reset.
2. Agent startups automating legacy desktop/web software (RPA replacement, QA, back-office, healthcare), who need Windows, reliability, replay, and compliance.
3. Enterprises, who want agents on governed desktops inside their perimeter (AWS WorkSpaces, Windows agent workspace, self-hosted sandboxes).
4. Hobbyists and homelabbers running personal agents in local VMs.
5. Coding agents that need a GUI to test the apps they build.

### Cited Findings
**RL / eval researchers and labs**
- The OSWorld maintainer recommends about 300 parallel envs to finish an evaluation within an hour (2025-11-13). [OSWorld issue 374](https://github.com/xlang-ai/OSWorld/issues/374)
- Qwen-CUA (2026) reports rollout infrastructure on Alibaba Cloud ECS with nearly 100,000 vCPUs supporting "tens of thousands of concurrent computer-use environments" (secondhand, via search summary). [arXiv 2608.02352](https://arxiv.org/pdf/2608.02352)
- ComputerRL (2025) describes infrastructure that orchestrates "thousands of parallel virtual desktop environments" (secondhand). [arXiv 2508.14040](https://arxiv.org/abs/2508.14040)
- OSGym manages "over a thousand OS replicas" and generates 1,420 multi-turn trajectories per minute. [arXiv 2511.11672](https://arxiv.org/abs/2511.11672)
- Modal's guide (2026): "Reinforcement learning for computer-use agents can require hundreds or thousands of parallel environments." [Modal resource](https://modal.com/resources/best-sandboxes-computer-use-agent-rl)
- Cua built Hoy for "customer demands for evals, and RL at scale" (2026-11-17). [HN 50534225](https://news.ycombinator.com/item?id=50534225)
- CUA-Lite (Microsoft/Berkeley authors including Dawn Song, 2026-10-19) says resources are "scattered" and "evaluation, RL, and SFT cannot share one infrastructure." It proposes "VM-free sandboxes that replicate OSWorld's desktop at much lower cost." [arXiv 2610.26071](https://arxiv.org/abs/2610.26071)
- Academics hit cloud quota walls (OSWorld VcpuLimitExceeded, 2025-10-27). [OSWorld issue 366](https://github.com/xlang-ai/OSWorld/issues/366)

**Agent startups (RPA replacement, QA, back-office, vertical)**
- Coasty (YC S26) builds agents for "legacy desktop software and web applications without usable APIs" (2026-07-15). [HN 48922706](https://news.ycombinator.com/item?id=48922706)
- TestDriver (QA) needed Windows support (2025-04-23). [HN 43773563](https://news.ycombinator.com/item?id=43773563)
- QA Wolf engineer (2026-10-06): "Everyone is giving agents their own sandboxes these days." [HN 49981045](https://news.ycombinator.com/item?id=49981045)
- Public-sector kiosks: "we're evaluating computer use agents for public kiosks ... Housing authorities, police departments, courthouses" (2025-04-26). [HN 43773563](https://news.ycombinator.com/item?id=43773563)
- Healthcare: OpenAI, "Advancing computer-use agents for healthcare teams" (2026-11-17, 53 pts). [HN 50535898](https://news.ycombinator.com/item?id=50535898)
- Many CUA-API startups per YC batch. A commenter asked how Coasty differs from "the other few CUA APIs in this batch and previous batches." [HN 48922706](https://news.ycombinator.com/item?id=48922706) (Secondhand) Stoa Research counted 10 F26 companies building VMs, sandboxes, or environments for agents. [Stoa Research](https://stoaresearch.substack.com/p/yc-f26-screening)

**Enterprises**
- AWS WorkSpaces agent access (preview, 2026-05-06): agents get IAM identities and a pre-signed URL to a WorkSpace. Instances range up to GPU, 32 vCPU, and 256GB RAM. [The Register](https://www.theregister.com/2026/05/06/aws_workspaces_agent_access/)
- Microsoft Windows 11 agent workspace (Nov 2025): "a separate, contained Windows session made just for AI agents, where they get their own account, desktop, and permissions" (quoted in HN). The story drew 703 pts. [HN 45960069](https://news.ycombinator.com/item?id=45960069); [HN story 45959795](https://news.ycombinator.com/item?id=45959795)
- E2B is used by large companies:
  - VentureBeat headline (July 2025): "how e2b became essential to 88 of fortune 100 companies." [VentureBeat](https://venturebeat.com/ai/how-e2b-became-essential-to-88-of-fortune-100-companies-and-raised-21-million)
  - (Secondhand) Sacra says that by Oct 2026 E2B reported more than 2 billion sandboxes started and 94 of the Fortune 100 signed up. [Sacra](https://sacra.com/c/e2b/)
- Claude Managed Agents self-hosted sandboxes, for keeping data "within your enterprise perimeter" (2026-05-19). [Claude blog](https://claude.com/blog/claude-managed-agents-updates)

**Hobbyists / local**
- "Recently I gave a Nix OS vm to my hermes agent and it has been a good experience. I don't really care if destroy the machine I can just rollback" (2026-06-24). [HN 48662999](https://news.ycombinator.com/item?id=48662999)
- Homelab (2026-11-03): "Beefy M6 Mini as the server. UTM managed MacOS VMs with screen sharing enabled. 1 VM per project." [HN 50358611](https://news.ycombinator.com/item?id=50358611)
- Agent37's early customers were "launching personal agents like Hermes, OpenClaw, Claude Code" (2026-10-12). [HN 50059096](https://news.ycombinator.com/item?id=50059096)
- Docker Sandboxes ("Disposable, isolated sandboxes for AI agents") got 694 pts and 396 comments (2026-08-10). [HN 49239751](https://news.ycombinator.com/item?id=49239751)

**Coding agents needing a GUI**
- "Computer use is very useful for developing GUI applications since claude code can build and test the entire app end-to-end" (2026-06-25). [HN 48662999](https://news.ycombinator.com/item?id=48662999)
- "I'm looking forward to a desktop OS optimized version so it can do the QA that I have no time for!" (2025-10-07). [HN 45507936](https://news.ycombinator.com/item?id=45507936)
- Cua's coding-agent offshoots:
  - "Show HN: CuaBot, Co-op computer-use for any coding agent" (2026-02-04). [HN 46887532](https://news.ycombinator.com/item?id=46887532)
  - "Show HN: Drive any macOS app in the background without stealing the cursor" (192 pts, 2026-04-28). [HN 47936312](https://news.ycombinator.com/item?id=47936312)
- Mobile: "I've been working with a lot of Android apps and emulation is killing me" on a MacBook Pro M1/32GB (2026-10-12). [HN 50059096](https://news.ycombinator.com/item?id=50059096)

### Inferences
- RL/eval is the segment where "a lot of sandboxed computers" literally means thousands, and the big labs mostly build in-house (Qwen on Alibaba ECS, DeepSeek DSec). The buyers who can't do that are academic groups, smaller labs, and RL-environment vendors, and they are the ones complaining about cost and quotas.
- Enterprise demand is being met top-down by hyperscalers and OS vendors (AWS WorkSpaces, Windows agent workspace). That squeezes independent startups unless they offer self-host, multi-OS, or better RL features.

### Gaps
- I found no data on how much each segment spends, or on how many desktops a typical agent startup runs.
- I found no first-person Reddit or X accounts from hobbyists running Claude/OpenAI computer use locally (both platforms blocked).

## 4. Market signals 2025-2026 (funding, big-lab moves, RL environment demand)

### Takeaway
Money in agent sandboxes is large and growing fast:
- Modal: $15.75B valuation (Oct 2026), with nearly a third of its more than $1B run rate from Sandboxes.
- Daytona: $1B valuation (Oct 2026), run rate up 25x in 9 months, with Anthropic, OpenAI, Meta, and Nvidia as customers.
- Browser infra: Browserbase at $300M (2025), Kernel $22M (2025).
- Newer computer-specific raises: Phaze $8.75M from a16z (Nov 2026).

Every major lab shipped computer use between 2025 and 2026, and labs are reported to be spending heavily on RL environments. Computer-use-specific sandbox vendors (Cua, Coasty, Orgo, Scrapybara) have far less visible funding than general sandbox vendors.

### Cited Findings
**Sandbox and browser-infra funding**
- Modal:
  - Series D (2026-10-14): $750M at $15.75B post-money, led by Accel, with Kleiner Perkins. Annualized run rate is over $1B and "nearly one-third" comes from Sandboxes. Modal plans RL and post-training services. [SiliconANGLE](https://siliconangle.com/?p=850503)
  - Series C (2026-05-21): $355M at $4.65B. [SiliconANGLE](https://siliconangle.com/2026/05/21/serverless-ai-infrastructure-startup-modal-labs-seals-355m-funding-round/)
  - (Secondhand) Sacra estimates Sandboxes alone at a $300M run rate by Oct 2026. [Sacra](https://sacra.com/c/modal-labs/)
- Daytona:
  - Series B (2026-10-20): $60M at a $1B valuation, led by FirstMark. Total raised is about $91M. Revenue run rate grew 25x from January to September, with 50.9% average monthly growth. Customers include Anthropic, OpenAI, Nvidia, Cursor, Cognition, Meta, Cohere, and Snorkel, and Snorkel alone has provisioned more than 1M sandboxes.
  - CEO Ivan Burazin on Meta's Muse agent: "a computer underneath for every single chat." On compute: "I strongly believe that there will be a CPU crunch."
  [Fortune](https://fortune.com/2026/10/20/daytona-startup-building-the-agentic-cloud-valued-at-1-billion-in-latest-funding-round/)
  - Earlier: "Daytona Raises $24M Series A to Give Every Agent a Computer" (2026-02-06). [HN 46911283](https://news.ycombinator.com/item?id=46911283)
- E2B:
  - $21M Series A (July 2025), led by Insight Partners. [E2B blog](https://e2b.dev/blog/series-a); [Insight Partners](https://www.insightpartners.com/ideas/e2b-raises-a-21m-series-a-to-offer-cloud-for-ai-agents-to-fortune-100/)
  - $11.5M seed (Oct 2024). [AIN](https://en.ain.ua/2024/10/18/e2b-raises-11m-in-seed-funding)
  - I found no 2026 Series B.
- Browserbase: $40M Series B at $300M (2025-06-17), led by Notable Capital. [SiliconANGLE](https://siliconangle.com/2025/06/17/browserbase-reels-40m-browser-automation-tools) I found no 2026 round.
- Kernel (browsers-as-a-service): $22M seed plus Series A led by Accel (Oct 2025). [Kernel blog](https://blog.onkernel.com/series-a-announcement); [Axios](https://www.axios.com/pro/enterprise-software-deals/2025/10/09/accel-22-million-kernel-ai-agents)
- Browser Use: $17M (2025-03-23). [TechCrunch](https://techcrunch.com/2025/03/23/browser-use-the-tool-making-it-easier-for-ai-agents-to-navigate-websites-raises-17m/)
- Phaze: $8.75M seed led by a16z (2026-11-04). CEO Benjy Boxer: "now AI agents need computers they can actually operate." [Utah Business](https://www.utahbusiness.com/press-releases/2026/11/04/phaze-raises-seed-led-a16z-give-ai-agents-access-computers-indie-nextview/)
- Cua (YC X25): aggregators list about $500K to $1M raised in 2025, and they conflict with each other. I found no 2026 round. [VCBacked](https://www.vcbacked.co/company/cua); [Caplight](https://www.caplight.com/company/cua)
- Commoditization narrative: Forkast headline, "the sandbox layer is commoditizing; modal just raised at 15.75x revenue anyway." [Forkast](https://forkast.news/the-sandbox-layer-is-commoditizing-modal-just-raised-at-15-75x-revenue-anyway/)

**Big-lab and platform moves**
- OpenAI:
  - "ChatGPT agent: bridging research and action" (2025-07-17, 686 pts). [HN 44595492](https://news.ycombinator.com/item?id=44595492)
  - "Equipping the Responses API with a computer environment" (2026-03-11). [OpenAI](https://openai.com/index/equip-responses-api-computer-environment/)
  - "Workspace Agents in ChatGPT" (2026-04-22, 161 pts). [HN 47866860](https://news.ycombinator.com/item?id=47866860)
  - Computer-use agents for healthcare teams (2026-11-17). [OpenAI](https://openai.com/index/advancing-computer-use-for-healthcare-teams/)
- Anthropic:
  - Acquired Vercept "to advance Claude's computer use capabilities" (2026-02-25). [Anthropic](https://www.anthropic.com/news/acquires-vercept)
  - Self-hosted sandboxes in Claude Managed Agents (2026-05-19). [Claude blog](https://claude.com/blog/claude-managed-agents-updates)
- Google:
  - Gemini 2.5 Computer Use model (2025-10-07, 636 pts). [Google blog](https://blog.google/technology/google-deepmind/gemini-computer-use-model/)
  - Computer use in Gemini 3.5 Flash (2026-06-24, 242 pts). [Google blog](https://blog.google/innovation-and-ai/models-and-research/gemini-models/introducing-computer-use-gemini-3-5-flash/)
  - Project Mariner shut down (The Verge via HN, 2026-05-07). [HN 48050287](https://news.ycombinator.com/item?id=48050287)
- Microsoft: Windows 11 agent workspace (Nov 2025). [HN 45959795](https://news.ycombinator.com/item?id=45959795)
- AWS: WorkSpaces for agents, preview (2026-05-06). [The Register](https://www.theregister.com/2026/05/06/aws_workspaces_agent_access/)
- Docker: Docker Sandboxes (2026-08-10, 694 pts). [HN 49239751](https://news.ycombinator.com/item?id=49239751)

**RL environment demand**
- TechCrunch (Maxwell Zeff, 2025-09-16/21):
  - "leading AI labs are now demanding more RL environments."
  - Citing The Information: "leaders at Anthropic have discussed spending more than $1 billion on RL environments" over the next year.
  - Surge CEO Edwin Chen reports a "significant increase" in demand.
  - Prime Intellect's Will Brown says RL environment training "can be more computationally expensive than previous AI training techniques."
  - An environment example: "simulate a Chrome browser and task an AI agent with purchasing a pair of socks on Amazon."
  [TechCrunch](https://techcrunch.com/2025/09/21/silicon-valley-bets-big-on-environments-to-train-ai-agents/)
- (Secondhand, vendor/directory blogs) Reported environment pricing is about $200 to $2,000 per task and about $20,000 per website or app replica. Lab contracts are six or seven figures per quarter. [Patronus FAQ](https://www.patronus.ai/resources/rl-environments-faq); [RL-List](https://www.rl-list.com/)
- Counter-signal: the blog post "Don't Build an RL Environment Startup" (2026-07-12). [HN 48884805](https://news.ycombinator.com/item?id=48884805)

### Inferences
- The fact that "nearly one-third" of Modal's more than $1B run rate is Sandboxes, together with Daytona's 25x run-rate growth, shows the agent-computer market is real at the nine-figure scale. Most of that spend is coding/Linux sandboxes and RL rollouts, not GUI desktops.
- Hyperscalers and OS vendors (AWS, Microsoft) entering agent desktops validates the category, and they will likely set enterprise expectations around identity, audit, and MCP access.

### Gaps
- I found no 2026 funding data for Scrapybara, Orgo, Steel, Hyperbrowser, Anchor, Morph, or Coasty.
- I found no verified figure for total lab spend on computer-use environments specifically, as distinct from RL environments in general.
- I could not open The Information's original $1B report directly (paywalled). It is cited here through TechCrunch.

## 5. Evidence of demand for an open-source / self-hosted option

### Takeaway
Open-source demand is strong:
- OSS agent and computer-use repos have very large star counts. trycua/cua went from 0 to 33k stars in 22 months, and Tencent's CubeSandbox reached 13.5k in about 7 months.
- Every few weeks in 2026 brought a new Show HN for a "self-hosted sandbox for agents."
- Users cite rug-pull and lock-in fear, which Daytona's closed-source move in June 2026 made worse.

The counter-trend is vendors moving to closed source or source-available licensing (Daytona, Cua Hoy, parts of Cua under FSL). Some prominent OSS desktop agents have also gone stale (Bytebot, last push Sept 2025).

### Cited Findings
- GitHub stars, read via the GitHub API on 2026-11-27:

  | Repo | Stars | Notes |
  |---|---|---|
  | browser-use/browser-use | 122,195 | |
  | OpenHands/OpenHands | 94,445 | |
  | daytonaio/daytona | 71,267 | archived |
  | trycua/cua | 33,064 | created 2025-01-31, MIT |
  | browserbase/stagehand | 26,166 | |
  | e2b-dev/E2B | 15,034 | |
  | browserless/browserless | 13,964 | |
  | TencentCloud/CubeSandbox | 13,477 | created 2026-04-10 |
  | simular-ai/Agent-S | 13,066 | |
  | bytebot-ai/bytebot | 11,074 | last push 2025-09-12 |
  | microsoft/UFO | 10,244 | |
  | steel-dev/steel-browser | 8,021 | |
  | agent-infra/sandbox | 6,398 | |
  | xlang-ai/OSWorld | 3,279 | |
  | CelestoAI/SmolVM | 2,087 | created 2026-02-15 |
  | e2b-dev/infra | 1,887 | |
  | e2b-dev/desktop | 1,576 | |
  | kernel/kernel-images | 1,160 | |
  | hud-evals/hud-python | 335 | |

  [GitHub trycua/cua](https://github.com/trycua/cua); [GitHub TencentCloud/CubeSandbox](https://github.com/TencentCloud/CubeSandbox); [GitHub e2b-dev/desktop](https://github.com/e2b-dev/desktop); [GitHub bytebot-ai/bytebot](https://github.com/bytebot-ai/bytebot)
- Self-hosted / open-source sandbox Show HNs, 2025-2026:
  - "BrowserStation: An open-source alternative to Browserbase" (2025-07-17). [HN 44596284](https://news.ycombinator.com/item?id=44596284)
  - "Era, Open-source local sandbox for AI agents" (2025-11-27, 62 pts). [HN 46065997](https://news.ycombinator.com/item?id=46065997)
  - "Verge Browser a self-hosted isolated browser sandbox for AI agents" (2026-03-12). [HN 47353827](https://news.ycombinator.com/item?id=47353827)
  - "Screenbox, Self-hosted virtual desktops for AI agents" (2026-04-02). [HN 47617846](https://news.ycombinator.com/item?id=47617846)
  - "SmolVM, open-source sandbox for coding and computer-use agents" (2026-04-10). [HN 47711887](https://news.ycombinator.com/item?id=47711887)
  - "We built a <60ms, open-source alternative to E2B using RustVMM and KVM" (Tencent CubeSandbox, 2026-04-22). [HN 47863430](https://news.ycombinator.com/item?id=47863430)
  - "Containarium, self-hosted sandbox for AI agents, MCP-native" (2026-05-14). [HN 48130526](https://news.ycombinator.com/item?id=48130526)
  - "Cordium, FOSS self-hosted sandbox platform alt. Codespaces/E2B/Daytona" (2026-06-07). [HN 48433738](https://news.ycombinator.com/item?id=48433738)
  - "AgentNest, self-hosted sandboxes for AI agents" (2026-07-23). [HN 49015852](https://news.ycombinator.com/item?id=49015852)
  - "Pi pod, Run your pi coding agent in sandboxes on your own server" (2026-10-02, 121 pts). [HN 49937304](https://news.ycombinator.com/item?id=49937304)
- Self-host demand in comments:
  - colesantiago wants a "one click sandbox agent on a self hosted instance that is, free, fast ... and that is open source" (2026-06-26). [HN 48688995](https://news.ycombinator.com/item?id=48688995)
  - Freestyle founder on the difficulty: "Self hosting can be doable for constant small/medium size workloads ... For hardware virtualized machines it much harder" (2026-04-06). [HN 47663147](https://news.ycombinator.com/item?id=47663147)
  - A self-hostable Firecracker orchestrator (bhatti) with a headless Chromium tier was offered in reply. [HN 47663147](https://news.ycombinator.com/item?id=47663147)
- Self-host requests in GitHub issues:
  - "[Bug]: No way to require authentication on /ws for a self-hosted computer-server instance" (2026-10-16). [cua issue 5366](https://github.com/trycua/cua/issues/5366)
  - "[RFC] Self-hosted multi-host paired remote Driver for physical PCs" (2026-07-25). [cua issue 2562](https://github.com/trycua/cua/issues/2562)
- Big labs and enterprises adding self-host paths: Claude Managed Agents self-hosted sandboxes (2026-05-19, [Claude blog](https://claude.com/blog/claude-managed-agents-updates)); OpenAI Agents API self-hosted environments ([HN 49651647](https://news.ycombinator.com/item?id=49651647), 2026-09-10). E2B offers BYOC only on Enterprise (from $3,000/month). [E2B pricing](https://e2b.dev/pricing)
- Counter-signals (closing up):
  - Daytona closed source on 2026-06-11 citing security. [Daytona post](https://www.daytona.io/dotfiles/updates/daytona-is-going-closed-source)
  - Cua's Hoy VMM is "closed-source to start with" (2026-11-17). [HN 50534225](https://news.ycombinator.com/item?id=50534225)
  - Modal's guide notes Cua has "mixed licensing," with some components (Cua Spaces, Cua Driver Bench) under FSL-1.1-MIT rather than MIT. [Modal resource](https://modal.com/resources/best-sandboxes-computer-use-agent-rl)

### Inferences
- Demand for an open-source, self-hostable computer-use sandbox is real and growing. Vendors are pulling back from open source at the same time (Daytona's archive, Cua's closed Hoy), which leaves room for a credible, permissively licensed, multi-OS option.
- Most self-hosted Show HNs are Linux/coding-focused with small traction. None in this sample combines a self-hosted fleet of Windows plus Linux desktops with RL-grade snapshot/fork. That combination looks unclaimed in OSS.

### Gaps
- Star counts measure interest, not production self-host deployments. I found no data on how many teams actually self-host E2B infra or Cua in production.
- Without Reddit (r/selfhosted) I could not measure self-host demand among homelab users directly.

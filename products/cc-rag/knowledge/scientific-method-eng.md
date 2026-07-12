# The scientific method applied to engineering decisions

## When
About to "optimize", "fix", or "improve" something on a plausible hunch. Run it as an
experiment so a wrong hunch dies cheaply instead of after wasted work.

## The loop
1. **Hypothesis** — state the believed cause/design explicitly.
2. **Prediction + measurement** — decide *before acting* what you'd observe if true, and
   build the instrument to observe it. **Validate the instrument** — can it actually
   distinguish the outcomes?
3. **Intervention** — the smallest change that tests the prediction.
4. **Observation** — read the real numbers/behavior, not what you expected.
5. **Falsification** — actively try to kill the hypothesis; report what the data showed.

## Hard-won lessons (from real falsifications)
- **A plausible cause is often wrong.** Hypothesis "plugin stack drives agent cost" was
  killed by measurement (prefix ~3k vs cache-write ~1.7M/session) *before* any plugins
  were disabled — saving the wasted intervention. That is the whole point of measuring.
- **Trust the instrument only if it can adjudicate.** A "fix looks worse" signal on an
  n=5 eval is noise, not evidence — swing per item ≈ 0.2. When the instrument is too
  weak, keep the principled default and *improve the measurement first*.
- **Don't chase noise into your design.** Overriding a principled choice because a tiny,
  noisy metric moved is bad science.
- **Report faithfully.** If it failed, say so with the output. If a step was skipped,
  say that. State done-and-verified plainly; don't hedge or dress up.

## В бою
"Let's just disable X / switch to Y to make it faster/cheaper" → first: what's the
hypothesis, what would confirm it, and can I measure that? Often the measurement alone
resolves it without the change.

# Calibrating DDD / Clean Architecture / SOLID against YAGNI / KISS

## When
A request asks for "best practices — DDD, Clean Architecture, SOLID" *and* "keep it
simple, YAGNI." These pull opposite directions. The skill is calibrating, not
cargo-culting all of them.

## The calibration
- **Apply the parts that earn their keep:**
  - **Ports & adapters** where a component realistically gets swapped (the embedder,
    the store, the data source). This buys reusability + testability; **SOLID falls out
    for free** (DIP via the port, SRP per component).
  - **Ubiquitous language** — clean domain terms — is cheap and always worth it.
- **Skip what's ceremony for a thin domain:**
  - Full tactical DDD (aggregates, domain events, CQRS, repository-for-everything) on a
    pipeline/tool with no rich invariants is gold-plating = a KISS/YAGNI violation.
  - A 300-line tool needs 1–2 real seams + clean function boundaries, **not** a hexagonal
    framework.
- **Knowing where a pattern is NOT needed is itself best practice.** "Apply all the
  patterns" is the junior move; "apply the right amount for this domain" is the senior one.

## Rule of thumb
Structure must be justified by a real, near-term need (a likely swap, a real invariant,
a real boundary), not by "it's more proper." A learning project may add structure
*deliberately as practice* — but name that as the goal; don't smuggle ceremony in as
"best practice."

## В бою
Asked to "build it properly with DDD/Clean Arch" → cut your own design first: which
seams are real? Everything else is plain, clean code. Present the calibration, not the
full pattern catalogue.

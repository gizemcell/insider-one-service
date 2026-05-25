# Architecture Decision Records

Each ADR documents a significant technical choice: the context, the decision, and the reasoning behind it.

Format: `ADR-NN — Title` → Status → Context → Decision → Consequences

---

### ADR-01 — ArgoCD for GitOps deployment

**Status**: Accepted

**Context**

The service needs a deployment mechanism that scales beyond a single engineer running `kubectl apply`. CI pipelines with direct cluster access create a long-lived secret (kubeconfig) stored in the CI system, and any drift between what was deployed and what is in Git goes undetected.

**Decision**

Instead of deploying directly from CI using `kubectl`, the pipeline updates the Git repository (image tag in `chart/values-*.yaml`) and ArgoCD reconciles the cluster state against the `main` branch.

**Consequences**

- No cluster credentials stored in CI — the attack surface is reduced.
- Declarative desired state: the cluster always converges to what Git says.
- Automatic drift correction via `selfHeal: true` — manual `kubectl` changes are reverted.
- Full deployment history is the Git log; rollback is a `git revert`.

---

### ADR-02 — Alpine runtime image

**Status**: Accepted

**Context**

Distroless images were evaluated for their smaller attack surface and lack of a shell. However, the service's `Dockerfile` uses Docker's `HEALTHCHECK` instruction (`wget --spider`), which requires a minimal userspace tool. Distroless images do not ship `wget` or a shell, making this incompatible without replacing the healthcheck mechanism entirely.

**Decision**

Alpine was selected as the runtime base image.

**Consequences**

- Docker `HEALTHCHECK` works out of the box via `wget`.
- A minimal shell is available for debugging in development and on the EC2 host.
- Image size stays small (Alpine base ~7 MB).
- A broader OS package surface exists compared to distroless; mitigated by Trivy scanning in CI and regular base image updates.
- Acceptable trade-off for demo and development workflows where debuggability matters.

---

### ADR-03 — Trivy + govulncheck split

**Status**: Accepted

**Context**

Container image vulnerability scanning and Go dependency vulnerability scanning address different layers. A single tool covering both tends to produce noisy results: image scanners report Go CVEs that `go mod` has already patched, and Go-specific tools miss OS-level packages.

**Decision**

Two tools are run in CI:

- **Trivy** — scans the built container image for OS-level vulnerabilities (Alpine packages, base image CVEs). Results are uploaded to the GitHub Security tab as SARIF.
- **govulncheck** — scans Go module dependencies against the Go vulnerability database, reporting only vulnerabilities reachable by the actual code paths.

**Consequences**

- Reduced false positives: each tool operates in its domain.
- Better coverage: OS layer and Go dependency layer are both checked independently.
- Two CI steps to maintain, but both are lightweight and cached.
- `govulncheck` only flags reachable vulnerabilities, so noise from transitive-but-unused dependencies is suppressed.

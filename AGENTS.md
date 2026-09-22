# AGENTS.md

## Read first
Before making changes, read:

1. `intel/history.md` — append-only historical record of significant repository
  changes.
2. `intel/notes.md` — durable engineering notes and unresolved technical questions.
3. `intel/plan.md` — active implementation plans and follow-on work.
4. `intel/cybersec.md`— security requirements, identified security issues,
  remediation items, and the status of security fixes.
5. `intel/maint.md` — authoritative architecture and maintainability guidance.
6. `intel/map.md`— repository structure map, component descriptions, and relevant
  technical diagrams

## Instruction Precedence
- Follow all applicable platform, organization, and account-level instructions.
- Then follow this `AGENTS.md`.
- If instructions conflict, do not make the conflicting change. Identify the
  conflict and request clarification or propose a compliant alternative.
- These instructions apply to the entire repository unless a closer `AGENTS.md`
  provides more-specific guidance.
- Follow repository instructions when they conflict with personal preferences.
- If requirements conflict or are ambiguous, identify the conflict and ask before
  making a broad or irreversible change.
- Do not create commits, amend commits, force-push, or modify Git history.

## Repository Intelligence Documents
Maintain the following documents in the repository-root `intel/` directory:
- `intel/maint.md` — authoritative architecture and maintainability guidance.
- `intel/map.md` — repository structure map, component descriptions, and relevant
  technical diagrams.
- `intel/cybersec.md`— security requirements, identified security issues,
  remediation items, and the status of security fixes.
- `intel/history.md` — append-only historical record of significant repository
  changes.
- `intel/notes.md` — durable engineering notes and unresolved technical questions.
- `intel/plan.md` — active implementation plans and follow-on work.
Maintain `CONTRIBUTING.md ` and `README.md` at the repository root.

## Document Update Rules
- Create a required document only if it does not already exist.
- Update a document only when the requested change materially affects the
  document's purpose.
- Preserve existing content and structure unless an update is necessary.
- Do not delete, truncate, rename, or overwrite content in `intel/history.md`.
  Append new entries only when recording a significant change.
- Treat `intel/maint.md` as the authoritative source for repository architecture
  and maintainability guidance.
- Keep `CONTRIBUTING.md ` consistent with `intel/maint.md` and focused on
  contributor setup, workflow, validation, coding expectations, and pull-request
  expectations.
- Keep `intel/map.md` concise. Update it when files, directories, components,
  dependencies, or major data flows materially change.
- Use Mermaid diagrams in `intel/map.md` when a diagram improves understanding of
  a significant architecture, dependency, request flow, or data flow. Do not add
  diagrams for trivial changes.
- Do not include secrets, credentials, tokens, private keys, sensitive production
  data, or other restricted content in these documents.

## Security Issue Tracking
- Use `intel/cybersec.md` to identify and document security issues discovered
  during repository analysis, implementation, review, testing, or maintenance.
- For each identified security issue, create an actionable remediation item in
  `intel/cybersec.md` that includes:
  - a unique identifier;
  - a concise description of the issue and affected component;
  - the security impact or risk;
  - required remediation steps;
  - status: `Open`, `In Progress`, `Blocked`, or `Closed`;
  - validation required to confirm the remediation.
- Update the remediation item as work progresses.
- Mark an item `Closed` only after the documented remediation is implemented and
  the required validation has been completed.
- Never delete closed security items. Preserve them as an audit trail of security
  decisions, fixes, and validation.
- Do not modify code, configuration, dependencies, infrastructure, tests, or
  documentation in a way that weakens, removes, bypasses, or regresses a
  documented security remediation.
- If a requested change would conflict with a closed or active security
  remediation item, do not implement the conflicting portion. Document the
  conflict and propose a compliant alternative.
### SEC-001 — Example issue title

- **Status:** Open
- **Affected component:** `src/auth/`
- **Risk:** [Concise description]
- **Required remediation:** [Specific corrective action]
- **Validation:** [Test, review, scan, or other evidence required]
- **Resolution:** [Completed only when status is Closed]

## Change discipline
- Keep changes narrowly scoped to the requested task.
- Do not perform unrelated refactoring, formatting, dependency upgrades, or file
  renames unless explicitly requested.
- Preserve existing public behavior and API compatibility unless the task
  explicitly authorizes a breaking change.
- Follow established patterns in the nearest comparable module before introducing
  a new abstraction or framework.

## Implementation standards
- Use the repository’s existing language, framework, formatting, and error-
  handling conventions.
- Prefer small, maintainable changes over broad rewrites.
- Do not introduce a new dependency when an existing dependency or platform
  capability satisfies the requirement.
- Add comments only when they explain non-obvious intent, constraints, or tradeoffs.
- Do not suppress compiler, linter, type-checker, or security warnings without
  documenting why the suppression is necessary.

## Protected files
- Do not manually edit generated files under `generated/`, `dist/`, or
  `vendor/`; modify the source or generator instead.
- Do not modify lockfiles unless a dependency change is required.
- Do not alter CI, deployment, infrastructure, or environment configuration
  unless the task explicitly requires it.

## Security
- Never commit, log, hard-code, or expose credentials, tokens, private keys,
  connection strings, or production data.
- Do not weaken authentication, authorization, input validation, encryption, or
  audit logging to make a change easier.
- Do not disable security checks, linters, type checks, or tests merely to make a
  build pass.
- Flag security-sensitive changes for human review.

## API design requirements
New and modified HTTP APIs must follow the repository REST API conventions.
Unless an existing API pattern or approved design document requires otherwise:
- Model endpoints as resources using plural nouns, e.g., `/users` and `/users/{userId}`.
- Do not use verb-based paths such as `/createUser` or `/deleteUser`.
- Use HTTP methods according to their defined semantics:
  - `GET` for retrieval;
  - `POST` for creation or non-idempotent processing;
  - `PUT` for full replacement;
  - `PATCH` for partial update;
  - `DELETE` for deletion.
- Return appropriate HTTP status codes and a consistent error-response schema.
- Preserve backward compatibility for published endpoints unless the task explicitly authorizes a breaking change.
- Follow existing repository conventions for authentication, pagination, filtering,
  sorting, API versioning, naming, and serialization.
- When a requirement conflicts with these rules, follow the established API pattern
  and document the exception in the change summary.

## Required README.md process
1. Inspect the repository before editing `README.md`.
2. Ground all README content in the current codebase, configuration, scripts,
   dependencies, examples, tests, and existing documentation.
3. Do not invent features, commands, environment variables, ports, deployment
   steps, supported platforms, or integrations.
4. Preserve accurate existing README content where possible; improve, reorganize,
   or remove content only when it is outdated, duplicated, unclear, or unsupported
   by the repository.
5. Follow the repository’s existing documentation style and instructions in
   `AGENTS.md` and applicable documentation files.

## README content
Include applicable sections, in this order where practical:

1. **Project title and short description**
   - State the project purpose in plain language.
   - Identify the primary users or intended use.

2. **Features**
   - List the implemented, user-relevant capabilities.
   - Do not list planned, experimental, or inferred features as complete.

3. **Use cases**
   - Provide realistic examples of how the project can be used.
   - Keep examples consistent with actual repository functionality.

4. **Prerequisites**
   - Identify required runtimes, package managers, external services, tools, and
     supported versions only when verified from repository files.

5. **Installation**
   - Provide complete, copyable installation steps.
   - Use the repository’s actual package manager and setup commands.
   - Document required environment configuration and reference example environment
     files when they exist.
   - Clearly distinguish required versus optional configuration.

6. **Quick start / usage**
   - Show the minimum steps to run the project locally.
   - Include verified commands and expected access URL, port, output, or behavior
     when available.

7. **Configuration**
   - Document relevant configuration options and environment variables.
   - Do not expose secret values; use placeholders such as `YOUR_API_KEY`.

8. **Containerization**
   - Include this section only if the repository contains container-related files
     such as a `Dockerfile`, `docker-compose.yml`, `compose.yaml`, container build
     scripts, or Kubernetes manifests.
   - Document the actual build and run commands supported by those files.
   - Explain required environment variables, ports, volumes, and service
     dependencies when verified.
   - Do not add Docker or container instructions if no containerization support
     exists in the repository.

9. **Testing and quality checks**
   - Document actual commands for tests, linting, formatting, type checking, and
     builds when available.

10. **Project structure**
    - Provide a concise directory map for important directories and files.

## Quality requirements
- Use concise, professional Markdown with clear headings and copyable code blocks.
- Prefer command examples that can be run from the repository root.
- Clearly label optional steps and platform-specific instructions.
- Use relative links to repository documentation where appropriate.
- Do not add badges, screenshots, architecture diagrams, or external links unless
  they already exist, are supported by repository content, or materially improve
  the README.
- Do not claim that any command was executed unless it was actually run.
- If required information is missing or cannot be verified, omit it or add a
  clearly labeled `TODO` only when maintaining the README requires human input.

## Completion report
At completion, provide:
1. A concise summary of the change.
2. Files changed and their purpose.
3. Validation commands run and results.
4. Tests not run, assumptions made, unresolved issues, and residual risks.

After updating the README, provide:
1. A concise summary of sections added or changed.
2. Any repository facts that could not be verified.
3. Any commands or validation checks run, including results.

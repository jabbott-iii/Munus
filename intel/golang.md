# Go Development Instructions

These instructions apply only to Go-related work.

## Scope

Apply these instructions when working with:

- Go source files (`*.go`)
- Module files (`go.mod`, `go.sum`, `go.work`)
- Go tests, benchmarks, examples, generated code, and documentation
- Go build, lint, formatting, dependency, CI, release, or tooling configuration
- Go services, CLIs, libraries, APIs, and concurrency designs

For mixed-language tasks, apply these instructions only to the Go-related portion of the work.

## Before Making Changes

1. Inspect the relevant package, nearby code, tests, module configuration, and project conventions.
2. Identify the module path, supported Go version, existing tooling, and established patterns before proposing changes.
3. Make the smallest coherent change that satisfies the task.
4. Preserve existing package boundaries, public APIs, error conventions, and dependency choices unless the task requires a change.
5. Do not add dependencies, modify `go.mod` or `go.sum`, change generated files, alter CI, or redesign public APIs unless necessary.
6. Do not edit generated files directly. Modify the source or generator input and document any required generation command.

## Code Style and Design

- Write idiomatic Go compatible with the repository’s declared Go version.
- Format all modified Go files with `gofmt -w -s`.
- Use `goimports` only if it is already part of the repository tooling.
- Prefer simple, readable code over abstraction, indirection, reflection, or generic frameworks.
- Follow existing package naming and directory conventions.
- Use short, conventional local variable names when context makes their purpose clear; use descriptive names for exported identifiers and non-obvious values.
- Keep functions focused. Extract helpers when they meaningfully improve readability, testability, or reuse.
- Prefer composition over inheritance-like patterns or unnecessary interfaces.
- Define interfaces at the consumer boundary and keep them small. Do not introduce an interface solely to make a single implementation appear abstract.
- Avoid global mutable state. If state must be shared, make ownership, lifecycle, and synchronization explicit.
- Use generics only when they provide a concrete reduction in duplication or improve type safety without obscuring the code.

## Package and API Design

- Follow Go visibility conventions: exported names must have a clear external purpose.
- Do not export identifiers unless they are required outside the package.
- Keep packages cohesive; avoid utility packages that collect unrelated functions.
- Avoid import cycles and do not restructure broad portions of the repository merely to avoid a small local design issue.
- Preserve backward compatibility for public APIs unless the task explicitly authorizes a breaking change.
- Add Go doc comments for exported types, functions, methods, constants, and variables when required by repository conventions or when behavior is not self-evident.
- Start exported identifier comments with the identifier name when practical.

## Error Handling

- Handle errors explicitly and immediately.
- Return errors to the appropriate caller, wrap them with useful operational context, or intentionally handle them with a documented reason.
- Use `%w` with `fmt.Errorf` when callers need to inspect or classify the underlying error with `errors.Is` or `errors.As`.
- Do not discard errors silently.
- Do not use `panic` for expected runtime, user-input, I/O, network, or service failure conditions.
- Use `panic` only for truly unrecoverable programmer invariants or initialization failures where the repository convention supports it.
- Match existing repository error conventions before introducing custom error types, sentinel errors, or an error package.
- Avoid error-string matching; use `errors.Is`, `errors.As`, typed errors, or established sentinels.

## Context, I/O, and Resource Management

- Accept `context.Context` as the first parameter for operations that may block, perform I/O, call external systems, or outlive the immediate call stack.
- Do not store `context.Context` in structs.
- Do not pass `nil` contexts; use `context.Background()` or `context.TODO()` only at appropriate top-level boundaries.
- Propagate caller contexts unless there is a clear, documented reason to derive a child context.
- Set timeouts and deadlines at service or operation boundaries according to existing project patterns.
- Always close resources such as files, HTTP response bodies, database rows, streams, and network connections.
- Place `defer` immediately after successful acquisition when it is appropriate and does not create unacceptable resource retention in a long-running loop.
- Avoid reading unbounded input into memory. Apply size limits and stream data where appropriate.

## Concurrency

- Do not introduce goroutines, channels, mutexes, worker pools, or parallelism unless the task requires them.
- Every goroutine must have a clear owner, purpose, cancellation path, and termination condition.
- Propagate context cancellation to goroutines and external calls.
- Avoid goroutine leaks, blocked channel sends or receives, and unbounded concurrency.
- Define channel ownership clearly: the sender is normally responsible for closing a channel.
- Do not close a channel from multiple locations.
- Protect shared mutable state with appropriate synchronization or redesign to avoid sharing it.
- Do not hold mutexes while performing blocking I/O, invoking unknown callbacks, or sending on channels unless explicitly justified.
- Use the race detector for modified concurrent code when available.

## HTTP and External Services

- Reuse configured `http.Client` instances; do not create a new default client for every request.
- Configure timeouts for outbound HTTP clients according to project conventions.
- Always close HTTP response bodies.
- Check HTTP status codes before decoding response bodies.
- Validate and bound untrusted request input at the boundary.
- Avoid exposing internal errors, credentials, stack traces, tokens, or sensitive implementation details in API responses or logs.
- Preserve existing API versioning, authentication, authorization, and serialization conventions.

## Security

- Treat external input as untrusted, including HTTP requests, files, environment variables, command-line arguments, message queues, and database values.
- Validate input length, format, ranges, required fields, and authorization at the relevant boundary.
- Do not log credentials, session tokens, API keys, private keys, secrets, or sensitive personal or mission data.
- Do not weaken TLS verification, authentication, authorization, rate limits, input validation, audit logging, or security controls for convenience.
- Use standard-library or established project libraries for cryptography; do not implement cryptographic algorithms or protocol logic from scratch.
- Use parameterized queries or the project’s approved data-access layer; never construct database queries by concatenating untrusted input.
- Use `os/exec` only when necessary. Avoid shell invocation, pass arguments separately, validate inputs, and set appropriate execution context and timeouts.

## Dependencies and Modules

- Prefer the Go standard library and dependencies already used by the repository.
- Add a dependency only when it materially improves correctness, security, or maintainability.
- Before adding a dependency, consider maintenance, license compatibility, security posture, module size, API stability, and existing alternatives.
- Keep dependency changes minimal and intentional.
- Run `go mod tidy` only when dependency changes require it or repository policy directs it.
- Review resulting `go.mod` and `go.sum` changes; do not accept unrelated dependency churn.
- Do not manually edit `go.sum` unless repository policy specifically requires it.
- Respect `replace`, `exclude`, workspace, vendor, and private-module configuration already present in the repository.

## Testing

For behavior changes, add or update tests at the appropriate level:

- Unit tests for isolated business logic and edge cases.
- Table-driven tests when multiple inputs share a common behavior pattern.
- Integration tests for package boundaries, datastore interactions, HTTP behavior, or external contracts.
- Regression tests for defects.
- Benchmarks only when performance is a stated concern or a performance-sensitive path changes.

Tests should cover:

- Expected behavior
- Invalid or malformed input
- Error paths
- Boundary conditions
- Cancellation and timeout behavior where applicable
- Concurrency behavior where applicable

Testing guidance:

- Prefer deterministic tests.
- Avoid timing-dependent sleeps where synchronization or test hooks are available.
- Use `t.Helper()` in reusable test helpers.
- Use `t.TempDir()` for temporary filesystem state.
- Use `t.Cleanup()` for test cleanup.
- Use `httptest` for HTTP client or handler testing when appropriate.
- Do not make unit tests depend on public network access, real cloud accounts, or ambient machine state unless the repository explicitly designates them as integration tests.
- Do not weaken assertions merely to make tests pass.

## Logging and Observability

- Follow the repository’s logging and telemetry conventions.
- Use structured logging when the project supports it.
- Include useful operational context without logging secrets or sensitive data.
- Do not introduce a new logging framework or telemetry dependency unless explicitly required.
- Preserve correlation IDs, trace context, metrics labels, and error classification conventions when they exist.

## Validation

Before completing Go work, run the repository-required commands. Unless project instructions specify otherwise, use the applicable commands below:

```sh
gofmt -w -s <modified-go-files>
go vet ./...
go test ./...
```

For concurrency-related changes, also run:

```sh
go test -race ./...
```

For module or dependency changes, also run:

```sh
go mod tidy
go test ./...
```

If the repository uses additional tools, run them as required, for example:

```sh
golangci-lint run
staticcheck ./...
```

Do not claim a command passed unless it was actually run. If command execution is unavailable, list the commands that should be run and state that validation was not performed.

## Completion Report

When completing a Go task, provide:

1. A concise summary of the change.
2. Files changed and the reason for each.
3. Tests and validation run, including results.
4. Any assumptions, limitations, migration considerations, or recommended follow-on work.

Do not make unrelated refactors, dependency upgrades, broad formatting churn, API redesigns, or module-wide changes unless explicitly requested.

# Architecture

Hexagonal architecture (ports and adapters, Alistair Cockburn) with Domain-Driven Design vocabulary (Eric Evans). It is a tool for managing change, not a goal. `docs/code-style.md` applies first: KISS and YAGNI.

## Start flat, grow into layers
The skeleton in `app/` is deliberately one package. Introduce layers only when a trigger appears:
- a second adapter for the same port (another database, another transport),
- domain rules that are hard to test because they are tangled with I/O,
- more than one developer or agent working on the same feature area.

A layer introduced earlier is code the scenarios did not ask for.

## The shape
- **Domain**: the rules of the business. Entities, value objects, domain services, domain errors. Depends on the standard library only. No I/O, no goroutines, no time or randomness taken from the environment (pass a clock or an ID source in).
- **Application**: use cases. It decides what happens and in which order: load, call the domain, save, publish. It contains no business rules.
- **Adapters, driving (inbound)**: HTTP, gRPC, message consumers, CLI. Decode the request, call one use case, encode the response, map errors to the protocol's status codes. No business logic.
- **Adapters, driven (outbound)**: database, message producers, HTTP clients. Implement the ports the inner layers declare.
- **Composition root** (`main`): the only place that constructs concrete types and wires them together.

## Rules
1. **Dependencies point inward.** Adapters import application and domain; domain imports no adapter and no application code. An import in the wrong direction is a defect.
2. **Ports belong to the consumer.** The package that needs a capability declares a small interface for it (one to three methods). The adapter satisfies it implicitly. Do not declare the interface next to the implementation.
3. **No foreign types cross a boundary.** Database rows, protobuf messages, JSON DTOs and client-library types are converted at the adapter. Inner layers see domain types, or simple application inputs.
4. **One use case, one responsibility.** A use case is a function or a small struct with one public method, named for what it does (`PlaceOrder`, not `OrderService`).
5. **Errors:** domain and application define typed or sentinel errors; adapters translate them to protocol responses. Wrap with `%w` as errors cross layers so `errors.Is` and `errors.As` keep working. Never collapse an unknown error into a generic one without logging it.
6. **Transactions are an adapter concern.** If a read-decide-write must be atomic, the port exposes it as one operation that takes the decision as a function.
7. **Time, randomness and IDs are injected**, so domain tests are deterministic.

## DDD vocabulary used here
- **Ubiquitous language:** names in code, scenarios and ADRs are the words the client used in the conversation. Rename code when the language changes.
- **Entity:** has identity that persists through change. **Value object:** defined by its values, immutable, validated on construction.
- **Aggregate:** a cluster of entities changed together behind one root, with the invariant enforced inside it. References between aggregates go by ID. Keep aggregates small.
- **Domain service:** a rule that does not belong to one entity.
- **Bounded context:** an area where one model and one language hold. Two contexts may use the same word differently; translate at their boundary instead of sharing a type.
- Do not model what no scenario asks for. Anemic structures are fine in a small product; reach for aggregates when an invariant spans several fields.

## Testing per layer
- Domain: plain unit tests, no mocks.
- Application: unit tests with fakes or generated mocks for the ports it declares.
- Inbound adapters: tests with the use case faked, asserting protocol behaviour.
- Outbound adapters: integration tests against the real technology in a container when mocking would be expensive.
- Scenarios in `scenarios/` exercise the system through its inbound adapter.

## Anti-patterns
- Domain package importing an adapter or application package.
- A goroutine, ticker or `Run` loop in the domain.
- Business rules in a handler, consumer or repository.
- An interface declared next to its only implementation, "for mocking".
- A driver or transport type in a use case signature.
- A shared "models" package used by every layer.
- Layers or aggregates added before a trigger above exists.

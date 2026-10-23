# Discussion answers

Fill in the sections below. Brief explanations and explicit tradeoffs are enough;
there is no single required architecture. See [CODING_TEST.md](CODING_TEST.md).

## 1. Structure and tradeoffs

How did you structure configuration, simulator access, MQTT messaging, and
orchestration? What tradeoffs did you make within the timebox?

<!-- Your answer. -->

## 2. Hundreds of devices

How would you extend the agent to support hundreds of devices with different
credentials and polling intervals?

<!-- Your answer. -->

## 3. Certificates and credentials

How would you provision trusted certificates and manage credentials outside
this local development setup?

<!-- Your answer. -->

## 4. Outages, stale state, and buffering

What happens to telemetry and commands when the broker or simulator is
unavailable? How would you avoid stale state and unbounded buffering?

<!-- Your answer. -->

## 5. Duplicate delivery

MQTT can deliver duplicate messages. Why is setting a desired switch state
useful, and how would you handle deduplication for non-idempotent commands?

<!-- Your answer. -->

## 6. Diagnostics

Which logs and metrics would you add to diagnose authentication failures,
slow devices, and lost MQTT connections?

<!-- Your answer. -->

## Submission notes

- Approximate time spent:
- Configuration defaults and required values:
- MQTT QoS, retained flags, and client/session settings chosen:
- Shutdown bound and what is cancelled:
- Retry/backoff policy for simulator and broker failures:
- Incomplete requirements, if any:
- Optional extensions attempted, if any:
- Verification commands run and results:
- Improvements with more time:

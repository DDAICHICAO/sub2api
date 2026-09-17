# Private distribution and data-preserving deployment

Execute in the current session, on `codex/my-feature`.

Goal: remove gateway-generated explicit product identifiers from upstream requests and use DDAICHICAO/sub2api for releases. Preserve all existing production data.

Architecture: keep module names, database schema, stored identity derivation, API compatibility and license notices unchanged. Replace generated outbound labels with neutral labels; never globally rewrite user prompts or tool results. Scope update caches to the fork and reject old repository cache entries. Pin production to an immutable fork build.

1. Add regressions covering generated outbound instructions/tool aliases and fork-only update lookup/cache.
2. Change outbound defaults in the Grok OAuth, Ollama usage, Codex transform/tool/model, Vertex batch and connectivity test paths. Strip internal product-specific request headers at the HTTP upstream boundary. Add an explicit deployment switch to disable proprietary billing probes, including manual probes.
3. Switch updater, user-facing release links, installer sources and default image references to the fork. Preserve release archive/binary names for updater compatibility. Update image source labels.
4. Run focused Go unit tests plus existing transformation/transport/update tests; build the embedded frontend and production image.
5. Save a PostgreSQL logical backup, Redis snapshot, app configuration/data and old image. Record protected table hashes/counts and mounts. Recreate only the app container using existing compose files and mounts.
6. Verify exact deployed version/image, update repository, protected data and real authenticated public API path. Publish the source and release through the user's GitHub repository. Keep rollback instructions with the deployment record.

Limits: removing explicit generated names is not a guarantee of traffic anonymity. User-supplied historical text, provider-required OAuth identity, protocol behavior and legally required attribution remain meaningful limitations.

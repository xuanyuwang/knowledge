## Summary

Harden the frontend feature-flag updater for production-scale customer configurations discovered while applying CONVI-7642.

- Locate only profile/use-case frontend `featureFlags` blocks instead of counting unrelated blocks elsewhere in customer YAML.
- Add an explicit `--application-level profile-only` mode mirroring ConfigService `LEVEL_PROFILE_ONLY`.
- Document the narrow fallback for customers whose mapless use cases would cause `LEVEL_ALL` to reject the entire customer.

## Safety boundary

The default remains `all` and continues to fail closed when an in-scope profile or use case lacks a frontend feature-flag map. Profile-only mode must use a separate explicit manifest, and the caller must verify affected use cases inherit the profile value and have no conflicting explicit value.

## Validation

- Python syntax compilation passed.
- Skill frontmatter parsed and required fields validated.
- Real production dry run passed for 356 `LEVEL_ALL` customers and 1,949 maps.
- Real production dry run passed for six explicit `LEVEL_PROFILE_ONLY` customers and six profile maps.
- Post-application repeat dry runs reported zero changes and all targeted maps already desired.
- The resulting config was independently checked as 1,972/1,972 effective profile/use-case scopes.

# Migrate from `github.com/google/uuid` to the standard `uuid` package

Go 1.27 added the standard library package `uuid`. sakumock supports one
version behind the latest stable Go and keeps the `go` directive at 1.26, so it
cannot use it yet. Do this migration once Go 1.28 is released and Go 1.26
support is dropped.

## Tasks

- [ ] Raise the `go` directive in `go.mod` to 1.27. This is the feature
      requirement the go.mod policy in `AGENTS.md` asks for. Note it in the
      PR/CHANGELOG, because library consumers will need Go 1.27.
- [ ] Replace the call sites. The std API is smaller than google/uuid's:

      | google/uuid | std `uuid` |
      |---|---|
      | `uuid.NewString()` | `uuid.New().String()` (`New` is v4) |
      | `uuid.Must(uuid.NewV7())`, `uuid.NewV7()` | `uuid.NewV7()` (no error) |
      | `uuid.Parse`, `uuid.MustParse` | same |
      | `id.Version()` (tests) | not provided; check `id[6]>>4 == 7` or drop the check |
      | `uuid.NewSHA1(uuid.NameSpaceURL, ...)` (`apigw/store_memory.go`, deterministic plan IDs) | no v5 in std; hard-code the plan IDs or derive them some other way, keeping the current values so existing IDs stay stable |

- [ ] Handle the SDK types. The SDK-generated types (`v1.ApplicationID`,
      `v1.WorkerNodeID`, apigw IDs, ...) are defined on `github.com/google/uuid`.
      Both types are `[16]byte`, so a conversion works
      (`uuid.UUID(a.GetApplicationID())`). Test helpers that take or return SDK
      IDs (`apigw/*_test.go`) may keep google/uuid in their signatures, or
      convert at the boundary.
- [ ] Check go.mod. google/uuid stays an indirect dependency through
      sacloud-sdk-go. Confirm it moves from the direct `require` block to
      `// indirect` once no package imports it directly.
- [ ] Update the "UUID generation" section in `AGENTS.md` to point at the std
      package.
- [ ] Delete this file.

## Current importers

Run `grep -rl '"github.com/google/uuid"' --include='*.go' .` for the
up-to-date list. At the time of writing these import it: addon, apigw (store and
tests), apprun, apprundedicated, iam, monitoringsuite, objectstorage, simplemq,
workflows (`expr/funcs.go`).

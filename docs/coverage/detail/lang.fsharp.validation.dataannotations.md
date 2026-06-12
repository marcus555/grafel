<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.fsharp.validation.dataannotations` — DataAnnotations (F# records)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [F#](../by-language/fsharp.md)
- **Category:** [validation](../by-category/validation.md)
- **Subcategory:** Validation
- **Capability cells:** 6

## Capabilities


### Schema

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Nested model extraction | 🔴 `missing` | — | 5049 | — | Nested record references are captured as field types but not expanded into a structured nested-schema tree, and no owner->nested VALIDATES edge is materialised yet. Honest-partial deferred to a follow-up. |
| Schema extraction | ✅ `full` | `2026-06-13` | 5049 | `internal/extractors/fsharp/du_record_members.go`<br>`internal/extractors/fsharp/fsharp_test.go` | #4942: F# record types emit each field as a SCOPE.Schema/field sub-entity (extractTypeMembers/parseRecordFields), with a type->field CONTAINS edge. Proven by TestFSharp_RecordFields. |

### Constraints

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Constraint extraction | ✅ `full` | `2026-06-13` | 5049 | `internal/extractors/fsharp/du_record_members.go`<br>`internal/extractors/fsharp/field_validations.go`<br>`internal/extractors/fsharp/fsharp_test.go` | #5049: System.ComponentModel.DataAnnotations attributes on record fields ([<Required>]/[<EmailAddress>]/[<StringLength(n)>]/[<MinLength(n)>]/[<MaxLength(n)>]/[<Range(lo,hi)>]/[<RegularExpression(..)>] and markers Phone/Url/CreditCard/Compare/...) are parsed off the preceding (and inline) [<...>] attribute lines and folded into terse comma-free chips on Properties["validations"] (Required, Email, MaxLength:120, Range:1..5, Pattern), which the dashboard ShapeTree renders. Mirrors the Java #4872 / TS #4858 / Python #4871 field-validation chips. Non-validation attributes ([<CLIMutable>]) are ignored. Proven by TestFSharp_DataAnnotationsValidations, TestFSharp_InlineValidationAttribute, TestFSharp_NonValidationAttributesIgnored. |
| Custom validator extraction | 🔴 `missing` | — | 5049 | — | [<CustomValidation(typeof<...>, "Method")>] and IValidatableObject.Validate custom validators are not yet linked. Deferred follow-up (Validus / FsToolkit.ErrorHandling Result/Validation pipelines also deferred). |

### Coercion

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Type coercion recognition | — `not_applicable` | `2026-06-13` | — | — | DataAnnotations validate, they do not coerce types; model binding/coercion is the HTTP framework's responsibility. |

### Testing

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Tests linkage | 🔴 `missing` | — | 5049 | — | No validation-specific test-linkage fixtures yet; general F# test-linkage substrate provides TESTS edges. Deferred follow-up. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.fsharp.validation.dataannotations ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```

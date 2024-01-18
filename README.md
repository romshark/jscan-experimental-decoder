# Experimental JSON decoder based on [jscan](github.com/romshark/jscan)

⚠️ Don't use in production! ⚠️

This is an experimental JSON Unmarshaler/Decoder implementation for Go based on
[jscan](github.com/romshark/jscan). This decoder will be moved to jscan v3 once it's
ready for production.

The jscan decoder is expected to be a backward compatible drop-in replacement for [encoding/json](https://pkg.go.dev/encoding/json).

Roadmap:
- [x] Primitive types
- [x] Struct types
- [x] Slices
- [x] Arrays
- [ ] JSON struct tags
    - [x] support for field names with case-insensitive matching.
    - [ ] option `string`
- [ ] Pointers
- [ ] `any`
- [ ] `map`
- [ ] `json.RawMessage`
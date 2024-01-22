<a href="https://goreportcard.com/report/github.com/romshark/jscan-experimental-decoder">
    <img src="https://goreportcard.com/badge/github.com/romshark/jscan-experimental-decoder" alt="GoReportCard">
</a>
<a href='https://coveralls.io/github/romshark/jscan-experimental-decoder/?branch=main'>
    <img src='https://coveralls.io/repos/github/romshark/jscan-experimental-decoder/badge.svg?branch=main' alt='Coverage Status' />
</a>

# Experimental JSON decoder based on [jscan](github.com/romshark/jscan)

⚠️ Don't use in production! ⚠️

This is an experimental JSON Unmarshaler/Decoder implementation for Go based on
[jscan](https://github.com/romshark/jscan). This decoder will be moved to jscan v3 once it's
ready for production.

The jscan decoder is expected to be a backward compatible drop-in replacement for [encoding/json](https://pkg.go.dev/encoding/json).

Roadmap:
- [x] Primitive types
- [x] Struct types
- [x] Slices
- [x] Arrays
- [ ] JSON struct tags
    - [x] Case-insensitive key matching.
    - [ ] Option `string`
- [ ] Pointers
- [ ] Type `any`
- [ ] Type `map`
- [ ] Type `Unmarshaler interface { UnmarshalJSON([]byte) error }`
- [ ] Type `json.RawMessage`

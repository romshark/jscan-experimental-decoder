<a href="https://goreportcard.com/report/github.com/romshark/jscan-experimental-decoder">
    <img src="https://goreportcard.com/badge/github.com/romshark/jscan-experimental-decoder" alt="GoReportCard">
</a>
<a href='https://coveralls.io/github/romshark/jscan-experimental-decoder/?branch=main'>
    <img src='https://coveralls.io/repos/github/romshark/jscan-experimental-decoder/badge.svg?branch=main' alt='Coverage Status' />
</a>

# Experimental JSON decoder based on [jscan](https://github.com/romshark/jscan)

⚠️ Don't use in production! ⚠️

This is an experimental JSON Unmarshaler/Decoder implementation for Go based on
[jscan](https://github.com/romshark/jscan). This decoder will be moved to jscan v3 once it's
ready for production.

The jscan decoder is expected to be a backward compatible drop-in replacement for [encoding/json](https://pkg.go.dev/encoding/json).

Roadmap:
- [x] Primitive types
- [x] Struct types
    - [x] Type `struct{}`
    - [ ] Recursive struct types
- [x] Slices
- [x] Arrays
- [x] Type `any`
- [x] Type `map`
    - [x] `string` keys
    - [x] `encoding.TextUnmarshaler` keys
    - [x] Integer keys
- [x] JSON struct tags
    - [x] Option `DisallowUnknownFields`
    - [x] Case-insensitive key matching
    - [x] Option `string`
- [x] Pointers
- [x] Type `Unmarshaler interface { UnmarshalJSON([]byte) error }`
- [x] Type `TextUnmarshaler interface { UnmarshalText(text []byte) error }`
- [ ] `encoding/json` compatible drop-in replacement package `jscandec/std`

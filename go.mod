module github.com/romshark/jscan-experimental-decoder

go 1.21.6

// Use branch "tokenizer" which isn't merged yet.
replace github.com/romshark/jscan/v2 => github.com/romshark/jscan/v2 v2.0.0-20240125233239-f1e817ac892f

require (
	github.com/go-json-experiment/json v0.0.0-20231102232822-2e55bd4e08b0
	github.com/goccy/go-json v0.10.2
	github.com/json-iterator/go v1.1.12
	github.com/mailru/easyjson v0.7.7
	github.com/pquerna/ffjson v0.0.0-20190930134022-aa0246cd15f7
	github.com/romshark/jscan/v2 v2.0.2
	github.com/stretchr/testify v1.8.4
	github.com/tidwall/gjson v1.17.0
	github.com/valyala/fastjson v1.6.4
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180228061459-e0a39a4cb421 // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

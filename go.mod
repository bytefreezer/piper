module github.com/n0needt0/bytefreezer-piper

go 1.23.0

toolchain go1.24.4

require (
	github.com/Cistern/sflow v0.0.0-20240622235316-ed105e3cf9fb
	github.com/aws/aws-sdk-go v1.55.8
	github.com/aws/aws-sdk-go-v2 v1.39.0
	github.com/aws/aws-sdk-go-v2/config v1.31.7
	github.com/aws/aws-sdk-go-v2/credentials v1.18.11
	github.com/aws/aws-sdk-go-v2/service/s3 v1.88.0
	github.com/bytedance/sonic v1.14.1
	github.com/google/uuid v1.6.0
	github.com/knadh/koanf/parsers/yaml v1.1.0
	github.com/knadh/koanf/providers/env v1.1.0
	github.com/knadh/koanf/providers/file v1.2.0
	github.com/knadh/koanf/providers/rawbytes v1.0.0
	github.com/knadh/koanf/v2 v2.2.2
	github.com/lib/pq v1.10.9
	github.com/n0needt0/go-goodies/log v0.0.0-20250911153747-5be7cbbfc35a
	github.com/swaggest/openapi-go v0.2.60
	github.com/swaggest/rest v0.2.75
	github.com/swaggest/swgui v1.8.4
	github.com/swaggest/usecase v1.3.1
	go.opentelemetry.io/otel/metric v1.38.0
	go.uber.org/zap v1.27.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.1 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.7 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.7 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.7 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.8.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.29.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.34.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.38.3 // indirect
	github.com/aws/smithy-go v1.23.0 // indirect
	github.com/bytedance/gopkg v0.1.3 // indirect
	github.com/bytedance/sonic/loader v0.3.0 // indirect
	github.com/cloudwego/base64x v0.1.6 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-chi/chi/v5 v5.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.9 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/oschwald/geoip2-golang v1.13.0 // indirect
	github.com/oschwald/maxminddb-golang v1.13.0 // indirect
	github.com/santhosh-tekuri/jsonschema/v3 v3.1.0 // indirect
	github.com/swaggest/form/v5 v5.1.1 // indirect
	github.com/swaggest/jsonschema-go v0.3.78 // indirect
	github.com/swaggest/refl v1.4.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ua-parser/uap-go v0.0.0-20250917011043-9c86a9b0f8f0 // indirect
	github.com/vearutop/statigz v1.4.0 // indirect
	go.opentelemetry.io/otel v1.38.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	go.yaml.in/yaml/v3 v3.0.3 // indirect
	golang.org/x/arch v0.0.0-20210923205945-b76863e36670 // indirect
	golang.org/x/sys v0.35.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

replace github.com/n0needt0/bytefreezer-control => ../bytefreezer-control

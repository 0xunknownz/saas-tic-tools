module saas-tic-tools/apps/api

go 1.26.0

require (
	github.com/0xunknownz/saas.starter v0.2.1
	github.com/PaddlePaddle/PaddleOCR/api_sdk/go v0.2.0
	github.com/aliyun/alibabacloud-oss-go-sdk-v2 v1.6.1
	github.com/crewjam/saml v0.4.14
	github.com/jackc/pgx/v5 v5.7.2
	github.com/pdfcpu/pdfcpu v0.11.0
	golang.org/x/crypto v0.38.0
)

require (
	github.com/beevik/etree v1.1.0 // indirect
	github.com/crewjam/httperr v0.2.0 // indirect
	github.com/golang-jwt/jwt/v4 v4.4.3 // indirect
	github.com/hhrutter/lzw v1.0.0 // indirect
	github.com/hhrutter/pkcs7 v0.2.0 // indirect
	github.com/hhrutter/tiff v1.0.2 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jonboulle/clockwork v0.2.2 // indirect
	github.com/mattermost/xml-roundtrip-validator v0.1.0 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/russellhaering/goxmldsig v1.3.0 // indirect
	golang.org/x/image v0.27.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	golang.org/x/time v0.7.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

// One version pins the CLI and the Foundation (docs/foundation-modularization.md 5); run it as
// `go tool saasctl` so the CLI always matches the module this Product compiles.
tool github.com/0xunknownz/saas.starter/cmd/saasctl

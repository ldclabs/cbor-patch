module github.com/ldclabs/cbor-patch

go 1.16

require (
	github.com/fxamacker/cbor/v2 v2.4.0
	github.com/stretchr/testify v1.8.0
)

replace github.com/fxamacker/cbor/v2 v2.4.0 => github.com/ldclabs/cbor/v2 v2.5.0-stg2

module github.com/cyberark/secrets-provider-for-k8s

go 1.19

require (
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/stretchr/testify v1.8.3
	go.opentelemetry.io/otel v1.16.0
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/api v0.27.2
	k8s.io/apimachinery v0.27.2
	k8s.io/client-go v0.27.2
	sigs.k8s.io/e2e-framework v0.2.0
)

require (
	github.com/alessio/shellescape v1.4.1 // indirect
	github.com/danieljoos/wincred v1.1.2 // indirect
	github.com/emicklei/go-restful/v3 v3.9.0 // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.1 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/google/gnostic v0.5.7-v3refs // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/zalando/go-keyring v0.2.3-0.20230503081219-17db2e5354bd // indirect
	go.opentelemetry.io/otel/metric v1.16.0 // indirect
	sigs.k8s.io/controller-runtime v0.14.5 // indirect
)

require (
	github.com/bgentry/go-netrc v0.0.0-20140422174119-9fd32a8b3d3d // indirect
	github.com/cyberark/conjur-api-go v0.10.1 // version will be ignored by auto release process
	github.com/cyberark/conjur-authn-k8s-client v0.26.0 // version will be ignored by auto release process
	github.com/cyberark/conjur-opentelemetry-tracer v1.55.55 // version will be ignored by auto release process
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fullsailor/pkcs7 v0.0.0-20190404230743-d7302db945fa // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	go.opentelemetry.io/otel/exporters/jaeger v1.7.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.7.0 // indirect
	go.opentelemetry.io/otel/sdk v1.7.0 // indirect
	go.opentelemetry.io/otel/trace v1.16.0 // indirect
	golang.org/x/net v0.8.0 // indirect
	golang.org/x/oauth2 v0.0.0-20220223155221-ee480838109b // indirect
	golang.org/x/sys v0.8.0 // indirect
	golang.org/x/term v0.6.0 // indirect
	golang.org/x/text v0.8.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.29.1 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/klog/v2 v2.90.1 // indirect
	k8s.io/kube-openapi v0.0.0-20230501164219-8b0f38b5fd1f // indirect
	k8s.io/utils v0.0.0-20230209194617-a36077c30491 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

// Automated release process replaces
// DO NOT EDIT: CHANGES TO THE 3 BELOW LINES WILL BREAK AUTOMATED RELEASES
replace github.com/cyberark/conjur-api-go => github.com/cyberark/conjur-api-go latest

replace github.com/cyberark/conjur-authn-k8s-client => github.com/cyberark/conjur-authn-k8s-client latest

replace github.com/cyberark/conjur-opentelemetry-tracer => github.com/cyberark/conjur-opentelemetry-tracer latest

// Security fixes to ensure we don't have old vulnerable packages in our
// dependency tree.  Only put specific versions on the left side of the =>
// so we don't downgrade future versions unintentionally.

exclude github.com/emicklei/go-restful v2.9.5+incompatible

replace github.com/emicklei/go-restful/v3 v3.9.0 => github.com/emicklei/go-restful/v3 v3.10.2

replace golang.org/x/crypto v0.0.0-20190308221718-c2843e01d9a2 => golang.org/x/crypto v0.0.0-20220525230936-793ad666bf5e

replace golang.org/x/crypto v0.0.0-20190510104115-cbcb75029529 => golang.org/x/crypto v0.0.0-20220525230936-793ad666bf5e

replace golang.org/x/crypto v0.0.0-20190605123033-f99c8df09eb5 => golang.org/x/crypto v0.0.0-20220525230936-793ad666bf5e

replace golang.org/x/crypto v0.0.0-20191011191535-87dc89f01550 => golang.org/x/crypto v0.0.0-20220525230936-793ad666bf5e

replace golang.org/x/crypto v0.0.0-20201002170205-7f63de1d35b0 => golang.org/x/crypto v0.0.0-20220525230936-793ad666bf5e

replace golang.org/x/crypto v0.0.0-20210921155107-089bfa567519 => golang.org/x/crypto v0.0.0-20220525230936-793ad666bf5e

replace golang.org/x/crypto v0.0.0-20211215153901-e495a2d5b3d3 => golang.org/x/crypto v0.0.0-20220525230936-793ad666bf5e

replace golang.org/x/crypto v0.0.0-20220214200702-86341886e292 => golang.org/x/crypto v0.0.0-20220525230936-793ad666bf5e

replace golang.org/x/net v0.0.0-20180826012351-8a410e7b638d => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20180724234803-3673e40ba225 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20180906233101-161cd47e91fd => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20190108225652-1e06a53dbb7e => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20190213061140-3a22650c66bd => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20190311183353-d8887717615a => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20190404232315-eb5bcb51f2a3 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20190501004415-9ce7a6920f09 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20190503192946-f4e77d36d62c => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20190603091049-60506f45cf65 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20190620200207-3b0461eec859 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20190628185345-da137c7871d7 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20190724013045-ca1201d0de80 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20190827160401-ba9fcec4b297 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20191209160850-c0dbc17a3553 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20200114155413-6afb5195e5aa => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20200202094626-16171245cfb2 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20200222125558-5a598a2470a0 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20200226121028-0de0cce0169b => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20200301022130-244492dfa37a => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20200324143707-d3edc9973b7e => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20200501053045-e0ff5e5a1de5 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20200506145744-7e3656a0809f => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20200513185701-a91f0712d120 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20200520004742-59133d7f0dd7 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20200520182314-0ba52f642ac2 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20200625001655-4c5254603344 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20200707034311-ab3426394381 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20200822124328-c89045814202 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20201021035429-f5854403a974 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20201031054903-ff519b6c9102 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20201110031124-69a78807bb2b => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20201209123823-ac852fbbde11 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20210119194325-5f4716e94777 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20210226172049-e18ecbb05110 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20210316092652-d523dce5a7f4 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20210405180319-a5a99cb37ef4 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20210428140749-89ef3d95e781 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20210503060351-7fd8e65b6420 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20211015210444-4f30a5c0130f => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20211112202133-69e39bad7dc2 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20220127200216-cd36cc0744dd => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20220923203811-8be639271d50 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20220225172249-27dd8689420f => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20220722155237-a158d28d115b => golang.org/x/net v0.7.0

replace golang.org/x/net v0.1.1-0.20221104162952-702349b0e862 => golang.org/x/net v0.7.0

replace golang.org/x/text v0.0.0-20170915032832-14c0d48ead0c => golang.org/x/text v0.4.0

replace golang.org/x/net v0.4.0 => golang.org/x/net v0.7.0

replace golang.org/x/net v0.0.0-20220425223048-2871e0cb64e4 => golang.org/x/net v0.7.0

replace golang.org/x/text v0.3.0 => golang.org/x/text v0.4.0

replace golang.org/x/text v0.3.1-0.20180807135948-17ff2d5776d2 => golang.org/x/text v0.4.0

replace golang.org/x/text v0.3.2 => golang.org/x/text v0.4.0

replace golang.org/x/text v0.3.3 => golang.org/x/text v0.4.0

replace golang.org/x/text v0.3.4 => golang.org/x/text v0.4.0

replace golang.org/x/text v0.3.5 => golang.org/x/text v0.4.0

replace golang.org/x/text v0.3.6 => golang.org/x/text v0.4.0

replace gopkg.in/yaml.v2 v2.2.1 => gopkg.in/yaml.v2 v2.2.8

replace gopkg.in/yaml.v2 v2.2.2 => gopkg.in/yaml.v2 v2.2.8

replace gopkg.in/yaml.v2 v2.2.3 => gopkg.in/yaml.v2 v2.2.8

replace gopkg.in/yaml.v2 v2.2.4 => gopkg.in/yaml.v2 v2.2.8

replace gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c => gopkg.in/yaml.v3 v3.0.1

replace gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 => gopkg.in/yaml.v3 v3.0.1

replace gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b => gopkg.in/yaml.v3 v3.0.1

replace google.golang.org/protobuf v0.0.0-20200109180630-ec00e32a8dfd => google.golang.org/protobuf v1.29.1

replace google.golang.org/protobuf v0.0.0-20200221191635-4d8936d0db64 => google.golang.org/protobuf v1.29.1

replace google.golang.org/protobuf v0.0.0-20200228230310-ab0ca4ff8a60 => google.golang.org/protobuf v1.29.1

replace google.golang.org/protobuf v1.20.1-0.20200309200217-e05f789c0967 => google.golang.org/protobuf v1.29.1

replace google.golang.org/protobuf v1.21.0 => google.golang.org/protobuf v1.29.1

replace google.golang.org/protobuf v1.22.0 => google.golang.org/protobuf v1.29.1

replace google.golang.org/protobuf v1.23.0 => google.golang.org/protobuf v1.29.1

replace google.golang.org/protobuf v1.23.1-0.20200526195155-81db48ad09cc => google.golang.org/protobuf v1.29.1

replace google.golang.org/protobuf v1.24.0 => google.golang.org/protobuf v1.29.1

replace google.golang.org/protobuf v1.25.0 => google.golang.org/protobuf v1.29.1

replace google.golang.org/protobuf v1.26.0-rc.1 => google.golang.org/protobuf v1.29.1

replace google.golang.org/protobuf v1.26.0 => google.golang.org/protobuf v1.29.1

// Resolves CVE-2022-1996 until k8s.io/client-go v0.25.0+ is released
replace k8s.io/kube-openapi v0.0.0-20220328201542-3ee0da9b0b42 => k8s.io/kube-openapi v0.0.0-20220627174259-011e075b9cb8

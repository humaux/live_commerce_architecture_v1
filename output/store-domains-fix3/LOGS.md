# store-domains fix3 — gate logs

Run against the disposable PG 18.6 container (task-owned, `test-focused.sh`), `-race -count=1`.

## Static gates
```
$ go build ./... && go vet ./...          # exit 0
$ gofmt -l internal/httpapi/storefront.go internal/storefrontdomains/service.go \
      internal/storefrontdomains/service_test.go tests/foundation/store_domains_test.go
                                          # clean (no output)
$ bash scripts/dev/check-gates.sh
check-gates: ok (56 modes, all documented; every tracked test file is run)
$ bash scripts/dev/depmap.sh --check
depmap: up to date
```

## Unit tests
```
$ go test ./internal/storefrontdomains/ ./internal/httpapi/
ok  	livecommerce/internal/storefrontdomains	1.325s
ok  	livecommerce/internal/httpapi	0.464s
```

## Focused foundation gate
```
$ bash scripts/dev/test-focused.sh '^(TestStoreDomains|TestStorefrontPublish|TestT06WorkerAuthorityAndFunctionACL)'
...
ok  	livecommerce/tests/foundation	13.365s
test-focused: top-level PASS=31 FAIL=0 SKIP=0 exit=0
```

31 passing top-level tests: T06WorkerAuthorityAndFunctionACL, SDW01–SDW15, BackfillsPre0106Stores,
DeployPassesBaseDomainToMigrate, the three new gates (Idempotency / MerchantViewKind / ApexEdgeInstructions),
and SPW01–SPW10.

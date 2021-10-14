mkdir -p ./testdata
rm -f ./testdata/*

CONJUR_SSL_CERTIFICATE="-----BEGIN CERTIFICATE-----
MIIDizCCAnOgAwIBAgIUeU9wyM/LD/MEm3nJc9/S0PDpPmUwDQYJKoZIhvcNAQEL
BQAwSDELMAkGA1UEBhMCVVMxCzAJBgNVBAgTAkNBMRYwFAYDVQQHEw1TYW4gRnJh
bmNpc2NvMRQwEgYDVQQDEwt0ZXN0LXNlcnZlcjAgFw0yMDA2MTYwOTUzMDBaGA8y
MTM0MDcxNjAwNTMwMFowETEPMA0GA1UEAxMGc2VydmVyMIIBIjANBgkqhkiG9w0B
AQEFAAOCAQ8AMIIBCgKCAQEAx2QlaVnpgjFsBFY/NtzWoeVPz5hJz+5MGkPoFdVs
GncroYvZsTAMl56/GA48TYdtCe+vA9GRXR5ns89cCmSjbuV2/sdyOpBDRei+ghHu
tQFoAoVbgva7Ic7Y8/jBwN0fX9O1XkN0pp2FsAj4GTSztydfHMdjY/jbJIbgTrx0
5RWaU1928GVANO3xInsaYYPMWjiYM4Mry++FSOAbx5+jPs2bfkKFtmipS415r/oF
zw+UdZ9E9oJDDEEsxYcoAxgNcLzrl9n57J0N5GB3FGyMg8lulcqzHFN6ueHd6lXi
BmmlIr2bqkOjkP/yv8jjf2POyOx/K4IwqqgSPyGxNpOlZQIDAQABo4GhMIGeMA4G
A1UdDwEB/wQEAwIFoDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwDAYD
VR0TAQH/BAIwADAdBgNVHQ4EFgQUshD/QApOyMGDCG+S5Wpvvkhs2BgwHwYDVR0j
BBgwFoAUnDihVk/UP0pAeIHnmACsWo27gUQwHwYDVR0RBBgwFoIJbG9jYWxob3N0
ggVteXNxbIICcGcwDQYJKoZIhvcNAQELBQADggEBAH9F+kw/DTnFl7Dylu5osJER
NxNuSWTB8Q0zhHIef3HesD+YIpPcihKqeUvlS1zU/YSTKp0a+oMLzuTWeXrK7kaD
iYNUywuW0XZ0lXFinilSsMUI6y08jNJGThpGEUdVOdSYhz9XtKf1CKWe/Bq2KIq+
nOqXQEge5R8zgmB9sNHecQ9L6d5V/p4g4A+Jz4etK2uYiSYvEKSwlqzADWZCjYIh
DwKcZmkBsZ4qQhe72zIMyWuYOCHB4JE8CvnPrwVnqBQfjSGO+rWUtveI0den/LRW
FI2qTPWpwVnXnhx70KfqTIElo+cc+Lit6wKpUgiMxIy/P3SvpNbXiK9dopylgdM=
-----END CERTIFICATE-----" \
CONJUR_AUTHN_URL=http://127.0.0.1:55643 \
CONJUR_ACCOUNT=conjuraccount \
CONJUR_AUTHN_LOGIN=host/conjurlogin \
MY_POD_NAMESPACE=podnamespace \
MY_POD_NAME=podname \
SECRETS_DESTINATION=k8s_secrets \
K8S_SECRETS=secretwithconjurmap \
DEBUG=true \
  go run ./main.go \
    -test=true \
    -f ./example-annotations \
    "$@"

# store type="file"
# Calling "./run.sh" as is will use "-f ./example-annotations" as above and will result in
# the provider running with store type "file" as per the annotations which override
# environment variables

# store type="k8s_secrets"
# Calling "./run.sh -f noexists" will use "k8s_secrets" because there are no annotations to
# override the environment variables

# Non-test mode
# Add "-test=false" to "./run.sh" to run in production using production dependencies. It
# will likely fail because the dependencies can not be configured.

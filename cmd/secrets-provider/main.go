package main

import (
	"github.com/cyberark/secrets-provider-for-k8s/pkg/entrypoint"
)

func main() {
	entrypoint.StartSecretsProvider()
}

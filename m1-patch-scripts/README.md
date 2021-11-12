# Scripts to run M1 E2E Demo

The scripts that are in this `m1-e2e-demo/` subdirectory are
intended to be used with the `m1-e2e-demo` branch of the
`cyberark/conjur-authn-k8s-client` GitHub repository.

## Setting up a Demo Environment

To set up a demo environment:

```
# Clone the 'cyberark/conjur-authn-k8s-client' repo, if you haven't already.
cd
mkdir -p cyberark
cd cyberark
git clone https://github.com/cyberark/conjur-authn-k8s-client

# Check out the 'm1-e2e-demo' branch
cd conjur-authn-k8s-client
git checkout m1-e2e-demo

# Run the M1 E2E workflow scripts
cd bin/test-workflow
./start -n -a secrets-provider-init
```

## Running Demo Scripts

After a demo environment has been set up, run any of the
`apply-m1-patch\*` scripts in the `my-patch-scripts/` directory, and
the script will use `kubectl patch ...` to add Annotations to the
demo application deployment, and then show the resulting secrets files
that are generated in the application Pod's shared volume.

## Cleaning up Demo Environment

To clean up the demo environment, run:

```
kubectl delete namespace app-test
kubectl delete namespace conjur-oss
```

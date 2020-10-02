# Solution Design - Secrets Provider Release

## Table of Contents

[//]: # "You can use this tool to generate a TOC - https://ecotrust-canada.github.io/markdown-toc/"

## Glossary

[//]: # "Describe terms that will be used throughout the design"
[//]: # "You can use this tool to generate a table - https://www.tablesgenerator.com/markdown_tables#"

| **Term**                                                     | **Description**                                              |
| ------------------------------------------------------------ | ------------------------------------------------------------ |
| [Github actions](https://docs.github.com/en/free-pro-team@latest/actions) | CI/CD platform that is event-driven to help automate task and execute workflows in repositories |
| [repostiory_dispatch](https://docs.github.com/en/free-pro-team@latest/actions/reference/events-that-trigger-workflows)  webhook event | a webhook event used when you want to trigger a workflow for activity that happens outside of Github |

## Useful links

[//]: # "Add links that may be useful for the reader"

Release documentation

- [Secrets Provider release documentation](https://github.com/cyberark/secrets-provider-for-k8s/blob/master/CONTRIBUTING.md#releases)

Open issues:

- [Secrets Provider - Release automation](https://app.zenhub.com/workspaces/palmtree-5d99d900491c060001c85cba/issues/cyberark/secrets-provider-for-k8s/233)

- [Secrets Provider - Github action for dispatching events](https://github.com/cyberark/secrets-provider-for-k8s/issues/239)

- [Community - Guidelines and tools for releasing artifacts to separate suite components](https://github.com/cyberark/community/issues/77)

PR:

- https://github.com/cyberark/secrets-provider-for-k8s/pulls

## Background

[//]: # "Give relevant background for the designed feature. What is the motivation for this solution?"

The Secrets Provider release process has become a long and heavily manual process. This has become more evident with the introduction of Helm to the project which now adds additional steps to the release process. 

Goals for this effort:

1. Automate the parts we can.
2. Knowledge sharing by providing guidelines for event-driven actions across multiple repositories.
3. Push 'edge' release on every master build for easier testing and to better align ourselves with other open source projects

## Solution

[//]: # "Elaborate on the solution you are suggesting in this page. Address the functional requirements and the non functional requirements that this solution is addressing. If there are a few options considered for the solution, mention them and explain why the actual solution was chosen over them. Add an execution plan when relevant. It doesn't have to be a full breakdown of the feature, but just a recommendation to how the solution should be approached."

As part of our Secrets Provider for K8s [release process](https://github.com/cyberark/secrets-provider-for-k8s/blob/master/CONTRIBUTING.md#releases) we need to manually package our Secrets Provider Helm Chart and open a PR attaching the artifact in our [cyberark/helm-charts](cyberark/helm-charts) repository. This process can be automated by introducing Github actions into our workflows and will be addressed in the below solution.

The solution will be twofold and impact both the `cyberark/secrets-provider-for-k8s` Jenkins pipeline and `cyberark/helm-chart` repository. The flow at a high-level is as follows:

`cyberark/secrets-provider-for-k8s` Jenkins pipeline

1. Package the Secrets Provider Helm chart as part of Jenkins pipeline
2. Dispatch a `helm-version-release` event by sending a curl request with Helm package as part of request

`cyberark/helm-charts` repository

1. Listen on `helm-version-release` events

2. Once received, begin Github workflow by:

   a. Creating and checking out a new branch

   b. Commiting the Helm package to the branch

   c. Creating a new PR and assigning reviewers

### Additions to `cyberark/secrets-provider-for-k8s` Jenkins pipeline

To handle the packaging the Helm Chart, we will [add a "Package artifacts" stage](https://github.com/cyberark/secrets-provider-for-k8s/pull/234/files#diff-58231b16fdee45a03a4ee3cf94a9f2c3R109) to the Jenkins pipeline that will run on *all* builds. This stage will be responsible for packaging the Helm Chart and saving the artifact to the `Artifacts` tabs of the build. An additional step will run on version builds (v1.1.0 for example) that will push those artifacts to `cyberark/helm-charts` repository. 

The change resembles the following:

```yaml
stage('Package artifacts') {
   steps {
      sh 'ci/jenkins_build'
        
      archiveArtifacts artifacts: "helm-artifacts/", fingerprint: false, allowEmptyArchive: true
	 }
   when { tag "v*" }
   steps {
     sh 'ci/dispatch_version_release_event'
   } 
}
```

The  `ci/jenkins_build` script spins up a container and packages the Helm Chart like so:

```bash
source bin/build_utils
helm_version=3.3.0

docker run --rm \
  -v $PWD/helm/secrets-provider:/root/helm/secrets-provider \
  -v $PWD/helm-artifacts/:/root/helm-artifacts \
  --workdir /root/helm-artifacts \
  alpine/helm:${helm_version} package ../helm/secrets-provider
```

 The `ci/dispatch_version_release_event` script will only run on release builds. A curl request will be sent passing in the packaged Helm chart, dispatching a `helm-version-release` event. This will trigger a workflow in `cyberark/helm-charts` to run.

```bash
curl \
	-X POST \
	-H "Authorization: Bearer 677...13c" \
	-H "Accept: application/vnd.github.v3+json" \
	https://api.github.com/repos/:org/:repo/dispatches \
	-d '{"event_type": "helm-version-release"}'
```

### Additions to `cyberark/helm-chart` Github repository

In the `cyberark/helm-chart` repository we will need to create a Github workflow that listens for `helm-version-release` events. Once received this will trigger a series of actions in response. The following is an incomplete Github workflow that will be added to the `cyberark/helm-chart` repository:

```pseudocode
name: Push Helm package workflow

on:
  repository_dispatch:
    types: [version-release]
    
jobs:
		push-helm-package:
      runs-on: ubuntu-latest
      steps:
        - name: checkout-branch
	        uses: peterjgrainger/action-create-branch@v1.0.0
          run: | 
          	echo "Checking out branch 'version-release'"
        		
        		...

       - name: pull-request-action
          uses: vsoch/pull-request-action@1.0.6
          env:
            GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
            BRANCH_PREFIX: "release-/"
            PULL_REQUEST_FROM_BRANCH: "release-v110"
            PULL_REQUEST_BRANCH: master
            PULL_REQUEST_TITLE: "Push Helm chart package for new version release"
            PULL_REQUEST_ASSIGNEES: sigalsax     	
```

### Push `edge` release on successful master build

***Completed***

Another key addition part of the release automation effort is pushing an `edge` release to Dockerhub on every green master build. `edge` releases are intended for frequent pushes of content and used for closely monitoring development. This tag should not be given to our customer's to use in production. 

This tagged release will allow us to better align ourselves with other open source projects and be used when we want to test our feature without having to manually build the image or wait for a release. 

### Affected Components

[//]: # "Address the components that will be affected by your solution [Conjur, DAP, clients, integrations, etc.]"

- `cyberark/secrets-provider-for-k8s`
  - Add a stage in the Jenkins pipeline to send curl request as part of Helm packaging script (`ci/jenkins_build`) to dispatch event.
  - The Helm chart JSON schema only accepts `latest` and numbered tags to install our Helm chart. We will need to update this to include `edge` tags. 
- `cyberark/helm-charts` - a Github action will be added to the repository that will be triggered by a version build in the Secrets Provider for K8s Jenkins pipeline.

## Obstacles/Limitations

- There might not be a Github Action for accepting files 

  Possible solution(s)

  - [Github action](https://www.google.com/search?q=github+action+ssh&oq=github+action+ssh&aqs=chrome..69i57j0l6j69i60.1471j0j7&sourceid=chrome&ie=UTF-8) for SSH into Jenkins machine but may not have privileges
  - [Github action](https://github.com/appleboy/jenkins-action) for triggering a Jenkins job and package the Helm chart but still may not have access to artifact
  - Have the workflow package the Helm chart

## Future developments

1. On a green master build after the merge of a *specific* version bump branch, launch an workflow to create a draft release where the description of the release is the latest changelog entry and the binaries are the Helm chart and tar of the image.
2. Use goreleaser to auto-attach files to GH releases and have the helm-chart workflow fetch from there

## Documentation

[//]: # "Add notes on what should be documented in this solution. Elaborate on where this should be documented. If the change is in open-source projects, we may need to update the docs in github too. If it's in Conjur and/or DAP mention which products are affected by it"

We will need to update our release documentation, adding the new `edge` release and will need to provide guidelines on how to use Github actions.

## Open questions

[//]: # "Add any question that is still open. It makes it easier for the reader to have the open questions accumulated here istead of them being acattered along the doc"

1. How to best to pass packaged Helm chart from Jenkins pipeline to another repository?

## Implementation plan

[//]: # "Break the solution into tasks"

- Ramp-up: Github actions ***1 days***
- Github actions in `cyberark/helm-charts` repository and add Jenkins stage in `cyberark/secrets-provider-for-k8s` ***5 days***
- Update documentation ***1 day***
- Create document detailing how event-driven actions across multiple repositories ***2 days***

***Total*** 9 days

Possible challenges that can delay project completion

- Cross-project dependency with:

  Community and Integrations team because we will be introducing a Github action in the `cyberark/helm-charts` repository.

  Infra team for passing `.tgz` file from our pipeline to `cyberark/helm-chart` which might not be possible

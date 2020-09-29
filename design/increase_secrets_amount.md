# Solution Design - Increase amount of secrets supported by Secrets Provider

## Table of Contents
[//]: # "You can use this tool to generate a TOC - https://ecotrust-canada.github.io/markdown-toc/"

## Glossary
[//]: # "Describe terms that will be used throughout the design"
[//]: # "You can use this tool to generate a table - https://www.tablesgenerator.com/markdown_tables#"

| **Term** | **Description** |
|----------|-----------------|
|          |                 |
|          |                 |

## Useful links
[//]: # "Add links that may be useful for the reader"
Epic - https://app.zenhub.com/workspaces/palmtree-5d99d900491c060001c85cba/issues/cyberark/secrets-provider-for-k8s/236

## Background
[//]: # "Give relevant background for the designed feature. What is the motivation for this solution?"

We would like to increase the amount of secrets the Secrets Provider for K8s can accept and deliever to our customers.

## Issue description

[//]: # "Elaborate on the issue you are writing a solution for"

Our current Batch API endpoint accepts secret variable ids in the URI in the form of `GET /secrets{?variable_ids=myaccount:variable:secret1,myaccount:variable:secret2}`. There are two problems with the current approach.

1. We are limited to the maximum URI length of our server which is problematic for customers with long secret paths.
2. For each `variableID` we attach the account name to the query so this consumes additional space.

## Solution

[//]: # "Elaborate on the solution you are suggesting in this page. Address the functional requirements and the non functional requirements that this solution is addressing. If there are a few options considered for the solution, mention them and explain why the actual solution was chosen over them. Add an execution plan when relevant. It doesn't have to be a full breakdown of the feature, but just a recommendation to how the solution should be approached."

There are three possible solutions for solving the above problems

1. Increase the `MAX_URI_LENGTH` to 10240 in the Conjur WEBrick server. See [here](https://stackoverflow.com/questions/4926740/omniauth-google-openid-webrickhttpstatusrequesturitoolarge/15261476#15261476) for a more detailed explanation. 
2. Remove the account from all the `variableID` and pass them in as part of URI `secrets/account?variable_ids=`
3. Create a [new Batch API endpoint](/batch_retrieval_design.md) and consume variableIDs in the body of the request. 

### Increase `MAX_URI_LENGTH`

Ruby WEBrick server [increased the maximum URL length](https://github.com/ruby/ruby/blob/master/lib/webrick/httprequest.rb#L446) to 2083 characters from the previous 1024  but this may not be enough. We manually increase the `MAX_URI_LENGTH` in our `httprequest.rb` to a number of our choosing.

### Create new Batch API endpoint

The current API endpoint takes all `variableID`s in the URI of  Current API has a fail fast approach. If one of the secrets failed to be retrieved, no secrets will be delievered to applications.

### Remove account from URI

Pros

Cons

### Design
[//]: # "Add any diagrams, charts and explanations about the design aspect of the solution. Elaborate also about the expected user experience for the feature"

### Backwards compatibility
[//]: # "Address how you are going to handle backwards compatibility, if necessary"

### Performance
[//]: # "Elaborate on whether your solution will affect the product's performance, and how"

### Affected Components
[//]: # "Address the components that will be affected by your solution [Conjur, DAP, clients, integrations, etc.]"

## Security
[//]: # "Are there any security issues with your solution and how do you plan to mitigate them? Even if you mentioned them somewhere in the doc it may be convenient for the security architect review to have them centralized here"

## Performance 

## Test Plan

[//]: # "Fill in the table below to depict the tests that should run to validate your solution"
[//]: # "You can use this tool to generate a table - https://www.tablesgenerator.com/markdown_tables#"

| **Title** | **Given** | **When** | **Then** | **Comment** |
|-----------|-----------|----------|----------|-------------|
|           |           |          |          |             |
|           |           |          |          |             |

## Logs

[//]: # "Fill in the table below to depict the log messages that can enhance the supportability of your solution"
[//]: # "You can use this tool to generate a table - https://www.tablesgenerator.com/markdown_tables#"

| Scenario | Log message | **Log level** |
| -------- | ----------- | ------------- |
|          |             |               |
|          |             |               |



### Audit 
[//]: # "Does this solution require additional audit messages?"

## Documentation
[//]: # "Add notes on what should be documented in this solution. Elaborate on where this should be documented. If the change is in open-source projects, we may need to update the docs in github too. If it's in Conjur and/or DAP mention which products are affected by it"

## Open questions
[//]: # "Add any question that is still open. It makes it easier for the reader to have the open questions accumulated here istead of them being acattered along the doc"

## Implementation plan
[//]: # "Break the solution into tasks"

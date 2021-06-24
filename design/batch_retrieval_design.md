# Batch retrieval design

+ [Useful Links](#useful-links)
+ [Motivation](#motivation)
+ [Design](#design)
+ [URI](#uri)
+ [Request Body](#request-body)
  - [Headers](#headers)
+ [Response](#response)
+ [Proposed API vs Current API](#proposed-api-vs-current-api)
+ [Permissions](#permissions)
+ [Open questions](#open-questions)

This document will cover the options for enhancing Batch Retrieval requests.



### Useful Links

| Name                                             | Link                                                         |
| ------------------------------------------------ | ------------------------------------------------------------ |
| Secrets Provider Job Milestone - Solution Design | https://github.com/cyberark/secrets-provider-for-k8s/blob/master/design/milestone_1_2_design_doc.md |
| Secrets Provider - Feature Doc                   | https://app.zenhub.com/workspaces/palmtree-5d99d900491c060001c85cba/issues/cyberark/secrets-provider-for-k8s/163 |

### References

- https://docs.microsoft.com/en-us/graph/json-batching#first-json-batch-request
- https://docs.microsoft.com/en-us/graph/json-batching#response-format

### Motivation

*Previously*, for the init container solution, each Secrets Provider was deployed to the same Pod as the customer application containers. Because of this, when a batch retrieval request was made, the requests were dispersed across many call to the Conjur server. When making the batch retrieval request and a permission error occurred for example, then the batch request would fail and only that Pod would not spin up successfully. 

*Now*, with the Job Milestone, the Secrets Provider is deployed outside of the customers' Pods, supporting multiple customer pods. With this, a single batch retrieval request will be made for *all* applications to fetch the necessary secrets to deploy properly. Therefore, if there is a failure in receiving one of the values, then the whole request will fail. In other words, **no application** Pod will spin up in the namespace. This leads to a faulty user experience.

The following table outlines possible solutions to overcome this:

|      | Solution                                                     | Pros                                                         | Cons                                               | Effort (T-shirt size) |
| ---- | ------------------------------------------------------------ | ------------------------------------------------------------ | -------------------------------------------------- | --------------------- |
| 1    | *Client side:* <br />For each K8s Secret, perform a new Batch request | - No server side changes                                     | Load on server with extra calls (*)                | S, 5 days             |
| 2    | *Server side:*<br />Update Batch retrieval to return list of variables **and** their individual responses (success/failure) (**) | - Best practice / straightforward design for batch endpoint<br />- Will help us during rotation for Milestone 2<br /> | - Requires both client/server implementation<br /> | M, 10 days            |
| 3    | Stay as is                                                   | No additional work needed                                    | - Bad UX<br />                                     | -                     |

(*) Fallback solution: use this solution as a safety net, only if the original Batch Retrieval request fails

(**) This solution can be broken up in two: 1. Create a new API endpoint or 2. use existing one (return 409 response)

*Decision:* We decided to go with #2 and create our own API. We decided to create a new batch retrieval API because of the following reasons:

1. Adding onto the existing batch endpoint would lead to changing behaviors for our needs in the Secrets Provider which would have cascading impacts on other teams. 
2. The two APIs (current and new) offer separate use-cases;  partial success (new) and fail fast (current)
3. The current batch retrieval API accepts variable IDs as query parameters. We anticipate a long string of variableIDs to be passed in for secret retrieval and this impacts performance.
4. Query parameters for HTTP GET requests are limited in length (2,048 characters) which is problematic if a customer has long secrets paths.
5. The new route offers is more useful if we choose to expand its functionality in the future. By using POST instead of GET we can not only opt to use query parameters but also utilize the body of the request.

At this stage this new batch retrieval API endpoint will be for internal use only.

### Design

#### Secrets Provider init container flow

As a small recap, the following explains the Secrets Provider init container flow

1. The `cyberark-secrets-provider-for-k8s` runs as an init container and authenticates with the Conjur/DAP Follower using the Kubernetes Authenticator (`authn-k8s`).

2. The `cyberark-secrets-provider-for-k8s` reads all Kubernetes secrets required by the applications in the same namespace.

3. For each mapped Kubernetes secret, the `cyberark-secrets-provider-for-k8s` 

   a. Performs a Batch Retrieval request and retrieves Conjur/DAP secrets.

   b. Updates the Kubernetes secret with the Conjur/DAP secret value.

4. The `cyberark-secrets-provider-for-k8s` init container runs to completion.

The new Batch Retrieval API will effect #2a. This will effect both init and application container (Milestone Job) flows.

### URI

##### POST /secrets

This method retuns a list of secrets, their status code, and their value if an error did not occur. For differences between proposed API and existing API see [Proposed API vs Current API](#proposed-api-vs-current-api) section below. We  require `account` as part of the URI instead of as part of the variableIDs

```
POST /secrets/{account}
```

### Request Body

The request body is a list of all variable IDs (paths to secrets in Conjur/DAP) that need to be retrieved. For example: 

```json
{
  "variables": [
    {"id": "var/path/conjurSecret"},
    {"id": "var/path/anotherConjurSecret"},
    ...
  ]
}
```

#### Headers

```
Content-Type: application/json
Authorization: Token token="eyJkYX...Rhb="
```

### Response

#### Batch Response

```json
# Headers
HTTP/1.1
Content-Type: application/json

# Status response code
200 OK

# Body
{
  "variables": [
  {
    "id": "var/path/name1",
    "status": 200,
    "value": "superSecret"
  },
  {
    "id": "var/path/name2",
    "status": 403,
    "error": {
      "message": "Forbidden"
    }
  },
  {
    "id": "var/path/name3",
    "status": 404,
    "error": {
      "message": "Not Found"
 		}
  },
  {
    "id": "var/path/name4",
    "status": 200,
    "value": "superSuperSecret"    
  }
 ]
}
```

| Code             | Description                                                  |
| ---------------- | ------------------------------------------------------------ |
| 200 OK           | All secret values were retrieved successfully                |
| 400 Bad Request  | Server cannot process request due to client error (unparsable request body) |
| 401 Unauthorized | The request lacks valid authentication credentials           |

If no Conjur/DAP secrets were able to be retrieved, the response will still return a 200 OK because the [request body is parsable]([https://docs.microsoft.com/en-us/graph/json-batching#:~:text=The%20status%20code%20on%20a,requests%20inside%20the%20batch%20succeeded](https://docs.microsoft.com/en-us/graph/json-batching#:~:text=The status code on a,requests inside the batch succeeded)) and request was handled.

If an invalid request body was supplied, a 400 Bad Request will be returned.

If the request was sent with an invalid access token, a 401 Unauthorized response will be returned.

#### Response body

All secret values will be treated as individual entities. If a single secret fails to be retrieved, this will ***not*** impact the integrity of the request. Each secret has it's own response code. 

| Parameter     | Description                                        |
| ------------- | -------------------------------------------------- |
| id            | ID for Conjur/DAP variable                         |
| value         | Secret for variable ID                             |
| error.status  | Response status code of the error. See table below |
| error.message | Response message of the error. See table below     |

##### Individual secret responses

The following are the possible error codes that can be returned for individual secrets.

| Status/Message  | Description                                                  |
| --------------- | ------------------------------------------------------------ |
| 400 Bad Request | Server cannot process request due to client error (invalid input parameters or logical validations). For example: adding `<>` in path to secret in Conjur/DAP |
| 403 Forbidden   | The authenticated user lacks the necessary privilege         |
| 404 Not Found   | Variable does not exist or variable does not have secret value |

### Proposed API vs Current API

|               | Current                                                      | Proposed                                                     |
| ------------- | ------------------------------------------------------------ | ------------------------------------------------------------ |
| URI           | `GET /secrets{?variable_ids}}`                               | `POST /secrets/{account}`                                    |
| Request Body  | -                                                            | See **Request Body** section                                 |
| VariableID    | `variable_ids=myorg:variable:var/path/conjurSecret`          | `variable_ids=var/path/conjurSecret`                         |
| Response Body | JSON of ***only*** successfully retrieved secrets and their values. Single failure, fails whole request and no secret is returned. | JSON of **all** secrets, their status code/messages, and values for successfully retrieved secrets |
| Behavior      | Request fails when individual request fails                  | Request does not fail when sub/individual requests fail      |

### Permissions

Only an authorized Host with `read` and `execute` privileges on the Conjur/DAP secrets will be able to fetch the secret values. Otherwise, the proper 403 Forbidden error code will be returned for `status` and `message` of the response respectively. 

During configuration of the Secrets Provider, the Host detailed for `authnLogin` is the Host that the Secrets Provider will use to authenticate to Conjur/DAP Follower and retrieve Conjur/DAP secrets. Therefore, this is the Host that will be used to make the request to this new Batch API endpoint and retrieve secrets from Conjur/DAP.

### Mitigation

Now that it is easy to brute-force multiple variables at a time in the body of the request, we should implement the necessary mitigations to close the attack surface.

### Audit

A new audit entry will made per secret fetched from Conjur/DAP as is done in the current Batch retrieval API endpoint. Also for all secrets that are attempted to be fetched and that the batch retrieval request was made.

### Test Plan

#### Conjur

The following integration tests will be implemented in Conjur

##### Request tests

The following table are possible responses ordered in terms of prioritization. 

| Priority | Scenario                  | Response                             |
| -------- | ------------------------- | ------------------------------------ |
| 1        | Access token is invalid   | 401 Unauthorized                     |
| 2        | Account does not exist    | 404 Not Found (if account is in URI) |
| 3        | JSON body is not parsable | 400 Bad Request                      |

##### Individual secret request tests

The following table are the permutations for the possible individual sercet responses. Each permutation will be its own test.

| Variable          | Execute | Read | Response | Message              |
| ----------------- | ------- | ---- | -------- | -------------------- |
| Not Exist         | -       | -    | 404      | Not Found            |
| Exists with value | No      | No   | 400      | Not Found            |
| ""                | Yes     | No   | 200      | OK                   |
| ""                | Yes     | Yes  | 200      | OK                   |
| ""                | No      | Yes  | 403      | Forbidden *Security! |
| Has no value      | No      | No   | 404      | Not Found            |
| ""                | Yes     | No   | ???      | Has No Value         |
| ""                | Yes     | Yes  | ???      | Has No value         |
| ""                | No      | Yes  | 403      | Forbidden            |

We will ensure all the proper audit record should appear for the whole request *and* each individual secret. All tests will be automated.

As part of input validation, we want to ensure that we deserialize only the parts of the JSON request that we expect. For example for input in the following format:

```
{
  "variables": [
    {"id": "var/path/conjurSecret"},
    {"anotherId": "var/path/notToBeUsed"},
    ...
  ]
}
```

We will only use the parts we expect and need. For example:

```
{
  "variables": [
    {"id": "var/path/conjurSecret"},
    ...
  ]
}
```



#### Secrets Provider for K8s

The following integration tests will be implemented in Secrets Provider for K8s

|      | Scenario                                                     | Given                                                        | When                           | Then                                                         | Manual/Automated |
| ---- | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------ | ------------------------------------------------------------ | ---------------- |
| 1    | *Vanilla flow*, Secret Provider Job successfully updates K8s Secrets | - Conjur is running<br />- Authenticator is defined<br />- Secrets defined in Conjur and mapped in K8s Secrets<br />- All mandatory values are defined in `custom-values.yaml`<br />- Customer installs Secrets provider Chart | Secrets Provider runs as a Job | - Secrets Provider pod authenticates and fetches Conjur secrets successfully<br />- All K8s Secrets are updated with  Conjur value <br />- Secrets Provider Job runs to completion<br />- Verify logs | Automated        |
| 2    | Partial success                                              | - Conjur is running<br />- Authenticator is defined<br />- Secrets defined in Conjur and mapped in K8s Secrets<br />- All mandatory values are defined in `custom-values.yaml`<br />- Host defined to Secrets Provider in `custom-values.yaml` does not have access to one of the Conjur secrets<br />- Customer installs Secrets provider Chart | Secrets Provider runs as a Job | - Secrets Provider pod authenticates and attempts to fetch Conjur secrets <br />- All K8s Secrets are updated with those Conjur values successfully fetched <br />- Secrets Provider Job runs to exhaustion<br />- Verify logs #1/2/3 appear | Automated        |
| 3    | *Vanilla test,* Conjur server is not up-to-date              | - Outdated Conjur server is running<br />- Authenticator is defined<br />- Secrets defined in Conjur and mapped in K8s Secrets<br />- All mandatory values are defined in `custom-values.yaml`<br />- Host defined to Secrets Provider in `custom-values.yaml` <br />- Customer installs Secrets provider Chart | Secrets Provider runs as a Job | - Secrets Provider pod authenticates and attempts to fetch Conjur secrets<br />- Application deploys successfully<br />- Verify logs #4 appear | Automated        |
| 4    | Conjur server is not up-to-date                              | - Outdated Conjur server is running<br />- Authenticator is defined<br />- Secrets defined in Conjur and mapped in K8s Secrets<br />- All mandatory values are defined in `custom-values.yaml`<br />- Host defined to Secrets Provider in `custom-values.yaml` does not have access to one of the Conjur secrets<br />- Customer installs Secrets provider Chart | Secrets Provider runs as a Job | - Secrets Provider pod authenticates and attempts to fetch Conjur secrets<br /><br />- Secrets Provider Job runs to exhaustion<br />- Application fails to be deployed<br />- Verify logs #1/4 appear | Automated        |

## Logs

|      | **Scenario**                                         | **Log message**                                              | Log level |
| ---- | ---------------------------------------------------- | ------------------------------------------------------------ | --------- |
| 1    | Secrets Provider batch retrieval has partial success | Failed to retrieve a portion of the Conjur/DAP secrets: Reason: '%s' | Error     |
| 2    | Secrets Provider batch retrieval total failure       | Failed to retrieve Conjur/DAP secrets: Reason: '%s'          | Error     |
| 3    | Secrets Provider batch retrieval has partial success | Skipped Kubernetes Secrets '%s'. Reason: Failed to retrieve Conjur secrets | Debug     |
| 4    | Secrets Provider success                             | Successfully updated '%d' out of '%d' Kubernetes Secrets     | Info      |
| 5    | Old batch retrieval endpoint                         | Warning: Secrets Provider cannot efficiently run because Conjur is not up to date. Please consider upgrading to the latest version | Warn      |

## Performance tests

The following performance tests will be run using *Jmeter*.
These tests will be run for both the new API and the existing one, and compare the results between them.

#### Benchmark

| Section     | Parameter                      | Value          |
| ----------- | ------------------------------ | -------------- |
| Conjur      | \# of standbys                 | 2              |
| Conjur      | \# of followers                | 1              |
| Environment | Kubernetes                     | OpenShift 3.11 |
| Environment | \# of cores                    | 16             |
| Environment | \# of nodes                    | 1              |
| Test        | \# of requesting threads       | 20             |
| Test        | \# of variables                | 1025           |
| Test        | \# of variabels per policy     | 5              |
| Test        | \# of policies                 | 205            |
| Test        | Variable ID length             | 50             |
| Test        | Variable value length (secret) | 50             |



|      | Scenario      | Given                                                        | When                                                         | Then                                                         | Manual/Automated |
| ---- | ------------- | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------------------------ | ---------------- |
| 1    | Response time | - Conjur is running<br />- Host is defined<br />- 1024 Secrets are defined<br /> | For X=1, X<=1024, X=X\*2:<br />    Concurrently retrieve X secrets for 5 minutes, measure average response time.<br />    Repeat each step 3 times.<br /> | Calculate the average response time between the 3 cycles for each step.<br />Plot the average response time per number of secrets provider. | Manual           |
| 2    | Throughput    | - Conjur is running<br />- Host is defined<br />- 1024 Secrets are defined<br /> | For X=1, X<=1024, X=X\*2:<br />    Concurrently retrieve X secrets for 5 minutes, measure throughput.<br />    Repeat each step 3 times.<br /> | Calculate the throughput between the 3 cycles for each step.<br />Plot the throughput per number of secrets provider. | Manual           |



### Open questions

1. Do we want this duplicated API documented? At this stage this API endpoint will be for internal use only.
2. Do we ultimately want to move to a single Batch Request API and depricate the existing one?
3. Do we want to support v4?

### Future enhancements

The following are ideas for possible extension of the batch retrieval endpoint API and not necessary directly related to the Secrets Provider project.

1. Add secret versions as a filtering query parameter so we can get all secrets after a certain version
2. Fetch all secrets that the user/host has permissions to
3. Fetch all secrets after `/var/path/*` that the user/host has permissions to

### Delivery Plan

- [ ] Fix flaky tests ***2 days***
- [ ] Implement new API endpoint in Conjur ***5 days***
- [ ] Add integration tests in Conjur ***4 days***
- [ ] Golang SDK
  - [ ] Implement batch logic ***2 days***
  - [ ] Add unit tests ***2 days***
- [ ] Implement change in Secrets Provider ***3 days***
- [ ] Implement test plan
  - [ ] Add integration tests in Secrets Provider ***3 days***
- [ ] Performance tests
  - [ ] In Conjur ***5 days***
  - [ ] In Secrets Provider ***3 days***
- [ ] Team/cross-team review cycle ***3 days***
- [ ] Logs reviewed by TW ***1 day***

Total: 33 days ~6.5 weeks


# Batch retrieval design

This document will cover the options for enhancing Batch Retrieval requests.

### Motivation

*Previously*, for the init container solution, each Secrets Provider was sitting within the same Deployment as the app. Because of this, when a batch retrieval request was made, the requests were dispersed across many call to the Conjur server. If one K8s Secret was not able to be updated with a Conjur value (for example due to a permission error), then the batch request would fail and only that pod would not spin up successfully. 

 *Now* that the Secrets Provider is outside, a single batch retrieval request will be made to Conjur for *all* application. So if there is a failure in receiving one of the values, then the whole request would fail. In other words, **no application** will spin up in the namespace.

|      | Solution                                                     | Pros                                                         | Cons                                | Effort |
| ---- | ------------------------------------------------------------ | ------------------------------------------------------------ | ----------------------------------- | ------ |
| 1    | *Client side:* <br />For each K8s Secret, perform a new Batch request | - No server side changes / no breaking changes               | Load on server with extra calls (*) |        |
| 2    | *Server side:*<br />Update Batch retrieval to return list of variables **and** their response (success/failure) | - Will help us during rotation for Milestone 2<br />- Better / straightforward design for how batch endpoints | May introduce breaking changes (**) |        |
| 3    | Stay as is                                                   | No additional work needed                                    | Bad UX                              | 0      |

(*) Fallback solution: use this solution as a safety net, only if the original Batch Retrieval request fails

(**) Fallback solution: slowly deprecate old batch retrieval API *or* create a new API endpoint




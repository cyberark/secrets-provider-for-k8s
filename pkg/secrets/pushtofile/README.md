This directory will be used to define a package for the Secrets Provider
"Push to File" functionality. It will contain source code to do the following:

- Parse the keys used in push-to-file annotations (e.g. sort annotations based
on secrets group)
- Retrieve Conjur secrets for each secrets group and write retrieved values to
a file info data struct
- Write secrets files to a shared volume


# Push to file demo

1. Generate annotations file
```shell
go run ./
```

2. Run push to file
```shell
go run ./testdata
```

3. Run unit tests and show coverage
```shell
go test -v -coverprofile cover.out -count 1 ./... && \
    go tool cover -html=cover.out -o cover.html \
    open ./cover.html
```

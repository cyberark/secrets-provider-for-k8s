# Push to file demo

0. Clean environment
```shell
rm testdata/annotations.txt testdata/db.js testdata/redis.json
```

1. Generate annotations file
```shell
go run ./testdata
```

2. Run push to file
```shell
go run ./ -f ./testdata/annotations.txt
```

3. Run unit tests and show coverage
```bash
go test -v -coverprofile cover.out -count 1 ./... && \
    go tool cover -html=cover.out -o cover.html && \
    open ./cover.html
```

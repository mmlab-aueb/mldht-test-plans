# mldht test plans
Test plans for testground for the [mldht project](https://mm.aueb.gr/projects/MLDHT)

## Import test plans
From the mldht test plans directory execute:
```
testground plan import --from ./mldht/ --name mldht
```

## Test plans run
Make sure `testground daemon` runs. Then the mldht test plans directory execute:
```
testground run composition -f ./compositions/find-peers.toml
```

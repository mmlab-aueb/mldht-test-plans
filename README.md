# mldht test plans
Test plans for testground for the [mldht project](https://mm.aueb.gr/projects/MLDHT)

## Import test plans
From the mldht test plans parent execute:
```
testground plan import --from ./tests/ --name mldht-test
```

## Test plans run
Make sure `testground daemon` runs. Then from the mldht test plans directory execute:
```
testground run composition -f compositions/mldht.toml
```

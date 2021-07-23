# mldht test plans
Test plans for testground for the [mldht project](https://mm.aueb.gr/projects/MLDHT)

## Import test plans
From the mldht test plans directory execute:
```
testground plan import --from ./tests/ --name mldht-test
```

## Test plans run
Make sure `testground daemon` runs. Then from the mldht test plans directory execute:
```
testground run composition -f compositions/mldht.toml
```

## Extracting results

```
grep -r 'routing-table-size' ~/testground/data/outputs/local_docker/mldht-test/<testid>/nodes/
grep -r 'records-found' ~/testground/data/outputs/local_docker/mldht-test/<testid>/nodes/
grep -r 'hops-to-provider' ~/testground/data/outputs/local_docker/mldht-test/<testid>/nodes/
```

## Notes
For experiments with many nodes (>30) you may experience errors because the arp
cache of the hosting system may have been exhausted. In that case you may have to
add to `/etc/sysctl.conf` the following commands 

```
net.ipv4.neigh.default.gc_interval = 3600
net.ipv4.neigh.default.gc_stale_time = 3600
net.core.somaxconn = 131072
net.netfilter.nf_conntrack_max = 1048576
net.ipv4.tcp_max_syn_backlog = 131072
net.core.netdev_max_backlog = 524288
net.ipv4.ip_local_port_range = 10000 65535
net.ipv4.tcp_tw_reuse = 1
net.core.rmem_max = 4194304
net.core.wmem_max = 4194304
net.ipv4.tcp_mem = 262144 524288 1572864
net.ipv4.tcp_rmem = 16384 131072 4194304
net.ipv4.tcp_wmem = 16384 131072 4194304
net.ipv4.neigh.default.gc_thresh2 = 4096
net.ipv4.neigh.default.gc_thresh3 = 32768
```

See also [this issue](https://github.com/testground/testground/issues/1251)

